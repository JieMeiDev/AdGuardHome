package upstream

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AdguardTeam/dnsproxy/internal/bootstrap"
	"github.com/ameshkov/dnsstamps"
	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// echState is a per-upstream, demand-refreshed ECH cache.  It does not run a
// background task.  Connections already authenticated with ECH remain usable
// after the DNS TTL expires; the next connection refreshes the configuration.
type echState struct {
	mu      sync.Mutex
	config  []byte
	expires time.Time
	loading chan struct{}
	lookup  func(context.Context) ([]byte, time.Duration, error)
}

func echHost(host string) string { return strings.ToLower(strings.TrimSuffix(host, ".")) }

// ValidateECHBootstrap validates the per-host ECH policy without network I/O.
// Bootstrap addresses must be IP literals or DNS stamps containing an IP, so
// ECH discovery never needs to resolve its own resolver through the main DNS.
func ValidateECHBootstrap(policy map[string][]string) error {
	for host, servers := range policy {
		if _, ok := dns.IsDomainName(host); !ok || net.ParseIP(host) != nil || host != echHost(host) || host == "" {
			return fmt.Errorf("ECH host must be a lowercase domain without a trailing dot: %q", host)
		}
		if len(servers) == 0 {
			return fmt.Errorf("ECH bootstrap for %s is empty", host)
		}
		for _, addr := range servers {
			var host string
			if strings.HasPrefix(addr, "sdns://") {
				stamp, err := dnsstamps.NewServerStampFromString(addr)
				if err != nil {
					return fmt.Errorf("invalid ECH bootstrap stamp: %w", err)
				}
				host = stamp.ServerAddrStr
			} else {
				if !strings.Contains(addr, "://") {
					addr = "udp://" + addr
				}
				u, err := url.Parse(addr)
				if err != nil {
					return fmt.Errorf("invalid ECH bootstrap: %w", err)
				}
				switch u.Scheme {
				case "udp", "tcp", "tls", "https", "h3", "quic":
				default:
					return fmt.Errorf("unsupported ECH bootstrap protocol %q", u.Scheme)
				}
				host = u.Hostname()
			}
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if _, err := netip.ParseAddr(host); err != nil {
				return fmt.Errorf("ECH bootstrap must contain a literal IP address: %w", err)
			}
		}
	}
	return nil
}

func (e *echState) get(ctx context.Context) ([]byte, error) {
	for {
		e.mu.Lock()
		if err := ctx.Err(); err != nil {
			e.mu.Unlock()
			return nil, err
		}
		if len(e.config) != 0 && time.Now().Before(e.expires) {
			config := slices.Clone(e.config)
			e.mu.Unlock()
			return config, nil
		}
		if pending := e.loading; pending != nil {
			e.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-pending:
				continue
			}
		}
		e.loading = make(chan struct{})
		e.mu.Unlock()
		break
	}
	config, ttl, err := e.lookup(ctx)
	if err == nil && !validECHList(config) {
		err = errors.New("invalid ECHConfigList")
	}
	e.mu.Lock()
	if err == nil {
		e.config, e.expires = slices.Clone(config), time.Now().Add(ttl)
	}
	close(e.loading)
	e.loading = nil
	e.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("discovering ECH: %w", err)
	}
	return slices.Clone(config), nil
}

// validECHList checks framing and rejects empty lists.  crypto/tls validates
// the supported versions, algorithms and individual configurations.
func validECHList(b []byte) bool {
	return len(b) > 2 && len(b) <= 65537 && int(binary.BigEndian.Uint16(b)) == len(b)-2
}

// retry records only authenticated server retry configurations.  It never
// extends the DNS lifetime.  A concurrent newer cache entry is not overwritten.
func (e *echState) retry(old, next []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if slices.Equal(e.config, old) {
		e.config = slices.Clone(next)
	}
}

func (p *dnsOverHTTPS) initECH(opts *Options) error {
	servers, enabled := opts.ECHBootstrap[echHost(p.addr.Hostname())]
	if !enabled {
		return nil
	}
	if err := ValidateECHBootstrap(opts.ECHBootstrap); err != nil {
		return err
	}
	if opts.InsecureSkipVerify {
		return errors.New("ECH requires certificate verification")
	}
	servers = slices.Clone(servers)
	p.ech = &echState{lookup: func(ctx context.Context) ([]byte, time.Duration, error) {
		return lookupECH(ctx, p.addr.Hostname(), p.addr.Port(), p.tlsConf.NextProtos, servers, opts)
	}}
	p.tlsConf.MinVersion = tls.VersionTLS13
	return nil
}

// handshakeECH performs at most two handshakes, including one retry with an
// authenticated server-supplied config.  An error never disables ECH.
func (p *dnsOverHTTPS) handshakeECH(ctx context.Context, base *tls.Config, dial func(context.Context, *tls.Config) error) error {
	ctx, cancel := context.WithTimeout(ctx, p.echTimeout())
	defer cancel()
	config, err := p.ech.get(ctx)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		conf := base.Clone()
		conf.MinVersion = tls.VersionTLS13
		conf.EncryptedClientHelloConfigList = config
		// Avoid early application data before ECH acceptance has been checked.
		conf.ClientSessionCache = nil
		err = dial(ctx, conf)
		if err == nil {
			p.logger.Debug("ECH accepted", "upstream", p.addrRedacted)
			return nil
		}
		var rejected *tls.ECHRejectionError
		if attempt != 0 || !errors.As(err, &rejected) || !validECHList(rejected.RetryConfigList) {
			break
		}
		p.logger.Debug("ECH rejected; retrying authenticated configuration", "upstream", p.addrRedacted)
		p.ech.retry(config, rejected.RetryConfigList)
		config = slices.Clone(rejected.RetryConfigList)
	}
	return fmt.Errorf("ECH handshake failed (no downgrade): %w", err)
}

func (p *dnsOverHTTPS) dialTLSECH(ctx context.Context, dial bootstrap.DialHandler, base *tls.Config) (conn *tls.Conn, err error) {
	err = p.handshakeECH(ctx, base, func(ctx context.Context, conf *tls.Config) error {
		raw, dialErr := dial(ctx, "tcp", "")
		if dialErr != nil {
			return dialErr
		}
		candidate := tls.Client(raw, conf)
		dialErr = candidate.HandshakeContext(ctx)
		if dialErr == nil && !candidate.ConnectionState().ECHAccepted {
			dialErr = errors.New("ECH was not accepted")
		}
		if dialErr != nil {
			_ = candidate.Close()
			return dialErr
		}
		conn = candidate
		return nil
	})
	return conn, err
}

func (p *dnsOverHTTPS) dialQUICECH(ctx context.Context, addr string, base *tls.Config, quicConf *quic.Config) (conn *quic.Conn, err error) {
	err = p.handshakeECH(ctx, base, func(ctx context.Context, conf *tls.Config) error {
		candidate, dialErr := quic.DialAddr(ctx, addr, conf, quicConf)
		if dialErr != nil {
			return dialErr
		}
		if !candidate.ConnectionState().TLS.ECHAccepted {
			_ = candidate.CloseWithError(QUICCodeNoError, "ECH not accepted")
			return errors.New("ECH was not accepted")
		}
		conn = candidate
		return nil
	})
	return conn, err
}

func (p *dnsOverHTTPS) echTimeout() time.Duration {
	if p.timeout > 0 {
		return p.timeout
	}
	return 10 * time.Second
}

// lookupECH tries the explicitly configured, independent bootstrap resolvers.
// Each exchange uses the remaining request deadline and is capped at 3 seconds.
// No system resolver or main upstream is used for these HTTPS-record queries.
func lookupECH(ctx context.Context, host, port string, alpns, servers []string, opts *Options) ([]byte, time.Duration, error) {
	var errs []error
	for _, server := range servers {
		name := dns.Fqdn(host)
		// RFC 9460 section 9.1: non-default HTTPS ports use a port prefix.
		if port != "443" {
			name = "_" + port + "._https." + name
		}
		ttl := time.Duration(1<<63 - 1)
		seen := map[string]bool{}
		for depth := 0; depth < 8; depth++ {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
			if seen[echHost(name)] {
				errs = append(errs, errors.New("ECH DNS alias loop"))
				break
			}
			seen[echHost(name)] = true
			timeout := 3 * time.Second
			if deadline, ok := ctx.Deadline(); ok {
				timeout = min(timeout, time.Until(deadline))
			}
			if timeout <= 0 {
				return nil, 0, context.DeadlineExceeded
			}
			boot, err := AddressToUpstream(server, &Options{Timeout: timeout, RootCAs: opts.RootCAs, Logger: opts.Logger})
			if err != nil {
				errs = append(errs, err)
				break
			}
			req := new(dns.Msg).SetQuestion(name, dns.TypeHTTPS)
			req.SetEdns0(1232, false)
			resp, err := boot.Exchange(req)
			_ = boot.Close()
			if err != nil {
				errs = append(errs, err)
				break
			}
			config, alias, answerTTL, err := echAnswer(resp, name, port, alpns)
			if err != nil {
				errs = append(errs, err)
				break
			}
			ttl = min(ttl, answerTTL)
			if len(config) != 0 {
				return config, ttl, nil
			}
			name = alias
			if depth == 7 {
				errs = append(errs, errors.New("ECH DNS alias limit exceeded"))
			}
		}
	}
	return nil, 0, fmt.Errorf("no usable ECH HTTPS record: %w", errors.Join(errs...))
}

// echAnswer accepts only matching answer owners.  AliasMode and CNAME targets
// are queried separately, with TTLs bounded by every hop.  Service records
// requiring endpoint redirection or unsupported mandatory keys are ignored.
func echAnswer(resp *dns.Msg, name, port string, alpns []string) ([]byte, string, time.Duration, error) {
	if resp == nil || resp.Rcode != dns.RcodeSuccess || resp.Truncated {
		return nil, "", 0, errors.New("unsuccessful or truncated HTTPS response")
	}
	var selected *dns.HTTPS
	var config []byte
	for _, rr := range resp.Answer {
		if !strings.EqualFold(rr.Header().Name, name) {
			continue
		}
		if cname, ok := rr.(*dns.CNAME); ok {
			return nil, cname.Target, time.Duration(cname.Hdr.Ttl) * time.Second, nil
		}
		https, ok := rr.(*dns.HTTPS)
		if !ok {
			continue
		}
		if https.Priority == 0 {
			if https.Target == "." {
				return nil, "", 0, errors.New("HTTPS alias indicates unavailable service")
			}
			return nil, https.Target, time.Duration(https.Hdr.Ttl) * time.Second, nil
		}
		if https.Target != "." && !strings.EqualFold(https.Target, name) {
			continue
		}
		var ech []byte
		usable := true
		for _, v := range https.Value {
			switch v := v.(type) {
			case *dns.SVCBECHConfig:
				ech = v.ECH
			case *dns.SVCBPort:
				usable = usable && fmt.Sprint(v.Port) == port
			case *dns.SVCBAlpn:
				usable = usable && slices.ContainsFunc(v.Alpn, func(a string) bool { return slices.Contains(alpns, a) })
			case *dns.SVCBMandatory:
				for _, key := range v.Code {
					if key != dns.SVCB_ECHCONFIG && key != dns.SVCB_ALPN && key != dns.SVCB_PORT {
						usable = false
					}
					if !slices.ContainsFunc(https.Value, func(v dns.SVCBKeyValue) bool { return v.Key() == key }) {
						usable = false
					}
				}
			}
		}
		if usable && validECHList(ech) && (selected == nil || https.Priority < selected.Priority) {
			selected, config = https, ech
		}
	}
	if selected == nil {
		return nil, "", 0, errors.New("no compatible ECH service record")
	}
	return slices.Clone(config), "", time.Duration(selected.Hdr.Ttl) * time.Second, nil
}

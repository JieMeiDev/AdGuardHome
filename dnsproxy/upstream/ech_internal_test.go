package upstream

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
)

// testECHKey generates a real X25519/HKDF-SHA256/AES-128-GCM ECH configuration.
func testECHKey(t *testing.T, id byte) (tls.EncryptedClientHelloKey, []byte) {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	require.NoError(t, err)
	name := "public.ech.test"
	contents := []byte{id, 0, 32, 0, 32}
	contents = append(contents, key.PublicKey().Bytes()...)
	contents = append(contents, 0, 4, 0, 1, 0, 1, 0, byte(len(name)))
	contents = append(contents, name...)
	contents = append(contents, 0, 0)
	config := binary.BigEndian.AppendUint16([]byte{0xfe, 0x0d}, uint16(len(contents)))
	config = append(config, contents...)
	list := binary.BigEndian.AppendUint16(nil, uint16(len(config)))
	list = append(list, config...)
	return tls.EncryptedClientHelloKey{Config: config, PrivateKey: key.Bytes(), SendAsRetry: true}, list
}

func testECHBootstrap(t *testing.T, config []byte, calls *atomic.Int32) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		calls.Add(1)
		resp := new(dns.Msg).SetReply(req)
		if len(config) != 0 {
			resp.Answer = []dns.RR{&dns.HTTPS{SVCB: dns.SVCB{
				Hdr:      dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeHTTPS, Class: dns.ClassINET, Ttl: 60},
				Priority: 1, Target: ".", Value: []dns.SVCBKeyValue{&dns.SVCBECHConfig{ECH: config}},
			}}}
		}
		_ = w.WriteMsg(resp)
	})}
	ready := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(ready) }
	go func() { _ = srv.ActivateAndServe() }()
	<-ready
	t.Cleanup(func() { _ = srv.Shutdown() })
	return pc.LocalAddr().String()
}

func TestECHExchange(t *testing.T) {
	for _, protocol := range []string{"https", "h3"} {
		for _, scenario := range []string{"accepted", "rotation", "rejected", "missing", "untrusted", "untrusted_retry", "malformed", "bootstrap_failover"} {
			t.Run(protocol+"/"+scenario, func(t *testing.T) {
				t.Parallel()
				key, config := testECHKey(t, 1)
				serverTLS, roots := createServerTLSConfig(t, "*.ech.test")
				serverTLS.MinVersion = tls.VersionTLS13
				serverTLS.EncryptedClientHelloKeys = []tls.EncryptedClientHelloKey{key}
				switch scenario {
				case "rotation":
					newKey, _ := testECHKey(t, 2)
					serverTLS.EncryptedClientHelloKeys = []tls.EncryptedClientHelloKey{newKey}
				case "rejected":
					serverTLS.EncryptedClientHelloKeys = nil
				case "missing":
					config = nil
				case "untrusted":
					roots = nil
				case "untrusted_retry":
					roots = nil
					newKey, _ := testECHKey(t, 2)
					serverTLS.EncryptedClientHelloKeys = []tls.EncryptedClientHelloKey{newKey}
				case "malformed":
					config = []byte{0, 1, 0}
				}
				var requests, accepted, dnsCalls atomic.Int32
				dnsHandler := createDoHHandler()
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					if r.TLS != nil && r.TLS.ECHAccepted {
						accepted.Add(1)
					}
					dnsHandler.ServeHTTP(w, r)
				})
				srv := httptest.NewUnstartedServer(handler)
				srv.TLS = serverTLS.Clone()
				srv.EnableHTTP2 = true
				srv.StartTLS()
				t.Cleanup(srv.Close)
				pc, err := net.ListenPacket("udp", srv.Listener.Addr().String())
				require.NoError(t, err)
				h3 := &http3.Server{TLSConfig: serverTLS.Clone(), Handler: handler}
				go func() { _ = h3.Serve(pc) }()
				t.Cleanup(func() { _ = h3.Close(); _ = pc.Close() })
				_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
				require.NoError(t, err)
				boots := []string{testECHBootstrap(t, config, &dnsCalls)}
				if scenario == "bootstrap_failover" {
					boots = append([]string{testECHBootstrap(t, nil, &dnsCalls)}, boots...)
				}
				u, err := AddressToUpstream(protocol+"://resolver.ech.test:"+port+"/dns-query", &Options{
					Logger: testLogger, RootCAs: roots, Timeout: 3 * time.Second,
					Bootstrap:    StaticResolver{netip.MustParseAddr("127.0.0.1")},
					ECHBootstrap: map[string][]string{"resolver.ech.test": boots},
				})
				require.NoError(t, err)
				t.Cleanup(func() { _ = u.Close() })
				for range 2 {
					resp, exchangeErr := u.Exchange(new(dns.Msg).SetQuestion("example.org.", dns.TypeA))
					if scenario == "rejected" || scenario == "missing" || scenario == "untrusted" || scenario == "untrusted_retry" || scenario == "malformed" {
						require.Error(t, exchangeErr)
						require.Nil(t, resp)
					} else {
						require.NoError(t, exchangeErr)
						require.Equal(t, dns.RcodeSuccess, resp.Rcode)
					}
				}
				if scenario == "rejected" || scenario == "missing" || scenario == "untrusted" || scenario == "untrusted_retry" || scenario == "malformed" {
					require.Zero(t, requests.Load(), "no DNS payload may be sent without ECH")
				} else {
					require.EqualValues(t, 2, requests.Load())
					require.EqualValues(t, 2, accepted.Load())
					want := int32(1)
					if scenario == "bootstrap_failover" {
						want = 2
					}
					require.Equal(t, want, dnsCalls.Load(), "ECH records must be cached")
				}
			})
		}
	}
}

func TestECHCache(t *testing.T) {
	_, config := testECHKey(t, 1)
	var calls atomic.Int32
	e := &echState{lookup: func(context.Context) ([]byte, time.Duration, error) {
		calls.Add(1)
		return config, time.Minute, nil
	}}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, err := e.get(context.Background())
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 1, calls.Load())
	e.expires = time.Now().Add(-time.Second)
	_, err := e.get(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load())
	e.expires = time.Now().Add(-time.Second)
	e.lookup = func(context.Context) ([]byte, time.Duration, error) { return nil, 0, errors.New("offline") }
	_, err = e.get(context.Background())
	require.ErrorContains(t, err, "offline", "expired ECH is not used after a refresh failure")
}

func TestECHRetryBound(t *testing.T) {
	_, config := testECHKey(t, 1)
	_, next := testECHKey(t, 2)
	p := &dnsOverHTTPS{logger: testLogger, timeout: time.Second, ech: &echState{
		lookup: func(context.Context) ([]byte, time.Duration, error) { return config, time.Minute, nil },
	}}
	attempts := 0
	err := p.handshakeECH(context.Background(), &tls.Config{}, func(_ context.Context, c *tls.Config) error {
		attempts++
		require.NotEmpty(t, c.EncryptedClientHelloConfigList)
		return &tls.ECHRejectionError{RetryConfigList: next}
	})
	require.ErrorContains(t, err, "no downgrade")
	require.Equal(t, 2, attempts)
}

func TestECHAnswer(t *testing.T) {
	_, config := testECHKey(t, 1)
	base := &dns.HTTPS{SVCB: dns.SVCB{
		Hdr:      dns.RR_Header{Name: "resolver.ech.test.", Rrtype: dns.TypeHTTPS, Class: dns.ClassINET, Ttl: 30},
		Priority: 1, Target: ".", Value: []dns.SVCBKeyValue{&dns.SVCBECHConfig{ECH: config}},
	}}
	for _, kind := range []string{"valid", "wrong_owner", "no_ech", "bad_list", "mandatory", "port", "alpn", "target", "truncated", "nxdomain"} {
		t.Run(kind, func(t *testing.T) {
			rr := dns.Copy(base).(*dns.HTTPS)
			resp := &dns.Msg{Answer: []dns.RR{rr}}
			switch kind {
			case "wrong_owner":
				rr.Hdr.Name = "attacker.test."
			case "no_ech":
				rr.Value = nil
			case "bad_list":
				rr.Value = []dns.SVCBKeyValue{&dns.SVCBECHConfig{ECH: []byte{0, 0}}}
			case "mandatory":
				rr.Value = append(rr.Value, &dns.SVCBMandatory{Code: []dns.SVCBKey{65400}})
			case "port":
				rr.Value = append(rr.Value, &dns.SVCBPort{Port: 8443})
			case "alpn":
				rr.Value = append(rr.Value, &dns.SVCBAlpn{Alpn: []string{"h2"}})
			case "target":
				rr.Target = "other.ech.test."
			case "truncated":
				resp.Truncated = true
			case "nxdomain":
				resp.Rcode = dns.RcodeNameError
			}
			got, _, ttl, err := echAnswer(resp, base.Hdr.Name, "443", []string{"h3"})
			if kind == "valid" {
				require.NoError(t, err)
				require.Equal(t, config, got)
				require.Equal(t, 30*time.Second, ttl)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestECHPolicyValidation(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:12345", "tcp://[::1]:53", "https://1.1.1.1/dns-query"} {
		require.NoError(t, ValidateECHBootstrap(map[string][]string{"dns.example": {addr}}))
	}
	for _, addr := range []string{"", "https://dns.example/dns-query", "tcp://localhost:53", "unknown://1.1.1.1"} {
		require.Error(t, ValidateECHBootstrap(map[string][]string{"dns.example": {addr}}), fmt.Sprint(addr))
	}
	require.Error(t, ValidateECHBootstrap(map[string][]string{"dns.example": nil}))
	require.Error(t, ValidateECHBootstrap(map[string][]string{"DNS.example": {"1.1.1.1"}}))
	opts := &Options{ECHBootstrap: map[string][]string{"dns.example": {"1.1.1.1"}}}
	require.Equal(t, opts.ECHBootstrap, opts.Clone().ECHBootstrap)
	opts.InsecureSkipVerify = true
	_, err := AddressToUpstream("https://dns.example/dns-query", opts)
	require.ErrorContains(t, err, "certificate verification")
}

func TestECHCacheCancellationAndZeroTTL(t *testing.T) {
	_, config := testECHKey(t, 1)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	e := &echState{lookup: func(context.Context) ([]byte, time.Duration, error) {
		close(started)
		<-release
		return config, 0, nil
	}}
	go func() { _, _ = e.get(context.Background()); close(done) }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := e.get(ctx)
	require.ErrorIs(t, err, context.Canceled)
	// A waiting caller's timeout must not wait for the active lookup's timeout.
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = e.get(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	close(release)
	<-done
	e.lookup = func(context.Context) ([]byte, time.Duration, error) { return nil, 0, errors.New("zero TTL refreshed") }
	_, err = e.get(context.Background())
	require.ErrorContains(t, err, "zero TTL refreshed")
}

func TestECHLookupAliases(t *testing.T) {
	_, config := testECHKey(t, 1)
	for _, kind := range []string{"cname", "alias", "loop", "nondefault_port"} {
		t.Run(kind, func(t *testing.T) {
			pc, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)
			var calls atomic.Int32
			srv := &dns.Server{PacketConn: pc, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
				calls.Add(1)
				name := req.Question[0].Name
				resp := new(dns.Msg).SetReply(req)
				hdr := dns.RR_Header{Name: name, Rrtype: dns.TypeHTTPS, Class: dns.ClassINET, Ttl: 60}
				if name == "target.ech.test." || name == "_8443._https.resolver.ech.test." {
					resp.Answer = []dns.RR{&dns.HTTPS{SVCB: dns.SVCB{Hdr: hdr, Priority: 1, Target: ".", Value: []dns.SVCBKeyValue{&dns.SVCBECHConfig{ECH: config}}}}}
				} else if kind == "cname" {
					hdr.Rrtype, hdr.Ttl = dns.TypeCNAME, 7
					resp.Answer = []dns.RR{&dns.CNAME{Hdr: hdr, Target: "target.ech.test."}}
				} else if kind != "nondefault_port" {
					hdr.Ttl = 7
					target := "target.ech.test."
					if kind == "loop" {
						target = name
					}
					resp.Answer = []dns.RR{&dns.HTTPS{SVCB: dns.SVCB{Hdr: hdr, Target: target}}}
				}
				_ = w.WriteMsg(resp)
			})}
			ready := make(chan struct{})
			srv.NotifyStartedFunc = func() { close(ready) }
			go func() { _ = srv.ActivateAndServe() }()
			<-ready
			t.Cleanup(func() { _ = srv.Shutdown() })
			port := "443"
			if kind == "nondefault_port" {
				port = "8443"
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, ttl, err := lookupECH(ctx, "resolver.ech.test", port, []string{"h3"}, []string{pc.LocalAddr().String()}, &Options{Logger: testLogger})
			if kind == "loop" {
				require.ErrorContains(t, err, "loop")
				require.EqualValues(t, 1, calls.Load())
			} else {
				require.NoError(t, err)
				require.Equal(t, config, got)
				if kind == "nondefault_port" {
					require.Equal(t, 60*time.Second, ttl)
				} else {
					require.Equal(t, 7*time.Second, ttl)
				}
			}
		})
	}
}

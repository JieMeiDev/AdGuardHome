package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type echInfoUpstream struct {
	name string
	ech  bool
	fail bool
}

func (u *echInfoUpstream) Address() string { return u.name }
func (u *echInfoUpstream) Close() error    { return nil }
func (u *echInfoUpstream) Exchange(req *dns.Msg) (*dns.Msg, error) {
	resp, _, err := u.ExchangeWithInfo(req)
	return resp, err
}
func (u *echInfoUpstream) ExchangeWithInfo(req *dns.Msg) (*dns.Msg, upstream.ExchangeInfo, error) {
	info := upstream.ExchangeInfo{ECHAccepted: u.ech || strings.HasPrefix(req.Question[0].Name, "ech-")}
	if u.fail {
		// Even an upstream returning stale information on error must not mark a fallback.
		return nil, info, fmt.Errorf("test upstream failure")
	}
	resp := new(dns.Msg).SetReply(req)
	resp.Answer = []dns.RR{&dns.A{
		Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   net.IPv4(127, 0, 0, 1),
	}}
	return resp, info, nil
}

func TestECHQueryAttribution(t *testing.T) {
	for _, mode := range []UpstreamMode{UpstreamModeLoadBalance, UpstreamModeParallel, UpstreamModeFastestAddr} {
		for _, tc := range []struct {
			name                           string
			mainECH, fallback, fallbackECH bool
		}{
			{name: "ech_main", mainECH: true},
			{name: "ordinary_main"},
			{name: "ordinary_fallback", mainECH: true, fallback: true},
			{name: "ech_fallback", fallback: true, fallbackECH: true},
		} {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				main := &echInfoUpstream{name: "main", ech: tc.mainECH, fail: tc.fallback}
				bad := &echInfoUpstream{name: "failed", ech: true, fail: true}
				conf := &Config{
					Logger: testLogger, CacheEnabled: true, UpstreamMode: mode,
					UDPListenAddr:  []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1)}},
					UpstreamConfig: &UpstreamConfig{Upstreams: []upstream.Upstream{bad, main}},
				}
				wantECH, wantAddress := tc.mainECH, "main"
				if tc.fallback {
					conf.Fallbacks = &UpstreamConfig{Upstreams: []upstream.Upstream{
						bad, &echInfoUpstream{name: "fallback", ech: tc.fallbackECH},
					}}
					wantECH, wantAddress = tc.fallbackECH, "fallback"
				}
				p := mustNew(t, conf)
				for attempt := range 2 {
					d := &DNSContext{Proto: ProtoUDP, Req: new(dns.Msg).SetQuestion("example.org.", dns.TypeA)}
					require.NoError(t, p.Resolve(context.Background(), d))
					require.Equal(t, dns.RcodeSuccess, d.Res.Rcode)
					if attempt == 0 {
						require.Equal(t, wantAddress, d.Upstream.Address())
						require.Equal(t, wantECH, d.QueryStatistics().ECHAccepted())
					} else {
						require.Nil(t, d.Upstream)
						require.True(t, d.QueryStatistics().Main()[0].IsCached)
						require.False(t, d.QueryStatistics().ECHAccepted())
					}
				}
			})
		}
	}
}

func TestECHQueryAttributionConcurrent(t *testing.T) {
	p := mustNew(t, &Config{
		Logger: testLogger, UpstreamMode: UpstreamModeParallel,
		UDPListenAddr:  []*net.UDPAddr{{IP: net.IPv4(127, 0, 0, 1)}},
		UpstreamConfig: &UpstreamConfig{Upstreams: []upstream.Upstream{&echInfoUpstream{name: "shared"}}},
	})
	for i := range 30 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			prefix := "plain"
			if i%2 == 0 {
				prefix = "ech"
			}
			d := &DNSContext{Proto: ProtoUDP, Req: new(dns.Msg).SetQuestion(fmt.Sprintf("%s-%d.example.", prefix, i), dns.TypeA)}
			require.NoError(t, p.Resolve(context.Background(), d))
			require.Equal(t, i%2 == 0, d.QueryStatistics().ECHAccepted())
		})
	}
}

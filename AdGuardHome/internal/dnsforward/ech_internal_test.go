package dnsforward

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/dnsproxy/proxy"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"
)

func TestUpstreamECHDiskConfig(t *testing.T) {
	var conf Config
	require.NoError(t, yaml.Unmarshal([]byte(`upstream_dns:
  - h3://dns.example/dns-query
upstream_ech:
  dns.example:
    - 192.0.2.53
`), &conf))
	s := &Server{conf: ServerConfig{Config: conf}}
	var disk Config
	s.WriteDiskConfig(&disk)
	data, err := yaml.Marshal(disk)
	require.NoError(t, err)
	var reread Config
	require.NoError(t, yaml.Unmarshal(data, &reread))
	require.Equal(t, conf.UpstreamECH, reread.UpstreamECH)
	disk.UpstreamECH["dns.example"][0] = "192.0.2.54"
	require.Equal(t, "192.0.2.53", s.conf.UpstreamECH["dns.example"][0])
}

// This runs the AGH forwarding server and proves a configured ECH upstream
// fails closed on a missing HTTPS record, while existing fallback still works.
func TestUpstreamECHFallback(t *testing.T) {
	var discoveries, fallbackQueries atomic.Int32
	boot := newLocalUpstreamListener(t, 0, dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		resp := new(dns.Msg).SetReply(req)
		switch req.Question[0].Qtype {
		case dns.TypeA:
			resp.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("127.0.0.1")}}
		case dns.TypeHTTPS:
			discoveries.Add(1)
		}
		_ = w.WriteMsg(resp)
	}))
	fallback := newLocalUpstreamListener(t, 0, dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		fallbackQueries.Add(1)
		resp := new(dns.Msg).SetReply(req)
		resp.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP("203.0.113.9")}}
		_ = w.WriteMsg(resp)
	}))
	bootAddr := "tcp://" + boot.String()
	s := createTestServer(t, &filtering.Config{BlockingMode: filtering.BlockingModeDefault}, ServerConfig{
		UDPListenAddrs: []*net.UDPAddr{{IP: net.ParseIP("127.0.0.1")}},
		TCPListenAddrs: []*net.TCPAddr{{IP: net.ParseIP("127.0.0.1")}},
		TLSConf:        &TLSConfig{}, ServePlainDNS: true,
		UpstreamTimeout: time.Second,
		Config: Config{
			UpstreamDNS:      []string{"h3://dns.example:8443/dns-query"},
			BootstrapDNS:     []string{bootAddr},
			UpstreamECH:      map[string][]string{"dns.example": {bootAddr}},
			FallbackDNS:      []string{"tcp://" + fallback.String()},
			UpstreamMode:     UpstreamModeLoadBalance,
			EDNSClientSubnet: &EDNSClientSubnet{}, ClientsContainer: EmptyClientsContainer{},
		},
	}, testTLSConfigProvider)
	startDeferStop(t, s)
	client := &dns.Client{Timeout: 3 * time.Second}
	resp, _, err := client.Exchange(new(dns.Msg).SetQuestion("fallback.example.", dns.TypeA), s.dnsProxy.Addr(proxy.ProtoUDP).String())
	require.NoError(t, err)
	require.Equal(t, dns.RcodeSuccess, resp.Rcode)
	require.Len(t, resp.Answer, 1)
	require.Equal(t, "203.0.113.9", resp.Answer[0].(*dns.A).A.String())
	require.Positive(t, discoveries.Load())
	require.EqualValues(t, 1, fallbackQueries.Load())
}

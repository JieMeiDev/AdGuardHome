package querylog

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func TestECHQueryLogPersistence(t *testing.T) {
	for _, tc := range []struct {
		name, address     string
		ech, cached, want bool
	}{
		{name: "accepted", address: "https://dns.example/dns-query", ech: true, want: true},
		{name: "ordinary", address: "https://dns.example/dns-query"},
		{name: "cache", address: "https://dns.example/dns-query", ech: true, cached: true},
		{name: "local", ech: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			l := &queryLog{logger: testLogger}
			entry := newLogEntry(ctx, testLogger, &AddParams{
				Question: new(dns.Msg).SetQuestion("example.org.", dns.TypeA),
				Result:   &filtering.Result{}, ClientIP: net.IPv4(127, 0, 0, 1),
				Upstream: tc.address, UpstreamECH: tc.ech, Cached: tc.cached,
			})
			require.Equal(t, tc.want, entry.UpstreamECH)
			data, err := json.Marshal(entry)
			require.NoError(t, err)
			decoded := &logEntry{}
			l.decodeLogEntry(ctx, decoded, string(data))
			require.Equal(t, entry.UpstreamECH, decoded.UpstreamECH)
			for _, item := range []*logEntry{entry, decoded} {
				api := l.entryToJSON(ctx, item, func(net.IP) {})
				require.Equal(t, tc.address, api["upstream"], "the raw address must remain unchanged")
				if tc.want {
					require.Equal(t, true, api["upstream_ech"])
				} else {
					require.NotContains(t, api, "upstream_ech")
				}
			}
		})
	}
	// Historical records have no flag and must not be inferred from today's configuration.
	l := &queryLog{logger: testLogger}
	old := &logEntry{}
	l.decodeLogEntry(context.Background(), old, `{"QH":"example.org","QT":"A","QC":"IN","Upstream":"https://dns.example/dns-query"}`)
	require.False(t, old.UpstreamECH)
}

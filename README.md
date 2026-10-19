# AdGuardHome ECH

Unofficial, AI-assisted source snapshots adding **client-side upstream ECH**
(Encrypted Client Hello) to AdGuard Home. Both revisions are based on official
**AdGuard Home v0.107.79**, with its **dnsproxy v0.83.2** dependency modified
alongside it. This is a personal project, not an official AdGuard release.

基于官方 `v0.107.79` 的个人修改版，增加 AdGuard Home 到 DoH/H3 上游的 ECH 支持。
仓库同时包含完整的 AdGuardHome 与 dnsproxy 源码，以 Go workspace 一起构建。
`.2` 在查询日志原有 DNS server 地址后增加 `(ECH)`，仅标记实际使用 ECH 的查询。

## Versions

| Git tag | Executable version | Changes |
| --- | --- | --- |
| `v0.107.79.1` | `v0.107.79-ech.1` | Upstream ECH support for DoH/H2 and DoH/H3 |
| `v0.107.79.2` | `v0.107.79-ech.2` | Adds per-query ECH attribution and a compact query-log indicator |

The tags distinguish personal revisions of the same official base. Executable
versions retain the `-ech.N` identifiers used in the tested deployments; these
tags do not claim to be newer official AdGuard Home releases.

## Upstream sources

| Component | Official version | Base commit |
| --- | --- | --- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | v0.107.79 | `05ba17b282da1c4393d6a4ba4db0cf519194a362` |
| [dnsproxy](https://github.com/AdguardTeam/dnsproxy) | v0.83.2 | `acf2b30e05202e171bbbaf4a3176ea07edb964b3` |

Original source, documentation, copyright notices, and licenses are retained
under `AdGuardHome/` and `dnsproxy/`. Module import paths and dependency pins
are unchanged. The root `go.work` selects the included dnsproxy implementation.
`SOURCE-MANIFEST.json` identifies the modified source files by SHA-256.

## Behavior

- Opt-in ECH policy per upstream hostname for `https://` and `h3://`.
- ECH configuration is discovered from HTTPS records through separately
  configured, independently reachable resolvers with literal-IP endpoints,
  including DNS stamps containing an IP address.
- TTL-bounded, demand-driven caching, bounded CNAME/HTTPS AliasMode traversal,
  and concurrent lookup coordination; no background refresh worker.
- Normal certificate verification and explicit ECH acceptance before sending
  DNS payloads. An authenticated ECH rejection permits one retry with the
  supplied new configuration. Required-ECH upstreams never silently downgrade.
- Failure uses the existing upstream/fallback mechanism. Each fallback keeps
  its own configured policy.
- HTTP connection reuse is preserved. TLS session resumption and H3 0-RTT
  are disabled on the ECH path in this prototype.

See `ech-config.example.yaml` for the provisional YAML interface. Configure
it in the existing `dns` section; use resolvers reachable from your own network.
The example endpoint is illustrative, not a reachability recommendation.
Deleting the hostname policy restores the original behavior for that upstream.

Scope: outgoing DoH/H3 connections only. This does not add downstream/server
ECH, DoT/DoQ ECH, or full SVCB alternate-endpoint/port routing. It does not
configure routing, a VPN, firewall rules, or a proxy. ECH hides the inner
server name; destination IP addresses and traffic metadata remain visible.

## Query-log indicator

In Response details, the existing DNS server address gains an ` (ECH)` suffix
only when the successful HTTP response's TLS connection accepted ECH. This
works with HTTP/2 and HTTP/3 connection reuse and follows the upstream that
actually returned the answer, including fallback. Cached/local answers and
historical entries without metadata are unmarked. The raw upstream address
is unchanged; query-log storage and the API use an optional boolean field.
No new UI controls or details rows are added.

## Build

Tested with Go 1.27.1 and Node.js 26.3.0. Install Go, Node.js/npm, and `shasum`, then
run from the repository root:

```sh
./build.sh
```

The script builds the frontend and embeds it in standalone Linux amd64 and
macOS amd64 binaries under `artifacts/`, with SHA-256 checksums. It does not
install or deploy them. For a different Go executable, set `GO=/path/to/go`.

## Validation

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

Local tests use real local TLS/H2/H3 endpoints and cover acceptance, key
rotation, rejection, missing/malformed records, certificate failures,
bootstrap failover, caching, concurrency, and fallback. Live H2/H3 exchanges
and an OpenWrt deployment were also verified with ECH accepted.

Revision .2 additionally covers request attribution, connection reuse, fallback,
cached/local answers, concurrent requests, log persistence, and old records.
Its five frontend formatting tests, TypeScript check, and modified-file ESLint
checks passed. Run the frontend checks after `npm ci` in `AdGuardHome/client`:

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

The full upstream suite is **not claimed to pass**: some existing tests need
public DNS endpoints such as 8.8.8.8:53 and 1.1.1.1:53 and timed out in the
test network. Full upstream CI, including `make go-check`, remains to be run.

## Upstream contribution

The design proposal is in [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573).
The proposed path is dnsproxy first, then AdGuard Home configuration/dependency
integration, subject to maintainer feedback. These snapshots are not a claim
of upstream acceptance or a submitted pull request.

## License

AdGuard Home and this combined distribution retain GPL-3.0 licensing; see
`LICENSE` and `AdGuardHome/LICENSE.txt`. dnsproxy retains its license in
`dnsproxy/LICENSE`. Original third-party copyright and license notices remain
with their respective source files.

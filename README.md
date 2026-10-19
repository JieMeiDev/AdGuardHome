# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-blue)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](README.zh-CN.md)

> [!IMPORTANT]
> To use this project's upstream ECH feature, install the **AdGuard Home core executable** provided in this repository's [Releases](https://github.com/JieMeiDev/AdGuardHome/releases). Official v0.107.79 does not include this extension; changing YAML or replacing the web interface alone is insufficient. After installation, configure an ECH-capable upstream as documented.

A personal modification of official **AdGuard Home v0.107.79**, adding **ECH (Encrypted Client Hello)** to outbound DNS-over-HTTPS connections over HTTP/2 and HTTP/3.

Latest release: **[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**. The latest GitHub tag, executable, and web interface use the same version string: `v0.107.79.2`.

This is an independently maintained, AI-assisted modification, not an official AdGuard release. Upstream source, copyright notices, and licenses are retained.

## OpenWrt installation and upgrades

See the [step-by-step OpenWrt guide](docs/OPENWRT.md) ([简体中文](docs/OPENWRT.zh-CN.md)) for CPU/firmware architecture selection, checksums, actual installation paths, backups, executable replacement, DNS/ECH verification, and rollback.

Linux packages follow the official 12-architecture matrix: `amd64`, `386`, `arm64`, `armv5`, `armv6`, `armv7`, `mips_softfloat`, `mipsle_softfloat`, `mips64_softfloat`, `mips64le_softfloat`, `ppc64le`, `riscv64`. Match the firmware ABI and endianness, and run the candidate's `--version` before stopping the current service. Existing x86_64 and macOS Intel downloads remain available in the release assets. The additional architectures are cross-compiled and inspected; they have not all been tested on physical routers.

## Changes

- **Upstream ECH:** explicitly enable ECH for selected `https://` and `h3://` upstream hostnames.
- **Independent discovery:** obtain ECH configurations from HTTPS DNS records using separately configured resolvers. Cache them according to DNS TTL and refresh on demand after expiry.
- **Handshake verification:** retain normal certificate verification and send DNS requests only after ECH is accepted. Retry the handshake at most once when an authenticated replacement configuration is received.
- **Failure handling:** an upstream requiring ECH returns an upstream error on failure, leaving the existing fallback mechanism to handle it. There is no silent retry with ECH disabled. Each fallback keeps its own policy.
- **Connection reuse:** retain HTTP/2 and HTTP/3 connection reuse; disable TLS session resumption and H3 0-RTT for ECH connections.
- **Query status:** append `(ECH)` to the existing DNS server address to show actual use.

The scope is outbound DoH/H3 client connections from AdGuard Home. It does not include downstream server-side ECH, DoT/DoQ ECH, or complete SVCB alternative-target routing. ECH hides the real service hostname in the handshake; destination IP addresses and traffic metadata remain visible.

## Enabling ECH

The example below uses **Crypto.sx** and **RethinkDNS Sky**, both verified with H3 and accepted ECH in our test network. Merge it into your existing `dns` section:

```yaml
dns:
  upstream_dns:
    - h3://doh.crypto.sx/dns-query
    - h3://sky.rethinkdns.com/
  upstream_ech:
    doh.crypto.sx:
      - sdns://AQcAAAAAAAAAEjk0LjE0MC4xNC4xNDA6NTQ0MyC16ETWuDo-PhJo62gfvqcN48X6aNvWiBQdvy7AZrLa-iUyLmRuc2NyeXB0LnVuZmlsdGVyZWQubnMxLmFkZ3VhcmQuY29t
    sky.rethinkdns.com:
      - sdns://AQcAAAAAAAAAEjk0LjE0MC4xNC4xNDA6NTQ0MyC16ETWuDo-PhJo62gfvqcN48X6aNvWiBQdvy7AZrLa-iUyLmRuc2NyeXB0LnVuZmlsdGVyZWQubnMxLmFkZ3VhcmQuY29t
```

This is a configuration fragment, not a complete configuration. Keep your existing `upstream_dns`, `bootstrap_dns`, `fallback_dns`, and other settings. Keys in `upstream_ech` are lowercase hostnames; values are lists of independent resolvers used to obtain ECH configurations.

The discovery resolver must be reachable from your network, return HTTPS records containing ECH parameters, and have an endpoint containing an IP address. DNS stamps containing IP addresses are also supported. The `sdns://` stamp selects AdGuard’s unfiltered DNSCrypt resolver at `94.140.14.140:5443`, independently verified for ECH discovery. Crypto.sx and RethinkDNS handle ordinary queries; the DNSCrypt resolver obtains their ECH configurations. Do not point discovery at AdGuard Home itself, as that can create a resolution dependency loop.

Restart AdGuard Home after editing the configuration. This option is currently configured through YAML; no web toggle has been added. Removing a hostname's policy restores that upstream's original behavior. See the commented [configuration example](ech-config.example.yaml).

## Confirming ECH

Open **Response details → DNS server** in the query log. A query that actually used ECH displays:

```text
https://doh.crypto.sx:443/dns-query (ECH)
```

The marker comes from the TLS state of the connection used for that successful HTTP response. It works with reused connections and follows the main or fallback upstream that supplied the final answer. Cached responses, local responses, and older records without ECH metadata are not marked.

Log storage and the API preserve the status in a separate optional field. The original upstream address remains unchanged; no extra detail row or setting is added.

## Versions and official bases

| Personal release | Changes |
| --- | --- |
| [`v0.107.79.1`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.1) | Initial upstream ECH support |
| [`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2) | Per-query ECH status and consistent latest-version labeling |

Both releases use the following official sources; their personal version numbers do not imply corresponding official releases:

| Component | Official version | Base commit |
| --- | --- | --- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | `v0.107.79` | `05ba17b282da1c4393d6a4ba4db0cf519194a362` |
| [dnsproxy](https://github.com/AdguardTeam/dnsproxy) | `v0.83.2` | `acf2b30e05202e171bbbaf4a3176ea07edb964b3` |

This repository contains the complete source of both modules. Module import paths and dependency versions are retained; the root `go.work` selects the bundled dnsproxy. [SOURCE-MANIFEST.json](SOURCE-MANIFEST.json) records SHA-256 hashes of modified source files.

## Building from source

Builds have been verified with Go **1.27.1** and Node.js **26.3.0**. With Go, Node.js/npm, and `shasum` available, run from the repository root:

```sh
./build.sh
```

For the full Linux matrix, use `sh scripts/build-linux.sh`; pass suffixes such as `arm64 armv7` to build selected targets. It builds the frontend once and writes executables plus `SHA256SUMS-linux` to `artifacts/`.

The script builds and embeds the frontend, then writes Linux amd64 and macOS amd64 executables and SHA-256 checksums to `artifacts/`. The executable version comes from the root [VERSION](VERSION) file. Set `GO=/path/to/go` to select a Go executable.

## Validation

ECH tests against local TLS/H2/H3 endpoints, related race tests, and real upstream H2/H3 ECH exchanges have passed. OpenWrt deployments have also been checked for DNS resolution and web-interface status display.

Coverage includes ECH acceptance, configuration rotation, rejection, missing or malformed records, certificate failures, discovery-resolver switching, caching, concurrency, and fallback. Release `.2` additionally covers connection reuse, final-answer attribution, log persistence, older-record compatibility, and frontend display.

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

All five frontend display tests, TypeScript checks, and ESLint checks on modified files have passed. After running `npm ci` in `AdGuardHome/client`, run these commands from the repository root:

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

The complete upstream test suite has not passed in full: some existing tests depend on public services such as `8.8.8.8:53` and `1.1.1.1:53`, which time out in the test network. Full upstream CI, including `make go-check`, remains unverified.

## Upstream contribution

A design discussion was submitted to [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573). The proposed path is to integrate transport support into dnsproxy first, then address AdGuard Home configuration and dependency updates, pending maintainer feedback. No PR has been submitted, and this implementation has not been officially accepted.

## Licenses

AdGuard Home and this combined distribution retain GPL-3.0 licensing; see [LICENSE](LICENSE) and the [upstream license](AdGuardHome/LICENSE.txt). dnsproxy retains its [Apache-2.0 license](dnsproxy/LICENSE). Original copyright notices and third-party license notices are preserved.

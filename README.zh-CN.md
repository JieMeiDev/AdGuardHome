# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-lightgrey)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-blue)](README.zh-CN.md)

基于官方 **AdGuard Home v0.107.79** 的修改版，为 DoH 上游连接增加 **ECH（Encrypted Client Hello）** 支持，适用于 HTTP/2 和 HTTP/3。

**当前版本：[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**

本项目由个人维护、OpenAI 协助开发，非 AdGuard 官方发布。原项目的源码、版权和许可证均予保留。

## 功能

- 对指定域名的 `https://`、`h3://` 上游启用 ECH，隐藏 TLS 握手中的真实服务域名。
- 通过独立解析器获取 ECH 配置，按 DNS TTL 缓存，过期后按需刷新。
- 保留证书验证；ECH 未被接受时不会向该上游发送 DNS 查询，也不会自动降级为普通连接。失败后交由现有 fallback 机制处理，各备用上游保留自己的策略。
- 在查询详情的 **DNS server** 地址后显示 `(ECH)`，反映实际握手结果。

支持连接复用；ECH 连接禁用 TLS 会话恢复和 H3 0-RTT。收到经过认证的新 ECH 配置时，最多重试一次握手。

范围限于 AdGuard Home 到上游的 DoH/H3 客户端连接，不包含下游服务端 ECH、DoT/DoQ ECH 或完整的 SVCB 替代目标调度。目标 IP 和流量元数据仍然可见。

## 启用 ECH

上游服务器必须支持 ECH，并发布可用的 ECH 配置。**解析器能够返回其他网站的 ECH 记录，不代表连接该解析器自身时也能使用 ECH。**

以下使用已在测试网络验证 H3 与 ECH 握手成功的 **Crypto.sx** 和 **RethinkDNS Sky**，让独立 DNS 服务商承接日常查询。将示例合入现有配置的 `dns` 段：

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

这里有三类不同的设置：

| 设置 | 用途 |
| --- | --- |
| `upstream_dns` / `fallback_dns` | 处理日常 DNS 查询的主上游与备用上游 |
| `bootstrap_dns` | 解析上游服务器的 IP 地址 |
| `upstream_ech` | 为指定上游获取 HTTPS 记录中的 ECH 配置，包括公钥等握手参数 |

`upstream_ech` 的键使用小写域名，值为独立解析器列表。示例中的 `sdns://` 指定 AdGuard 无过滤 DNSCrypt 解析器 `94.140.14.140:5443`，已独立验证可以获取这两家上游的 ECH 配置。它在这里仅负责参数发现，日常查询由 Crypto.sx 和 RethinkDNS 处理。端点必须包含 IP 地址，也支持内含 IP 的 `sdns://` DNS stamp。不要指向 AdGuard Home 自身，以免形成循环依赖。

此示例不是完整配置。合并时保留原有的上游、`bootstrap_dns`、`fallback_dns` 及其他设置，修改后重启 AdGuard Home。该选项目前通过 YAML 配置；删除对应域名的策略即可恢复该上游原有行为。更多说明见[配置示例](ech-config.example.yaml)。

## 确认 ECH 生效

打开查询日志的 **Response details → DNS server**，实际使用 ECH 的查询会显示：

```text
https://doh.crypto.sx:443/dns-query (ECH)
```

标记取自成功 HTTP 响应所用连接的 TLS 状态，支持连接复用，并跟随最终提供答案的主上游或备用上游。缓存响应、本地响应及没有 ECH 元数据的旧记录不显示标记。

查询日志 API 通过可选布尔字段 `upstream_ech` 返回状态；日志会持久化该状态，原始上游地址保持不变。

## 更新现有安装

1. 获取对应架构的程序包，核对 `SHA256SUMS` 后解压。
2. 备份现有程序和配置，停止 AdGuard Home 服务。
3. 用包内的 `AdGuardHome/AdGuardHome` 替换原可执行文件，保留配置、工作目录和服务脚本。
4. 启动服务，检查版本和 DNS 解析。

```sh
tar -xzf AdGuardHome_linux_amd64.tar.gz
./AdGuardHome/AdGuardHome --version
# AdGuard Home, version v0.107.79.2
```

OpenWrt 使用的是独立程序压缩包，不是 `.ipk`。首次安装可参考 [AdGuard Home 官方说明](AdGuardHome/README.md)。

## 从源码构建

已使用 Go **1.27.1**、Node.js **26.3.0** 验证构建。准备 Go、Node.js/npm 和 `shasum` 后，在仓库根目录运行：

```sh
./build.sh
```

脚本会构建并嵌入 Web 界面，将 Linux amd64、macOS amd64 可执行文件及 SHA-256 校验文件输出到 `artifacts/`。程序版本读取自根目录的 [VERSION](VERSION)。可通过 `GO=/path/to/go` 指定 Go 路径。

## 验证情况

已验证本地 TLS/H2/H3 端点、真实上游的 ECH 交换、相关 race 测试，以及 OpenWrt 部署后的解析和页面显示。

覆盖 ECH 接受与拒绝、配置轮换、记录缺失或损坏、证书失败、解析器切换、缓存、并发和 fallback；查询状态还覆盖连接复用、最终回答归属、日志持久化与旧记录兼容。

在仓库根目录执行相关 Go 测试：

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

前端的 5 项显示测试、TypeScript 检查及修改文件的 ESLint 检查已通过。先在 `AdGuardHome/client` 执行 `npm ci`，再从仓库根目录运行：

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

完整上游测试尚未全部通过：部分原有测试访问 `8.8.8.8:53`、`1.1.1.1:53` 时超时；包括 `make go-check` 在内的完整上游 CI 仍待验证。

## 上游贡献

已在 [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573) 提交设计讨论，建议先向 dnsproxy 集成传输支持，再处理 AdGuard Home 配置和依赖更新。目前尚未提交 PR，也未获官方接受。

## 许可证

AdGuard Home 及本组合发行保留 GPL-3.0 许可，见 [LICENSE](LICENSE) 和[原项目许可证](AdGuardHome/LICENSE.txt)。dnsproxy 保留 [Apache-2.0 许可证](dnsproxy/LICENSE)。各文件原有的版权与第三方许可声明均予保留。

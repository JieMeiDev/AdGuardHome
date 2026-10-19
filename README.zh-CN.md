# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-lightgrey)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-blue)](README.zh-CN.md)

基于官方 **AdGuard Home v0.107.79** 的修改版，为 AdGuard Home 到 DoH 上游的连接增加 **ECH（Encrypted Client Hello）** 支持，兼容 HTTP/2 和 HTTP/3。

当前最新版：**[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**。GitHub 标签、最新版程序及页面均显示 `v0.107.79.2`。

这是个人维护、由 OpenAI 辅助开发的修改版，不是 AdGuard 官方发布。官方项目的源码、版权及许可证保留在仓库中。

## 现有安装更新

1. 下载适合设备架构的程序包，核对 `SHA256SUMS` 后解压。
2. 备份现有程序及配置，停止现有 AdGuard Home 服务。
3. 用包内的 `AdGuardHome/AdGuardHome` 替换原可执行文件，保留原配置、工作目录和服务配置。
4. 启动服务，检查页面版本与 DNS 解析。

例如，Linux 程序包可这样解压并查看版本：

```sh
tar -xzf AdGuardHome_linux_amd64.tar.gz
./AdGuardHome/AdGuardHome --version
# AdGuard Home, version v0.107.79.2
```

OpenWrt 下载项是独立程序压缩包，不是 `.ipk`；现有安装继续使用原服务脚本。首次安装可参考 [AdGuard Home 官方项目说明](AdGuardHome/README.md)。

## 本版改动

- **上游 ECH**：对指定域名的 `https://`、`h3://` 上游显式启用 ECH。
- **独立引导解析**：通过单独配置的解析器获取 HTTPS 记录中的 ECH 配置，按 DNS TTL 缓存，过期后按需刷新。
- **握手验证**：保留正常证书验证，确认 ECH 被接受后才发送 DNS 请求；收到经过认证的新配置时，最多重试一次握手。
- **失败处理**：要求 ECH 的上游失败时返回上游错误，由原有 fallback 机制处理；不会自动关闭该上游的 ECH 重试。各 fallback 保留自己的策略。
- **连接复用**：保留 HTTP/2、HTTP/3 连接复用；ECH 路径禁用 TLS 会话恢复及 H3 0-RTT。
- **查询状态**：在原有 DNS server 地址后显示 `(ECH)`，方便确认实际使用情况。

目前范围是 AdGuard Home 到上游的 DoH/H3 客户端连接，不包含下游服务端 ECH、DoT/DoQ ECH 或完整的 SVCB 替代目标调度。ECH 隐藏握手中的真实服务域名，目标 IP 和流量元数据仍然可见。

## 启用 ECH

在现有配置的 `dns` 段合入以下片段，将 `dns.example.net` 换成实际的上游域名：

```yaml
dns:
  upstream_ech:
    dns.example.net:
      - https://1.1.1.1/dns-query
```

这只是配置片段，请保留原有 `upstream_dns`、`bootstrap_dns` 和 `fallback_dns` 等设置。`upstream_ech` 的键使用小写域名，值为获取 ECH 配置的独立解析器列表。

独立解析器必须在你的网络中可达、能返回带 ECH 参数的 HTTPS 记录，且端点包含 IP 地址；也支持内含 IP 的 DNS stamp。上面的地址仅作格式示例。不要指向 AdGuard Home 自身，以免形成解析依赖循环。

配置后重启 AdGuard Home。该选项目前通过 YAML 设置，没有新增网页开关；删除对应域名的策略可恢复该上游原有行为。更多注释见 [配置示例](ech-config.example.yaml)。

## 如何确认 ECH

打开查询日志中的 **Response details → DNS server**。实际使用 ECH 的查询会显示：

```text
https://dns.example.net:443/dns-query (ECH)
```

标记来自该次成功 HTTP 响应所用连接的 TLS 状态，支持连接复用，并跟随最终返回答案的主上游或 fallback。缓存、本地响应和没有 ECH 元数据的旧记录不标记。

日志存储和 API 通过独立的可选字段保存状态，原始上游地址保持不变；没有新增详情行或设置项。

## 从源码构建

已使用 Go **1.27.1**、Node.js **26.3.0** 验证构建。准备 Go、Node.js/npm 和 `shasum` 后，在仓库根目录运行：

```sh
./build.sh
```

脚本会构建并嵌入前端，输出 Linux amd64、macOS amd64 可执行文件及 SHA-256 校验文件到 `artifacts/`。程序版本从根目录的 [VERSION](VERSION) 读取。若需指定 Go 路径，可设置 `GO=/path/to/go`。

## 验证情况

已通过本地真实 TLS/H2/H3 端点的 ECH 测试、相关 race 测试及真实上游 H2/H3 ECH 交换验证。OpenWrt 部署也已验证解析及页面显示。

测试覆盖 ECH 接受、配置轮换、拒绝、缺失或损坏记录、证书失败、引导解析切换、缓存、并发及 fallback。`.2` 还覆盖连接复用、最终回答归属、日志持久化、旧记录兼容和前端显示。

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

前端的 5 项显示测试、TypeScript 检查及修改文件的 ESLint 检查已通过。在 `AdGuardHome/client` 执行 `npm ci` 后，可从仓库根目录运行：

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

完整上游测试尚未全部通过：部分原有测试依赖 `8.8.8.8:53`、`1.1.1.1:53` 等公网服务，在测试网络中超时；完整上游 CI（包括 `make go-check`）仍待验证。

## 上游贡献

已在 [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573) 提交设计讨论。拟先向 dnsproxy 集成传输支持，再处理 AdGuard Home 配置及依赖更新，等待维护者反馈。当前未提交 PR，也不代表官方已接受该实现。

## 许可证

AdGuard Home 及本组合发行保留 GPL-3.0 许可，见 [LICENSE](LICENSE) 和 [原项目许可证](AdGuardHome/LICENSE.txt)。dnsproxy 保留 [Apache-2.0 许可证](dnsproxy/LICENSE)，各文件的原始版权及第三方许可声明均予保留。

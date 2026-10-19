# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-lightgrey)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-blue)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](README.zh-CN.md)

基於官方 **AdGuard Home v0.107.79** 的個人修改版，為 AdGuard Home 到 DoH 上游的連線增加 **ECH（Encrypted Client Hello）** 支援，相容 HTTP/2 和 HTTP/3。

當前最新版：**[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**。GitHub 標籤、最新版程式及頁面均顯示 `v0.107.79.2`。

這是個人維護、由 AI 輔助開發的修改版，不是 AdGuard 官方釋出。官方專案的原始碼、版權及許可證保留在倉庫中。

## OpenWrt 安裝與升級

請依照 [OpenWrt 完整教學（簡體中文）](docs/OPENWRT.zh-CN.md)或[英文教學](docs/OPENWRT.md)操作，內容涵蓋 CPU 與韌體架構、校驗、實際路徑、備份、程式替換、DNS/ECH 驗證及還原。

Linux 套件依官方 12 種架構提供：amd64、386、ARM64、ARMv5/v6/v7、MIPS/MIPSLE/MIPS64/MIPS64LE softfloat、PowerPC64LE、RISC-V64。請核對韌體 ABI 與大小端，在停止舊服務前先執行新程式的 `--version`。新增架構已交叉編譯並檢查產物，但並非全部經過路由器實機驗證。

## 本版改動

- **上游 ECH**：對指定域名的 `https://`、`h3://` 上游顯式啟用 ECH。
- **獨立引導解析**：透過單獨配置的解析器獲取 HTTPS 記錄中的 ECH 配置，按 DNS TTL 快取，過期後按需重新整理。
- **握手驗證**：保留正常證書驗證，確認 ECH 被接受後才傳送 DNS 請求；收到經過認證的新配置時，最多重試一次握手。
- **失敗處理**：要求 ECH 的上游失敗時返回上游錯誤，由原有 fallback 機制處理；不會自動關閉該上游的 ECH 重試。各 fallback 保留自己的策略。
- **連線複用**：保留 HTTP/2、HTTP/3 連線複用；ECH 路徑禁用 TLS 會話恢復及 H3 0-RTT。
- **查詢狀態**：在原有 DNS server 地址後顯示 `(ECH)`，方便確認實際使用情況。

目前範圍是 AdGuard Home 到上游的 DoH/H3 客戶端連線，不包含下游服務端 ECH、DoT/DoQ ECH 或完整的 SVCB 替代目標排程。ECH 隱藏握手中的真實服務域名，目標 IP 和流量後設資料仍然可見。

## 啟用 ECH

以下使用已在測試網路驗證 H3 與 ECH 握手成功的 **Crypto.sx** 和 **RethinkDNS Sky**。請將片段合併至現有設定的 `dns` 區段：

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

這只是配置片段，請保留原有 `upstream_dns`、`bootstrap_dns` 和 `fallback_dns` 等設定。`upstream_ech` 的鍵使用小寫域名，值為獲取 ECH 配置的獨立解析器列表。

獨立解析器必須在你的網路中可達、能返回帶 ECH 引數的 HTTPS 記錄，且端點包含 IP 地址；也支援內含 IP 的 DNS stamp。`sdns://` 指定的是已獨立驗證可取得 ECH 設定的 AdGuard 無過濾 DNSCrypt 解析器 `94.140.14.140:5443`。Crypto.sx 和 RethinkDNS 處理日常查詢，這個 DNSCrypt 解析器只在此處負責取得它們的 ECH 設定。不要指向 AdGuard Home 自身，以免形成解析依賴迴圈。

配置後重啟 AdGuard Home。該選專案前透過 YAML 設定，沒有新增網頁開關；刪除對應域名的策略可恢復該上游原有行為。更多註釋見 [配置示例](ech-config.example.yaml)。

## 如何確認 ECH

開啟查詢日誌中的 **Response details → DNS server**。實際使用 ECH 的查詢會顯示：

```text
https://doh.crypto.sx:443/dns-query (ECH)
```

標記來自該次成功 HTTP 響應所用連線的 TLS 狀態，支援連線複用，並跟隨最終返回答案的主上游或 fallback。快取、本地響應和沒有 ECH 後設資料的舊記錄不標記。

日誌儲存和 API 透過獨立的可選欄位儲存狀態，原始上游地址保持不變；沒有新增詳情行或設定項。

## 版本與官方基線

| 個人版本 | 程式顯示版本 | 改動 |
| --- | --- | --- |
| [`v0.107.79.1`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.1) | `v0.107.79-ech.1`（歷史標識） | 加入上游 ECH 支援 |
| [`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2) | `v0.107.79.2` | 增加查詢日誌 ECH 狀態，並統一最新版版本標識 |

兩個版本均基於以下官方原始碼，不代表官方已經發布對應版本：

| 元件 | 官方版本 | 基線提交 |
| --- | --- | --- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | `v0.107.79` | `05ba17b282da1c4393d6a4ba4db0cf519194a362` |
| [dnsproxy](https://github.com/AdguardTeam/dnsproxy) | `v0.83.2` | `acf2b30e05202e171bbbaf4a3176ea07edb964b3` |

倉庫同時包含兩個模組的完整原始碼。模組匯入路徑和依賴版本保持原樣，根目錄的 `go.work` 選擇倉庫內的 dnsproxy；[SOURCE-MANIFEST.json](SOURCE-MANIFEST.json) 記錄修改檔案的 SHA-256。

## 從原始碼構建

已使用 Go **1.27.1**、Node.js **26.3.0** 驗證構建。準備 Go、Node.js/npm 和 `shasum` 後，在倉庫根目錄執行：

```sh
./build.sh
```

完整 Linux 架構可使用 `sh scripts/build-linux.sh`；加上 `arm64 armv7` 等參數可限定目標。腳本只建置一次前端，將執行檔與 `SHA256SUMS-linux` 輸出至 `artifacts/`。

指令碼會構建並嵌入前端，輸出 Linux amd64、macOS amd64 可執行檔案及 SHA-256 校驗檔案到 `artifacts/`。程式版本從根目錄的 [VERSION](VERSION) 讀取。若需指定 Go 路徑，可設定 `GO=/path/to/go`。

## 驗證情況

已透過本地真實 TLS/H2/H3 端點的 ECH 測試、相關 race 測試及真實上游 H2/H3 ECH 交換驗證。OpenWrt 部署也已驗證解析及頁面顯示。

測試覆蓋 ECH 接受、配置輪換、拒絕、缺失或損壞記錄、證書失敗、引導解析切換、快取、併發及 fallback。`.2` 還覆蓋連線複用、最終回答歸屬、日誌持久化、舊記錄相容和前端顯示。

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

前端的 5 項顯示測試、TypeScript 檢查及修改檔案的 ESLint 檢查已透過。在 `AdGuardHome/client` 執行 `npm ci` 後，可從倉庫根目錄執行：

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

完整上游測試尚未全部透過：部分原有測試依賴 `8.8.8.8:53`、`1.1.1.1:53` 等公網服務，在測試網路中超時；完整上游 CI（包括 `make go-check`）仍待驗證。

## 上游貢獻

已在 [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573) 提交設計討論。擬先向 dnsproxy 整合傳輸支援，再處理 AdGuard Home 配置及依賴更新，等待維護者反饋。當前未提交 PR，也不代表官方已接受該實現。

## 許可證

AdGuard Home 及本組合發行保留 GPL-3.0 許可，見 [LICENSE](LICENSE) 和 [原專案許可證](AdGuardHome/LICENSE.txt)。dnsproxy 保留 [Apache-2.0 許可證](dnsproxy/LICENSE)，各檔案的原始版權及第三方許可宣告均予保留。

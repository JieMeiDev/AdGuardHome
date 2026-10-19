# OpenWrt 安裝與升級

[![English](https://img.shields.io/badge/Language-English-lightgrey)](OPENWRT.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](OPENWRT.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](OPENWRT.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-blue)](OPENWRT.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-CN.md)

[README](../README.zh-TW.md)

本教程用於替換**已經正常執行的 OpenWrt AdGuard Home** 的可執行檔案，保留現有服務、執行賬戶、配置、工作目錄、DNS 埠及 DHCP 設定。路由器上的命令透過 SSH 以 root 執行，無須重啟路由器。

首次安裝請先安裝並配置 [OpenWrt 官方 AdGuard Home 軟體包](https://openwrt.org/docs/guide-user/services/dns/adguard-home)。OpenWrt 25.12 及以後使用 `apk`，更早的受支援版本使用 `opkg`。先確認普通 DNS 解析正常，再按本教程升級。本專案提供的是獨立程式壓縮包，不是 `.apk` 或 `.ipk`。

## 1. 識別架構

在路由器上執行：

```sh
uname -m
ubus call system board
if command -v apk >/dev/null 2>&1; then
    apk --print-arch
else
    opkg print-architecture
fi
```

選擇程式包時，需要同時確認 **CPU 架構和韌體的使用者態 ABI**，不能只看裝置型號或宣傳中的“64 位”。64 位 CPU 也可能執行 32 位韌體。MIPS 的 `uname -m` 不一定能區分大小端；例如包架構 `mipsel_24kc` 是小端，`mips_24kc` 是大端。不確定時，可在工具可用的情況下檢查 `readelf -h /bin/busybox`，或查閱韌體目標架構說明，再選擇程式包。

Linux 程式包命名參考[官方 AdGuard Home v0.107.79](https://github.com/AdguardTeam/AdGuardHome/releases/tag/v0.107.79)：

| 路由器／韌體架構 | 包名字尾（`agh_arch`） | Go 編譯目標 |
| --- | --- | --- |
| `x86_64` | `amd64` | `amd64` |
| `i386 / i686` | `386` | `386` |
| `aarch64` | `arm64` | `arm64` |
| `armv5` | `armv5` | `arm`, `GOARM=5` |
| `armv6` | `armv6` | `arm`, `GOARM=6` |
| `armv7` | `armv7` | `arm`, `GOARM=7` |
| `mips` | `mips_softfloat` | `mips`, `GOMIPS=softfloat` |
| `mipsle` | `mipsle_softfloat` | `mipsle`, `GOMIPS=softfloat` |
| `mips64` | `mips64_softfloat` | `mips64`, `GOMIPS64=softfloat` |
| `mips64le` | `mips64le_softfloat` | `mips64le`, `GOMIPS64=softfloat` |
| `ppc64le` | `ppc64le` | `ppc64le` |
| `riscv64` | `riscv64` | `riscv64` |

完整包名為 `AdGuardHome_linux_<后缀>.tar.gz`。OpenWrt 不能使用 macOS 的 `darwin` 包。CPU 指令集、浮點支援、使用者態 ABI 和核心相容性仍需匹配，因此**必須在停止舊服務前，先在路由器上執行新程式的 `--version`**。新增架構經過交叉編譯和產物檢查，不代表已在每一種路由器上實機驗證。

## 2. 確認現有安裝路徑

```sh
cat /etc/config/adguardhome
cat /etc/init.d/adguardhome
ps w | grep '[A]dGuardHome'
df -h /tmp /usr/bin /root
```

常見路徑如下：

| 專案 | 常見位置 |
| --- | --- |
| 可執行檔案 | `/usr/bin/AdGuardHome`；也可能自定義為 `/opt/AdGuardHome/AdGuardHome` 等路徑 |
| YAML 配置 | 新版常用 `/etc/adguardhome/adguardhome.yaml`；部分舊版使用 `/etc/adguardhome.yaml` |
| 工作目錄 | `/var/lib/adguardhome`，或自定義路徑 |
| 服務指令碼 | `/etc/init.d/adguardhome` |

新版 UCI 配置使用 `config_file`、`work_dir`，舊版可能使用 `config`、`workdir`。程序引數中的 `-c`／`--config`、`-w`／`--work-dir` 也能幫助確認實際路徑。**不要把新版的配置檔案隨意移到 `/etc` 根目錄**：服務可能要求獨立配置目錄，並使用 `adguardhome` 賬戶及 jail 執行。

按實際情況設定以下變數，並在後續步驟中保持使用同一個 SSH 會話：

```sh
agh_bin=/usr/bin/AdGuardHome
agh_config=/etc/adguardhome/adguardhome.yaml
agh_work=/var/lib/adguardhome
agh_service=/etc/init.d/adguardhome
```

路徑不明確、有多個例項執行或空間不足時，先不要繼續。持久化儲存需要容納舊程式備份和待替換的新程式；`/tmp` 需要容納壓縮包和解壓後的程式。必要時將備份另存到電腦。歷史查詢日誌和統計仍保留在工作目錄，如需完整資料備份，請另外備份該目錄。

## 3. 下載與校驗

下面以 **ARM64** 為例，請按架構表修改 `agh_arch`：

```sh
agh_version=v0.107.79.2
agh_arch=arm64
agh_archive="AdGuardHome_linux_${agh_arch}.tar.gz"
agh_stage=$(mktemp -d /tmp/agh-upgrade.XXXXXX)
agh_release="https://github.com/JieMeiDev/AdGuardHome/releases/download/${agh_version}"
cd "$agh_stage"
wget -O "$agh_archive" "$agh_release/$agh_archive"
wget -O SHA256SUMS "$agh_release/SHA256SUMS"
awk -v name="$agh_archive" '$2 == name { print }' SHA256SUMS > archive.sha256
test -s archive.sha256
sha256sum -c archive.sha256
```

只有下載成功、校驗結果顯示 `OK` 後才繼續。這裡先篩選對應包的一條校驗記錄，避免依賴 BusyBox 不一定支援的 `--ignore-missing`。

如果路由器不能訪問 GitHub，可先在電腦下載對應程式包和 `SHA256SUMS`，再透過 SCP／SFTP 上傳到臨時目錄。舊版 OpenWrt SSH 服務不支援 SFTP 時，可使用 `scp -O`。上傳後仍按上面的方式在路由器校驗，不要關閉 HTTPS 證書驗證來繞過下載問題。

```sh
tar -xzf "$agh_archive"
agh_candidate="$agh_stage/AdGuardHome/AdGuardHome"
chmod 755 "$agh_candidate"
"$agh_candidate" --version
"$agh_candidate" --check-config -c "$agh_config" -w "$agh_work"
```

版本應為 `v0.107.79.2`，配置檢查也必須成功。出現 `Exec format error`、`Illegal instruction`、找不到可執行檔案直譯器或檢查失敗時，**不要替換正在執行的程式**，先確認架構和韌體相容性。

## 4. 備份、停止服務並替換

建立許可權受限的持久化備份：

```sh
umask 077
agh_backup="/root/agh-backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$agh_backup"
cp -p "$agh_bin" "$agh_backup/AdGuardHome"
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
cp -p "$agh_service" "$agh_backup/service.init"
cp -p /etc/config/adguardhome "$agh_backup/service.uci"
printf '%s\n' "$agh_backup"
```

任一備份命令失敗，都應先處理再繼續。先把新程式暫存到**目標檔案所在目錄**，使最終重新命名發生在同一檔案系統；先複製原檔案，以保留屬主、屬組和許可權：

```sh
cp -p "$agh_bin" "$agh_bin.ech-new"
cat "$agh_candidate" > "$agh_bin.ech-new"
cmp "$agh_candidate" "$agh_bin.ech-new"
```

複製和比較均成功後，停止服務：

```sh
"$agh_service" stop
sleep 2
pidof AdGuardHome
```

`pidof` 應不再返回 PID。如果舊程序仍然存在，不要替換，先檢查服務或多例項情況。AdGuard Home 停止時可能儲存配置，因此要在**停止後**更新配置備份：

```sh
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
mv "$agh_bin.ech-new" "$agh_bin"
"$agh_bin" --version
"$agh_service" start
```

服務停止後若有命令失敗，請按下文回滾。不要刪除備份或原工作目錄，也不要直接向正在執行的可執行檔案寫入，否則可能報 `Text file busy`。

## 5. 驗證 DNS 和 ECH

```sh
pidof AdGuardHome
"$agh_bin" --version
logread -e AdGuardHome | tail -n 30
nslookup example.org 127.0.0.1
```

確認 Web 頁面版本正確，服務正常啟動。上面的 `nslookup` 檢查路由器本機 53 埠的解析路徑，可能經過 dnsmasq。如需從裝有 `dig` 的電腦檢查其他埠，使用實際地址和埠，例如 `dig @192.168.1.1 -p 5335 example.org`。**5335 只是示例，不要求改成這個埠**；已有安裝可能使用 53、54 或其他埠。

升級驗證成功後，再停止服務，將 [ECH 配置示例](../ech-config.example.yaml)合入現有 `dns` 段並啟動。保留本地 DNS／DHCP 設定、引導解析器和所需的備用策略。支援 ECH 的程式不會自動為所有上游啟用 ECH。

在未命中快取的查詢中，**Response details → DNS server** 後出現 `(ECH)`，才表示實際選擇的上游接受了 ECH。快取和本地響應不會顯示標記。測試時可查詢尚未快取的域名，或先在 Web 頁面清空 DNS 快取。

## 6. 回滾

繼續使用此前確認的路徑變數，以及輸出的 `agh_backup` 目錄。重新連線 SSH 後，應先重新設定這些變數，再執行：

```sh
"$agh_service" stop
cp -p "$agh_backup/AdGuardHome" "$agh_bin.rollback"
mv "$agh_bin.rollback" "$agh_bin"
cp -p "$agh_backup/adguardhome.yaml" "$agh_config"
"$agh_service" start
"$agh_bin" --version
nslookup example.org 127.0.0.1
```

需要同時恢復程式和 YAML，因為舊版官方程式可能不理解此修改版的設定。若另行修改過服務指令碼和 UCI，也可使用相應備份；本教程的替換流程不會修改它們。

## 後續升級

每次升級都重複架構確認、校驗、備份和執行檢查。官方頁面自動更新、包管理器重灌／升級，以及韌體升級，都可能將可執行檔案覆蓋為官方版本，或改變服務配置。之後應再次確認版本與 ECH 行為，不要假定自定義程式能跨 OpenWrt 升級保留。保留持久化備份，並將重要配置另存一份到電腦。

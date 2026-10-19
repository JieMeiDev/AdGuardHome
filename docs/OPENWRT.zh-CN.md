# OpenWrt 安装与升级

[![English](https://img.shields.io/badge/Language-English-lightgrey)](OPENWRT.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](OPENWRT.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](OPENWRT.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-blue)](OPENWRT.zh-CN.md)

[README](../README.zh-CN.md)

本教程用于替换**已经正常运行的 OpenWrt AdGuard Home** 的可执行文件，保留现有服务、运行账户、配置、工作目录、DNS 端口及 DHCP 设置。路由器上的命令通过 SSH 以 root 执行，无须重启路由器。

首次安装请先安装并配置 [OpenWrt 官方 AdGuard Home 软件包](https://openwrt.org/docs/guide-user/services/dns/adguard-home)。OpenWrt 25.12 及以后使用 `apk`，更早的受支持版本使用 `opkg`。先确认普通 DNS 解析正常，再按本教程升级。本项目提供的是独立程序压缩包，不是 `.apk` 或 `.ipk`。

## 1. 识别架构

在路由器上执行：

```sh
uname -m
ubus call system board
if command -v apk >/dev/null 2>&1; then
    apk --print-arch
else
    opkg print-architecture
fi
```

选择程序包时，需要同时确认 **CPU 架构和固件的用户态 ABI**，不能只看设备型号或宣传中的“64 位”。64 位 CPU 也可能运行 32 位固件。MIPS 的 `uname -m` 不一定能区分大小端；例如包架构 `mipsel_24kc` 是小端，`mips_24kc` 是大端。不确定时，可在工具可用的情况下检查 `readelf -h /bin/busybox`，或查阅固件目标架构说明，再选择程序包。

Linux 程序包命名参考[官方 AdGuard Home v0.107.79](https://github.com/AdguardTeam/AdGuardHome/releases/tag/v0.107.79)：

| 路由器／固件架构 | 包名后缀（`agh_arch`） | Go 编译目标 |
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

完整包名为 `AdGuardHome_linux_<后缀>.tar.gz`。OpenWrt 不能使用 macOS 的 `darwin` 包。CPU 指令集、浮点支持、用户态 ABI 和内核兼容性仍需匹配，因此**必须在停止旧服务前，先在路由器上执行新程序的 `--version`**。新增架构经过交叉编译和产物检查，不代表已在每一种路由器上实机验证。

## 2. 确认现有安装路径

```sh
cat /etc/config/adguardhome
cat /etc/init.d/adguardhome
ps w | grep '[A]dGuardHome'
df -h /tmp /usr/bin /root
```

常见路径如下：

| 项目 | 常见位置 |
| --- | --- |
| 可执行文件 | `/usr/bin/AdGuardHome`；也可能自定义为 `/opt/AdGuardHome/AdGuardHome` 等路径 |
| YAML 配置 | 新版常用 `/etc/adguardhome/adguardhome.yaml`；部分旧版使用 `/etc/adguardhome.yaml` |
| 工作目录 | `/var/lib/adguardhome`，或自定义路径 |
| 服务脚本 | `/etc/init.d/adguardhome` |

新版 UCI 配置使用 `config_file`、`work_dir`，旧版可能使用 `config`、`workdir`。进程参数中的 `-c`／`--config`、`-w`／`--work-dir` 也能帮助确认实际路径。**不要把新版的配置文件随意移到 `/etc` 根目录**：服务可能要求独立配置目录，并使用 `adguardhome` 账户及 jail 运行。

按实际情况设置以下变量，并在后续步骤中保持使用同一个 SSH 会话：

```sh
agh_bin=/usr/bin/AdGuardHome
agh_config=/etc/adguardhome/adguardhome.yaml
agh_work=/var/lib/adguardhome
agh_service=/etc/init.d/adguardhome
```

路径不明确、有多个实例运行或空间不足时，先不要继续。持久化存储需要容纳旧程序备份和待替换的新程序；`/tmp` 需要容纳压缩包和解压后的程序。必要时将备份另存到电脑。历史查询日志和统计仍保留在工作目录，如需完整数据备份，请另外备份该目录。

## 3. 下载与校验

下面以 **ARM64** 为例，请按架构表修改 `agh_arch`：

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

只有下载成功、校验结果显示 `OK` 后才继续。这里先筛选对应包的一条校验记录，避免依赖 BusyBox 不一定支持的 `--ignore-missing`。

如果路由器不能访问 GitHub，可先在电脑下载对应程序包和 `SHA256SUMS`，再通过 SCP／SFTP 上传到临时目录。旧版 OpenWrt SSH 服务不支持 SFTP 时，可使用 `scp -O`。上传后仍按上面的方式在路由器校验，不要关闭 HTTPS 证书验证来绕过下载问题。

```sh
tar -xzf "$agh_archive"
agh_candidate="$agh_stage/AdGuardHome/AdGuardHome"
chmod 755 "$agh_candidate"
"$agh_candidate" --version
"$agh_candidate" --check-config -c "$agh_config" -w "$agh_work"
```

版本应为 `v0.107.79.2`，配置检查也必须成功。出现 `Exec format error`、`Illegal instruction`、找不到可执行文件解释器或检查失败时，**不要替换正在运行的程序**，先确认架构和固件兼容性。

## 4. 备份、停止服务并替换

创建权限受限的持久化备份：

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

任一备份命令失败，都应先处理再继续。先把新程序暂存到**目标文件所在目录**，使最终重命名发生在同一文件系统；先复制原文件，以保留属主、属组和权限：

```sh
cp -p "$agh_bin" "$agh_bin.ech-new"
cat "$agh_candidate" > "$agh_bin.ech-new"
cmp "$agh_candidate" "$agh_bin.ech-new"
```

复制和比较均成功后，停止服务：

```sh
"$agh_service" stop
sleep 2
pidof AdGuardHome
```

`pidof` 应不再返回 PID。如果旧进程仍然存在，不要替换，先检查服务或多实例情况。AdGuard Home 停止时可能保存配置，因此要在**停止后**更新配置备份：

```sh
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
mv "$agh_bin.ech-new" "$agh_bin"
"$agh_bin" --version
"$agh_service" start
```

服务停止后若有命令失败，请按下文回滚。不要删除备份或原工作目录，也不要直接向正在运行的可执行文件写入，否则可能报 `Text file busy`。

## 5. 验证 DNS 和 ECH

```sh
pidof AdGuardHome
"$agh_bin" --version
logread -e AdGuardHome | tail -n 30
nslookup example.org 127.0.0.1
```

确认 Web 页面版本正确，服务正常启动。上面的 `nslookup` 检查路由器本机 53 端口的解析路径，可能经过 dnsmasq。如需从装有 `dig` 的电脑检查其他端口，使用实际地址和端口，例如 `dig @192.168.1.1 -p 5335 example.org`。**5335 只是示例，不要求改成这个端口**；已有安装可能使用 53、54 或其他端口。

升级验证成功后，再停止服务，将 [ECH 配置示例](../ech-config.example.yaml)合入现有 `dns` 段并启动。保留本地 DNS／DHCP 设置、引导解析器和所需的备用策略。支持 ECH 的程序不会自动为所有上游启用 ECH。

在未命中缓存的查询中，**Response details → DNS server** 后出现 `(ECH)`，才表示实际选择的上游接受了 ECH。缓存和本地响应不会显示标记。测试时可查询尚未缓存的域名，或先在 Web 页面清空 DNS 缓存。

## 6. 回滚

继续使用此前确认的路径变量，以及输出的 `agh_backup` 目录。重新连接 SSH 后，应先重新设置这些变量，再执行：

```sh
"$agh_service" stop
cp -p "$agh_backup/AdGuardHome" "$agh_bin.rollback"
mv "$agh_bin.rollback" "$agh_bin"
cp -p "$agh_backup/adguardhome.yaml" "$agh_config"
"$agh_service" start
"$agh_bin" --version
nslookup example.org 127.0.0.1
```

需要同时恢复程序和 YAML，因为旧版官方程序可能不理解此修改版的设置。若另行修改过服务脚本和 UCI，也可使用相应备份；本教程的替换流程不会修改它们。

## 后续升级

每次升级都重复架构确认、校验、备份和运行检查。官方页面自动更新、包管理器重装／升级，以及固件升级，都可能将可执行文件覆盖为官方版本，或改变服务配置。之后应再次确认版本与 ECH 行为，不要假定自定义程序能跨 OpenWrt 升级保留。保留持久化备份，并将重要配置另存一份到电脑。

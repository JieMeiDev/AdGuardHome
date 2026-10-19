# OpenWrt installation and upgrade

[![English](https://img.shields.io/badge/Language-English-blue)](OPENWRT.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](OPENWRT.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](OPENWRT.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-CN.md)

> [!IMPORTANT]
> To use this project's upstream ECH feature, install the **AdGuard Home core executable** provided in this repository's [Releases](https://github.com/JieMeiDev/AdGuardHome/releases). Official v0.107.79 does not include this extension; changing YAML or replacing the web interface alone is insufficient. After installation, configure an ECH-capable upstream as documented.

[README](../README.md)

These instructions replace the executable in an **existing, working OpenWrt AdGuard Home installation**. They preserve the service integration, account, configuration, working directory, DNS ports, and DHCP setup. Run router commands as root over SSH. No router reboot is required.

For a new installation, first install and configure the [official OpenWrt package](https://openwrt.org/docs/guide-user/services/dns/adguard-home). OpenWrt 25.12 and later use `apk`; older supported releases use `opkg`. Verify normal DNS resolution before following this upgrade guide. This project's archives are standalone executables, not `.apk` or `.ipk` packages.

## 1. Identify the architecture

On the router:

```sh
uname -m
ubus call system board
if command -v apk >/dev/null 2>&1; then
    apk --print-arch
else
    opkg print-architecture
fi
```

Use the CPU architecture **and the installed userspace ABI**, not just the model name or whether the CPU is advertised as 64-bit. A 64-bit CPU can run 32-bit firmware. For MIPS, `uname -m` alone may not identify endianness. Package architecture names such as `mipsel_24kc` identify little-endian MIPS; `mips_24kc` identifies big-endian MIPS. If uncertain, inspect `readelf -h /bin/busybox` when available or consult the firmware's target documentation before choosing.

The Linux archive names follow [official AdGuard Home v0.107.79](https://github.com/AdguardTeam/AdGuardHome/releases/tag/v0.107.79):

| Router / firmware architecture | Archive suffix (`agh_arch`) | Go target |
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

The full filename is `AdGuardHome_linux_<suffix>.tar.gz`. Do not use a macOS `darwin` package on OpenWrt. CPU instruction set, floating-point support, userspace ABI, and kernel compatibility still matter: **run the candidate's `--version` on the router before stopping the current service**. The additional architectures are cross-compiled and inspected, not certified by testing on every router model.

## 2. Find the actual installation paths

```sh
cat /etc/config/adguardhome
cat /etc/init.d/adguardhome
ps w | grep '[A]dGuardHome'
df -h /tmp /usr/bin /root
```

Common paths are:

| Item | Common locations |
| --- | --- |
| Executable | `/usr/bin/AdGuardHome`, or a custom path such as `/opt/AdGuardHome/AdGuardHome` |
| YAML | `/etc/adguardhome/adguardhome.yaml` on newer packages; `/etc/adguardhome.yaml` on some older installations |
| Working directory | `/var/lib/adguardhome`, or the configured custom path |
| Service | `/etc/init.d/adguardhome` |

Newer packages use UCI `config_file` and `work_dir`; older packages may use `config` and `workdir`. Command-line `-c`/`--config` and `-w`/`--work-dir` also identify the active paths. **Do not move an old configuration into `/etc` on a newer package**: its service may require a dedicated configuration directory and run as `adguardhome` inside a jail.

Set these variables to your confirmed paths, and keep using the same SSH shell:

```sh
agh_bin=/usr/bin/AdGuardHome
agh_config=/etc/adguardhome/adguardhome.yaml
agh_work=/var/lib/adguardhome
agh_service=/etc/init.d/adguardhome
```

Stop here if the paths are ambiguous, multiple instances are running, or space is insufficient. Persistent storage must hold the backup and staged replacement; `/tmp` must hold the archive and extracted executable. Copy the backup to another machine if needed. Existing query logs and statistics remain in the working directory; back up that directory separately if you need a full data backup.

## 3. Download and verify

Example for **ARM64**; change `agh_arch` using the table above:

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

Continue only when the download commands succeed and the checksum reports `OK`. These commands select one checksum entry because BusyBox's `sha256sum` does not always support `--ignore-missing`.

If GitHub is unreachable from the router, download the archive and `SHA256SUMS` on your computer, then upload them using SCP/SFTP to the staging directory. If an older OpenWrt SSH server lacks SFTP support, use `scp -O`. Verify the checksum on the router as above. Keep HTTPS certificate verification enabled.

```sh
tar -xzf "$agh_archive"
agh_candidate="$agh_stage/AdGuardHome/AdGuardHome"
chmod 755 "$agh_candidate"
"$agh_candidate" --version
"$agh_candidate" --check-config -c "$agh_config" -w "$agh_work"
```

The version must be `v0.107.79.2`, and the configuration check must succeed. `Exec format error`, `Illegal instruction`, a missing executable interpreter, or a failed check means **do not replace the running program**. Recheck the architecture and firmware compatibility.

## 4. Back up, stop, and replace

Create a persistent backup with restrictive permissions:

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

If any backup command fails, resolve it before continuing. Stage the executable **beside the destination** so the final rename stays on the same filesystem. Copy the existing file first to retain its owner, group, and permissions:

```sh
cp -p "$agh_bin" "$agh_bin.ech-new"
cat "$agh_candidate" > "$agh_bin.ech-new"
cmp "$agh_candidate" "$agh_bin.ech-new"
```

Continue only if copying and comparison succeed. Then stop the service:

```sh
"$agh_service" stop
sleep 2
pidof AdGuardHome
```

`pidof` should return no PID. If the old process is still present, do not replace it; investigate the service or multiple instances. Refresh the configuration backup **after shutdown**, because AdGuard Home can save settings while stopping:

```sh
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
mv "$agh_bin.ech-new" "$agh_bin"
"$agh_bin" --version
"$agh_service" start
```

If a command fails after the service is stopped, use the rollback steps below. Do not delete the backup or existing working directory. Avoid copying over the currently running executable directly: it can fail with `Text file busy`.

## 5. Verify DNS and ECH

```sh
pidof AdGuardHome
"$agh_bin" --version
logread -e AdGuardHome | tail -n 30
nslookup example.org 127.0.0.1
```

Confirm that the web interface shows the new version and the service starts normally. `nslookup` above checks the router's local port 53 path, which may include dnsmasq. To test an AdGuard Home listener on another port from a computer with `dig`, use its actual address and port, for example `dig @192.168.1.1 -p 5335 example.org`. **5335 is an example, not a required setting**; some installations use 53, 54, or another port.

Only after the upgrade works, stop the service before editing YAML, merge the [ECH configuration example](../ech-config.example.yaml) into its existing `dns` section, and start it again. Preserve local DNS/DHCP settings, bootstrap resolvers, and required fallback policies. An ECH-capable executable does not enable ECH for all upstreams automatically.

In an uncached query's **Response details → DNS server**, `(ECH)` confirms actual acceptance by the selected upstream. Cached and local responses do not display this marker. Test a domain that is not already cached, or clear the DNS cache in the web interface before testing.

## 6. Roll back

Use the same confirmed path variables and the printed `agh_backup` directory. After reconnecting SSH, set them again before running these commands:

```sh
"$agh_service" stop
cp -p "$agh_backup/AdGuardHome" "$agh_bin.rollback"
mv "$agh_bin.rollback" "$agh_bin"
cp -p "$agh_backup/adguardhome.yaml" "$agh_config"
"$agh_service" start
"$agh_bin" --version
nslookup example.org 127.0.0.1
```

Restore the YAML as well as the executable: an older official binary may not understand this fork's settings. The service and UCI backups are available if you also changed them; this upgrade procedure does not alter them.

## Future updates

Repeat the architecture, checksum, backup, and preflight checks for each release. Official in-app updates, package-manager reinstalls/upgrades, and firmware upgrades may replace this executable with the official one or change service integration. Verify the version and ECH behavior afterward; do not assume a custom executable survives an OpenWrt upgrade. Keep a persistent backup and an external copy of important configuration.

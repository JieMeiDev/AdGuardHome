# OpenWrt: Installation und Aktualisierung

[![English](https://img.shields.io/badge/Language-English-lightgrey)](OPENWRT.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-blue)](OPENWRT.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](OPENWRT.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-CN.md)

> [!IMPORTANT]
> Für die Upstream-ECH-Funktion dieses Projekts muss die **AdGuard-Home-Programmdatei (Core)** aus den [Releases dieses Repositorys](https://github.com/JieMeiDev/AdGuardHome/releases) installiert werden. Die offizielle Version v0.107.79 enthält diese Erweiterung nicht; YAML-Änderungen oder der Austausch der Weboberfläche allein reichen nicht aus. Anschließend einen ECH-fähigen Upstream gemäß der Anleitung konfigurieren.

[README](../README.de.md)

Diese Anleitung ersetzt die ausführbare Datei einer **bereits funktionierenden AdGuard-Home-Installation unter OpenWrt**. Dienstintegration, Benutzerkonto, Konfiguration, Arbeitsverzeichnis, DNS-Ports und DHCP-Einstellungen bleiben erhalten. Die Router-Befehle werden als root über SSH ausgeführt. Ein Neustart des Routers ist nicht erforderlich.

Für eine Neuinstallation zunächst das [offizielle OpenWrt-Paket](https://openwrt.org/docs/guide-user/services/dns/adguard-home) installieren und konfigurieren. OpenWrt ab 25.12 verwendet `apk`, ältere unterstützte Versionen verwenden `opkg`. Vor dieser Aktualisierung die normale DNS-Auflösung prüfen. Die Archive dieses Projekts enthalten eigenständige Programme und sind keine `.apk`- oder `.ipk`-Pakete.

## 1. Architektur ermitteln

Auf dem Router ausführen:

```sh
uname -m
ubus call system board
if command -v apk >/dev/null 2>&1; then
    apk --print-arch
else
    opkg print-architecture
fi
```

Entscheidend sind sowohl die CPU-Architektur **als auch die ABI des installierten Userspace**. Modellname oder eine beworbene 64-Bit-CPU reichen nicht aus: Eine solche CPU kann mit 32-Bit-Firmware betrieben werden. Bei MIPS lässt sich die Byte-Reihenfolge nicht immer allein mit `uname -m` erkennen. Paketarchitekturen wie `mipsel_24kc` bezeichnen Little-Endian-MIPS; `mips_24kc` bezeichnet Big-Endian-MIPS. Im Zweifel, sofern verfügbar, `readelf -h /bin/busybox` prüfen oder die Dokumentation des Firmware-Targets heranziehen.

Die Linux-Paketnamen folgen der [offiziellen Version v0.107.79](https://github.com/AdguardTeam/AdGuardHome/releases/tag/v0.107.79):

| Router-/Firmware-Architektur | Archivsuffix (`agh_arch`) | Go-Ziel |
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

Der vollständige Dateiname lautet `AdGuardHome_linux_<suffix>.tar.gz`. Unter OpenWrt kein macOS-Paket mit `darwin` verwenden. Befehlssatz, Gleitkommaunterstützung, Userspace-ABI und Kernel müssen kompatibel sein. Deshalb **vor dem Stoppen des bisherigen Dienstes die neue Datei auf dem Router mit `--version` ausführen**. Die zusätzlichen Architekturen wurden kreuzkompiliert und untersucht, aber nicht auf jedem Routermodell getestet.

## 2. Tatsächliche Installationspfade feststellen

```sh
cat /etc/config/adguardhome
cat /etc/init.d/adguardhome
ps w | grep '[A]dGuardHome'
df -h /tmp /usr/bin /root
```

Häufige Pfade:

| Bestandteil | Häufige Pfade |
| --- | --- |
| Programmdatei | `/usr/bin/AdGuardHome` oder ein eigener Pfad wie `/opt/AdGuardHome/AdGuardHome` |
| YAML | Bei neueren Paketen `/etc/adguardhome/adguardhome.yaml`, bei manchen älteren Installationen `/etc/adguardhome.yaml` |
| Arbeitsverzeichnis | `/var/lib/adguardhome` oder ein konfigurierter eigener Pfad |
| Dienstskript | `/etc/init.d/adguardhome` |

Neuere Pakete verwenden in UCI `config_file` und `work_dir`, ältere gegebenenfalls `config` und `workdir`. Die Prozessargumente `-c`/`--config` und `-w`/`--work-dir` geben ebenfalls die aktiven Pfade an. **Bei einem neueren Paket die Konfiguration nicht einfach direkt nach `/etc` verschieben**: Der Dienst kann ein eigenes Konfigurationsverzeichnis voraussetzen und als Benutzer `adguardhome` in einem Jail laufen.

Die Variablen auf die bestätigten Pfade setzen und für die nächsten Schritte dieselbe SSH-Shell verwenden:

```sh
agh_bin=/usr/bin/AdGuardHome
agh_config=/etc/adguardhome/adguardhome.yaml
agh_work=/var/lib/adguardhome
agh_service=/etc/init.d/adguardhome
```

Bei unklaren Pfaden, mehreren laufenden Instanzen oder unzureichendem Speicherplatz nicht fortfahren. Der dauerhafte Speicher muss Sicherung und vorbereitete neue Programmdatei aufnehmen können; `/tmp` benötigt Platz für Archiv und entpacktes Programm. Die Sicherung bei Bedarf zusätzlich auf einen anderen Rechner kopieren. Abfrageprotokolle und Statistiken bleiben im Arbeitsverzeichnis. Für eine vollständige Datensicherung dieses Verzeichnis separat sichern.

## 3. Herunterladen und prüfen

Beispiel für **arm64**. `agh_arch` anhand der Tabelle anpassen:

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

Nur fortfahren, wenn die Downloads erfolgreich waren und die Prüfsumme `OK` meldet. Es wird gezielt ein Prüfsummeneintrag ausgewählt, da BusyBox bei `sha256sum` nicht immer `--ignore-missing` unterstützt.

Ist GitHub vom Router nicht erreichbar, Archiv und `SHA256SUMS` zunächst auf dem Computer herunterladen und per SCP/SFTP in das temporäre Verzeichnis übertragen. Unterstützt ein älterer OpenWrt-SSH-Server kein SFTP, kann `scp -O` verwendet werden. Anschließend die Prüfsumme wie oben auf dem Router prüfen. Die HTTPS-Zertifikatsprüfung aktiviert lassen.

```sh
tar -xzf "$agh_archive"
agh_candidate="$agh_stage/AdGuardHome/AdGuardHome"
chmod 755 "$agh_candidate"
"$agh_candidate" --version
"$agh_candidate" --check-config -c "$agh_config" -w "$agh_work"
```

Die Version muss `v0.107.79.2` sein, und die Konfigurationsprüfung muss erfolgreich sein. Bei `Exec format error`, `Illegal instruction`, fehlendem Programminterpreter oder fehlgeschlagener Prüfung **das laufende Programm nicht ersetzen**. Zuerst Architektur und Firmware-Kompatibilität prüfen.

## 4. Sichern, Dienst stoppen und Programm ersetzen

Eine dauerhafte Sicherung mit eingeschränkten Zugriffsrechten erstellen:

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

Schlägt ein Sicherungsbefehl fehl, zuerst den Fehler beheben. Die neue Datei **neben der Zieldatei** vorbereiten, damit das abschließende Umbenennen auf demselben Dateisystem erfolgt. Zunächst die bisherige Datei kopieren, um Eigentümer, Gruppe und Rechte beizubehalten:

```sh
cp -p "$agh_bin" "$agh_bin.ech-new"
cat "$agh_candidate" > "$agh_bin.ech-new"
cmp "$agh_candidate" "$agh_bin.ech-new"
```

Nur fortfahren, wenn Kopieren und Vergleich erfolgreich waren. Dann den Dienst stoppen:

```sh
"$agh_service" stop
sleep 2
pidof AdGuardHome
```

`pidof` darf keine PID mehr ausgeben. Läuft der alte Prozess weiter, die Datei nicht ersetzen; zuerst Dienst oder Mehrfachinstanzen untersuchen. Da AdGuard Home beim Beenden Einstellungen speichern kann, die Konfigurationssicherung **nach dem Stoppen** aktualisieren:

```sh
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
mv "$agh_bin.ech-new" "$agh_bin"
"$agh_bin" --version
"$agh_service" start
```

Schlägt nach dem Stoppen ein Befehl fehl, die Wiederherstellung unten verwenden. Sicherung und Arbeitsverzeichnis nicht löschen. Eine laufende Programmdatei nicht direkt überschreiben; dies kann mit `Text file busy` fehlschlagen.

## 5. DNS und ECH prüfen

```sh
pidof AdGuardHome
"$agh_bin" --version
logread -e AdGuardHome | tail -n 30
nslookup example.org 127.0.0.1
```

Die Weboberfläche muss die neue Version anzeigen und der Dienst normal starten. Der gezeigte `nslookup` prüft den lokalen Port 53 des Routers; dieser Pfad kann dnsmasq einschließen. Einen anderen AdGuard-Home-Port von einem Computer mit `dig` über die tatsächliche Adresse und Portnummer prüfen, zum Beispiel `dig @192.168.1.1 -p 5335 example.org`. **5335 ist nur ein Beispiel**, keine vorgeschriebene Einstellung. Bestehende Installationen können 53, 54 oder andere Ports verwenden.

Erst nach erfolgreicher Aktualisierung den Dienst vor der YAML-Bearbeitung stoppen, das [ECH-Konfigurationsbeispiel](../ech-config.example.yaml) in den vorhandenen Abschnitt `dns` integrieren und den Dienst wieder starten. Lokale DNS/DHCP-Einstellungen, Bootstrap-Resolver und benötigte Fallback-Richtlinien beibehalten. Eine ECH-fähige Programmdatei aktiviert ECH nicht automatisch für alle Upstreams.

Bei einer nicht zwischengespeicherten Abfrage bestätigt `(ECH)` unter **Response details → DNS server**, dass der ausgewählte Upstream ECH angenommen hat. Antworten aus dem Cache und lokale Antworten erhalten diese Kennzeichnung nicht. Eine noch nicht zwischengespeicherte Domain abfragen oder vorher den DNS-Cache in der Weboberfläche leeren.

## 6. Wiederherstellen

Dieselben bestätigten Pfadvariablen und das ausgegebene Verzeichnis `agh_backup` verwenden. Nach einer neuen SSH-Verbindung die Variablen erneut setzen, bevor die Befehle ausgeführt werden:

```sh
"$agh_service" stop
cp -p "$agh_backup/AdGuardHome" "$agh_bin.rollback"
mv "$agh_bin.rollback" "$agh_bin"
cp -p "$agh_backup/adguardhome.yaml" "$agh_config"
"$agh_service" start
"$agh_bin" --version
nslookup example.org 127.0.0.1
```

Sowohl Programm als auch YAML wiederherstellen: Eine ältere offizielle Programmversion versteht möglicherweise die Einstellungen dieser Anpassung nicht. Sicherungen von Dienstskript und UCI stehen bereit, falls diese ebenfalls geändert wurden; die hier beschriebene Aktualisierung verändert sie nicht.

## Spätere Aktualisierungen

Architektur, Prüfsummen, Sicherung und Vorabprüfung bei jeder Veröffentlichung erneut prüfen. Offizielle Updates aus der Anwendung, Neuinstallationen oder Aktualisierungen des Pakets sowie Firmware-Upgrades können die eigene Programmdatei durch die offizielle ersetzen oder die Dienstintegration ändern. Danach Version und ECH-Verhalten erneut überprüfen. Nicht davon ausgehen, dass die angepasste Datei ein OpenWrt-Upgrade übersteht. Eine dauerhafte Sicherung und eine externe Kopie wichtiger Einstellungen aufbewahren.

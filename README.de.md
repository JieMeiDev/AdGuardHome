# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-lightgrey)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-blue)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-lightgrey)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](README.zh-CN.md)

Eine persönliche Anpassung des offiziellen **AdGuard Home v0.107.79**, die ausgehende DNS-over-HTTPS-Verbindungen über HTTP/2 und HTTP/3 um **ECH (Encrypted Client Hello)** erweitert.

Aktuelle Version: **[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**. Der aktuelle GitHub-Tag, die ausführbare Datei und die Weboberfläche verwenden dieselbe Versionsangabe: `v0.107.79.2`.

Dies ist eine unabhängig gepflegte, mit KI-Unterstützung entwickelte Anpassung und keine offizielle AdGuard-Veröffentlichung. Der ursprüngliche Quellcode sowie Urheberrechts- und Lizenzhinweise bleiben erhalten.

## OpenWrt: Installation und Aktualisierung

Die [ausführliche OpenWrt-Anleitung (Englisch)](docs/OPENWRT.md) beschreibt Architekturauswahl, Prüfsummen, Installationspfade, Sicherung, Austausch der Programmdatei, DNS/ECH-Prüfung und Wiederherstellung.

Die Linux-Pakete folgen den zwölf offiziellen Architekturvarianten: `amd64`, `386`, `arm64`, `armv5`, `armv6`, `armv7`, `mips_softfloat`, `mipsle_softfloat`, `mips64_softfloat`, `mips64le_softfloat`, `ppc64le`, `riscv64`. Firmware-ABI und Byte-Reihenfolge müssen passen. Vor dem Stoppen des bisherigen Dienstes die neue Datei mit `--version` prüfen. Die zusätzlichen Architekturen wurden kreuzkompiliert und untersucht, aber nicht alle auf physischen Routern getestet.

## Änderungen

- **ECH für Upstream-Verbindungen:** ECH explizit für ausgewählte Hostnamen von `https://`- und `h3://`-Upstreams aktivieren.
- **Unabhängige Ermittlung:** ECH-Konfigurationen aus HTTPS-DNS-Einträgen über separat konfigurierte Resolver abrufen. Die Konfigurationen werden entsprechend der DNS-TTL zwischengespeichert und nach Ablauf bei Bedarf erneuert.
- **Prüfung des Handshakes:** Die normale Zertifikatsprüfung bleibt erhalten. DNS-Anfragen werden erst nach erfolgreicher ECH-Annahme gesendet. Bei einer authentifizierten Ersatzkonfiguration wird der Handshake höchstens einmal wiederholt.
- **Fehlerbehandlung:** Schlägt ein Upstream mit vorgeschriebenem ECH fehl, wird ein Upstream-Fehler an den bestehenden Fallback-Mechanismus übergeben. ECH wird nicht stillschweigend für einen erneuten Versuch deaktiviert. Jeder Fallback behält seine eigene Richtlinie.
- **Wiederverwendung von Verbindungen:** HTTP/2- und HTTP/3-Verbindungen werden weiterhin wiederverwendet. TLS-Sitzungswiederaufnahme und H3-0-RTT sind für ECH-Verbindungen deaktiviert.
- **Abfragestatus:** Die bestehende DNS-Server-Adresse erhält den Zusatz `(ECH)`, wenn ECH tatsächlich verwendet wurde.

Der Funktionsumfang umfasst ausgehende DoH/H3-Clientverbindungen von AdGuard Home. Serverseitiges ECH für eingehende Verbindungen, DoT/DoQ mit ECH und vollständige SVCB-Weiterleitung zu alternativen Zielen sind nicht enthalten. ECH verbirgt den eigentlichen Dienst-Hostnamen im Handshake; Ziel-IP-Adressen und Verkehrsmetadaten bleiben sichtbar.

## ECH aktivieren

Das folgende Beispiel verwendet **Crypto.sx** und **RethinkDNS Sky**. Beide wurden in unserem Testnetz mit H3 und erfolgreich angenommenem ECH geprüft. Diesen Ausschnitt in den bestehenden Abschnitt `dns` einfügen:

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

Dies ist nur ein Konfigurationsausschnitt. Bestehende Einstellungen wie `upstream_dns`, `bootstrap_dns` und `fallback_dns` beibehalten. Die Schlüssel in `upstream_ech` sind kleingeschriebene Hostnamen. Die Werte sind Listen unabhängiger Resolver, über die ECH-Konfigurationen abgerufen werden.

Der Resolver zur ECH-Ermittlung muss aus dem eigenen Netz erreichbar sein, HTTPS-Einträge mit ECH-Parametern zurückgeben und eine IP-Adresse im Endpunkt enthalten. DNS-Stamps mit eingebetteten IP-Adressen werden ebenfalls unterstützt. Der `sdns://`-Stamp wählt den ungefilterten DNSCrypt-Resolver von AdGuard unter `94.140.14.140:5443`, der unabhängig für die ECH-Ermittlung geprüft wurde. Crypto.sx und RethinkDNS bearbeiten die normalen Abfragen; der DNSCrypt-Resolver liefert ihre ECH-Konfigurationen. Nicht AdGuard Home selbst als Resolver eintragen, da dadurch eine zyklische Auflösungsabhängigkeit entstehen kann.

Nach der Änderung AdGuard Home neu starten. Diese Option wird derzeit über YAML konfiguriert; ein zusätzlicher Schalter in der Weboberfläche ist nicht vorhanden. Durch Entfernen der Richtlinie für einen Hostnamen wird das ursprüngliche Verhalten dieses Upstreams wiederhergestellt. Siehe das kommentierte [Konfigurationsbeispiel](ech-config.example.yaml).

## ECH überprüfen

Im Abfrageprotokoll **Response details → DNS server** öffnen. Eine Abfrage, die tatsächlich ECH verwendet hat, zeigt:

```text
https://doh.crypto.sx:443/dns-query (ECH)
```

Die Kennzeichnung stammt aus dem TLS-Status der Verbindung, über die die erfolgreiche HTTP-Antwort empfangen wurde. Sie funktioniert auch bei wiederverwendeten Verbindungen und gehört zum Haupt- oder Fallback-Upstream, der die endgültige Antwort geliefert hat. Antworten aus dem Cache, lokale Antworten und ältere Einträge ohne ECH-Metadaten werden nicht gekennzeichnet.

Protokollspeicherung und API bewahren den Status in einem separaten optionalen Feld auf. Die ursprüngliche Upstream-Adresse bleibt unverändert. Es gibt keine zusätzliche Detailzeile oder Einstellung.

## Versionen und offizielle Ausgangsversionen

| Persönliche Veröffentlichung | Angezeigte Programmversion | Änderungen |
| --- | --- | --- |
| [`v0.107.79.1`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.1) | `v0.107.79-ech.1` (historische Kennzeichnung) | Erste Unterstützung für Upstream-ECH |
| [`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2) | `v0.107.79.2` | ECH-Status je Abfrage und vereinheitlichte Versionsangabe |

Beide Veröffentlichungen basieren auf den folgenden offiziellen Quellen. Die persönlichen Versionsnummern bedeuten nicht, dass entsprechende offizielle Versionen veröffentlicht wurden:

| Komponente | Offizielle Version | Ausgangs-Commit |
| --- | --- | --- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | `v0.107.79` | `05ba17b282da1c4393d6a4ba4db0cf519194a362` |
| [dnsproxy](https://github.com/AdguardTeam/dnsproxy) | `v0.83.2` | `acf2b30e05202e171bbbaf4a3176ea07edb964b3` |

Das Repository enthält den vollständigen Quellcode beider Module. Modul-Importpfade und Abhängigkeitsversionen bleiben erhalten. Die Datei `go.work` im Stammverzeichnis wählt den mitgelieferten dnsproxy aus. [SOURCE-MANIFEST.json](SOURCE-MANIFEST.json) dokumentiert die SHA-256-Prüfsummen der geänderten Quelldateien.

## Aus dem Quellcode bauen

Builds wurden mit Go **1.27.1** und Node.js **26.3.0** geprüft. Wenn Go, Node.js/npm und `shasum` verfügbar sind, im Stammverzeichnis ausführen:

```sh
./build.sh
```

Die vollständige Linux-Matrix baut `sh scripts/build-linux.sh`; Argumente wie `arm64 armv7` beschränken die Ziele. Das Frontend wird einmal gebaut; Programmdateien und `SHA256SUMS-linux` landen in `artifacts/`.

Das Skript baut das Frontend und bettet es ein. Anschließend werden ausführbare Dateien für Linux amd64 und macOS amd64 sowie SHA-256-Prüfsummen in `artifacts/` abgelegt. Die Programmversion wird aus der Datei [VERSION](VERSION) im Stammverzeichnis gelesen. Mit `GO=/path/to/go` lässt sich die Go-Programmdatei festlegen.

## Validierung

ECH-Tests mit lokalen TLS/H2/H3-Endpunkten, zugehörige Race-Tests und ECH-Abfragen an reale H2/H3-Upstreams wurden erfolgreich ausgeführt. Bei OpenWrt-Installationen wurden außerdem DNS-Auflösung und Statusanzeige in der Weboberfläche geprüft.

Die Tests decken ECH-Annahme, Konfigurationswechsel, Ablehnung, fehlende oder fehlerhafte Einträge, Zertifikatsfehler, Wechsel des ECH-Resolvers, Caching, Nebenläufigkeit und Fallback ab. Version `.2` prüft zusätzlich die Wiederverwendung von Verbindungen, die Zuordnung der endgültigen Antwort, Protokollpersistenz, Kompatibilität mit älteren Einträgen und die Frontend-Anzeige.

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

Alle fünf Frontend-Anzeigetests, die TypeScript-Prüfung und die ESLint-Prüfung der geänderten Dateien wurden bestanden. Nach `npm ci` in `AdGuardHome/client` können diese Befehle im Stammverzeichnis ausgeführt werden:

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

Die gesamte ursprüngliche Testsuite wurde noch nicht vollständig bestanden: Einige bestehende Tests benötigen öffentliche Dienste wie `8.8.8.8:53` und `1.1.1.1:53`, die im Testnetz in ein Timeout laufen. Die vollständige Upstream-CI einschließlich `make go-check` ist weiterhin ungeprüft.

## Beitrag zum offiziellen Projekt

Eine Designdiskussion wurde unter [AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573) eingereicht. Vorgeschlagen ist, zunächst die Transportunterstützung in dnsproxy zu integrieren und anschließend die AdGuard-Home-Konfiguration und Abhängigkeiten anzupassen. Eine Rückmeldung der Maintainer steht aus. Es wurde kein PR eingereicht, und die Implementierung wurde nicht offiziell angenommen.

## Lizenzen

AdGuard Home und diese kombinierte Distribution behalten die GPL-3.0-Lizenz bei; siehe [LICENSE](LICENSE) und die [ursprüngliche Lizenz](AdGuardHome/LICENSE.txt). dnsproxy behält seine [Apache-2.0-Lizenz](dnsproxy/LICENSE). Ursprüngliche Urheberrechtsvermerke und Lizenzhinweise Dritter bleiben erhalten.

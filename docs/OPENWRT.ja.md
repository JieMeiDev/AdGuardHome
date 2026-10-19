# OpenWrt のインストールと更新

[![English](https://img.shields.io/badge/Language-English-lightgrey)](OPENWRT.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](OPENWRT.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-blue)](OPENWRT.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](OPENWRT.zh-CN.md)

[README](../README.ja.md)

この手順は、**OpenWrt 上で正常に動作している AdGuard Home** の実行ファイルを置き換えるためのものです。既存のサービス連携、実行アカウント、設定、作業ディレクトリ、DNS ポート、DHCP 設定は維持します。ルーターのコマンドは SSH 経由で root として実行します。ルーターの再起動は不要です。

新規インストールでは、まず [OpenWrt 公式パッケージ](https://openwrt.org/docs/guide-user/services/dns/adguard-home)をインストールして設定してください。OpenWrt 25.12 以降は `apk`、それ以前のサポート対象バージョンは `opkg` を使用します。通常の DNS 名前解決が正常に動くことを確認してから、この更新手順に進みます。本プロジェクトのアーカイブは単体の実行ファイルであり、`.apk` や `.ipk` パッケージではありません。

## 1. アーキテクチャを確認する

ルーターで次を実行します。

```sh
uname -m
ubus call system board
if command -v apk >/dev/null 2>&1; then
    apk --print-arch
else
    opkg print-architecture
fi
```

機種名や「64 ビット CPU」という表記だけでなく、**CPU アーキテクチャとインストール済みユーザー空間の ABI** を確認します。64 ビット CPU で 32 ビットのファームウェアを実行している場合もあります。MIPS は `uname -m` だけではエンディアンを判別できないことがあります。パッケージアーキテクチャの `mipsel_24kc` はリトルエンディアン、`mips_24kc` はビッグエンディアンです。不明な場合は、利用可能なら `readelf -h /bin/busybox` を確認するか、ファームウェアのターゲットに関する資料を参照してください。

Linux アーカイブ名は[公式 v0.107.79](https://github.com/AdguardTeam/AdGuardHome/releases/tag/v0.107.79)に合わせています。

| ルーター／ファームウェア | アーカイブ接尾辞（`agh_arch`） | Go ターゲット |
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

完全なファイル名は `AdGuardHome_linux_<suffix>.tar.gz` です。OpenWrt で macOS 用の `darwin` パッケージを使用しないでください。CPU の命令セット、浮動小数点対応、ユーザー空間 ABI、カーネルの互換性も必要です。**既存サービスを停止する前に、候補の実行ファイルをルーター上で `--version` を付けて実行してください。** 追加アーキテクチャはクロスコンパイルと生成物の検査を行っていますが、すべてのルーター機種で実機検証したわけではありません。

## 2. 実際のインストール先を確認する

```sh
cat /etc/config/adguardhome
cat /etc/init.d/adguardhome
ps w | grep '[A]dGuardHome'
df -h /tmp /usr/bin /root
```

よく使われるパスは次のとおりです。

| 項目 | 一般的な場所 |
| --- | --- |
| 実行ファイル | `/usr/bin/AdGuardHome`、または `/opt/AdGuardHome/AdGuardHome` などの独自パス |
| YAML | 新しいパッケージは `/etc/adguardhome/adguardhome.yaml`、一部の古い環境は `/etc/adguardhome.yaml` |
| 作業ディレクトリ | `/var/lib/adguardhome`、または設定済みの独自パス |
| サービススクリプト | `/etc/init.d/adguardhome` |

新しいパッケージの UCI 項目は `config_file` と `work_dir`、古いものでは `config` と `workdir` が使われる場合があります。プロセスの引数 `-c`/`--config` と `-w`/`--work-dir` でも実際のパスを確認できます。**新しいパッケージで設定ファイルを `/etc` 直下へ移動しないでください。** 専用ディレクトリが必要で、`adguardhome` アカウントと jail で実行される場合があります。

確認したパスに合わせて次の変数を設定し、以降も同じ SSH シェルを使用します。

```sh
agh_bin=/usr/bin/AdGuardHome
agh_config=/etc/adguardhome/adguardhome.yaml
agh_work=/var/lib/adguardhome
agh_service=/etc/init.d/adguardhome
```

パスが不明、複数インスタンスが稼働中、または空き容量不足の場合は先に進まないでください。永続ストレージにはバックアップと置換用実行ファイル、`/tmp` にはアーカイブと展開後の実行ファイルを保存する空きが必要です。必要ならバックアップを別のコンピューターにもコピーします。クエリログと統計は作業ディレクトリに残ります。完全なデータバックアップが必要なら、そのディレクトリも別途保存してください。

## 3. ダウンロードと検証

以下は **arm64** の例です。表を参照して `agh_arch` を変更してください。

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

ダウンロードが成功し、チェックサムが `OK` と表示された場合だけ続行します。BusyBox の `sha256sum` は `--ignore-missing` に対応しない場合があるため、対象パッケージのチェックサムを 1 行だけ抽出しています。

ルーターから GitHub に接続できない場合は、コンピューターでアーカイブと `SHA256SUMS` をダウンロードし、SCP/SFTP で一時ディレクトリへ転送します。古い OpenWrt の SSH サーバーが SFTP に対応していない場合は `scp -O` を使用できます。転送後もルーター上で同じチェックサム検証を行ってください。HTTPS 証明書検証は有効のままにします。

```sh
tar -xzf "$agh_archive"
agh_candidate="$agh_stage/AdGuardHome/AdGuardHome"
chmod 755 "$agh_candidate"
"$agh_candidate" --version
"$agh_candidate" --check-config -c "$agh_config" -w "$agh_work"
```

バージョンが `v0.107.79.2` で、設定検証も成功する必要があります。`Exec format error`、`Illegal instruction`、実行ファイルのインタープリターが見つからない、または検証に失敗した場合は、**稼働中のプログラムを置き換えないでください。** アーキテクチャとファームウェアの互換性を再確認します。

## 4. バックアップ、停止、置換

アクセス権を制限した永続バックアップを作成します。

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

いずれかのバックアップコマンドが失敗したら、解決してから続行してください。最後の名前変更が同じファイルシステム内で行われるよう、新しい実行ファイルを**置換先と同じディレクトリ**に準備します。元のファイルを先にコピーし、所有者、グループ、権限を維持します。

```sh
cp -p "$agh_bin" "$agh_bin.ech-new"
cat "$agh_candidate" > "$agh_bin.ech-new"
cmp "$agh_candidate" "$agh_bin.ech-new"
```

コピーと比較が成功した場合だけ、サービスを停止します。

```sh
"$agh_service" stop
sleep 2
pidof AdGuardHome
```

`pidof` が PID を返さないことを確認します。旧プロセスが残っていたら置き換えず、サービスや複数インスタンスの状態を調べてください。AdGuard Home は停止時に設定を書き出すことがあるため、**停止後に**設定バックアップを更新します。

```sh
cp -p "$agh_config" "$agh_backup/adguardhome.yaml"
mv "$agh_bin.ech-new" "$agh_bin"
"$agh_bin" --version
"$agh_service" start
```

サービス停止後にコマンドが失敗した場合は、下のロールバック手順を使います。バックアップや既存の作業ディレクトリは削除しないでください。稼働中の実行ファイルへの直接上書きは `Text file busy` で失敗する場合があります。

## 5. DNS と ECH を確認する

```sh
pidof AdGuardHome
"$agh_bin" --version
logread -e AdGuardHome | tail -n 30
nslookup example.org 127.0.0.1
```

Web 画面に新しいバージョンが表示され、サービスが正常に起動していることを確認します。上の `nslookup` はルーターのローカルポート 53 を確認するもので、dnsmasq を経由する場合があります。別ポートの AdGuard Home を確認する場合は、`dig` のあるコンピューターから実際のアドレスとポートを指定します。例：`dig @192.168.1.1 -p 5335 example.org`。**5335 は例であり、必須設定ではありません。** 既存環境は 53、54、その他のポートを使用している場合があります。

更新が正常に動作してから、YAML 編集前にサービスを停止し、[ECH 設定例](../ech-config.example.yaml)を既存の `dns` セクションに統合して再起動します。ローカル DNS/DHCP 設定、ブートストラップリゾルバー、必要な fallback ポリシーを維持してください。ECH 対応の実行ファイルを入れるだけでは、すべての上流で ECH が自動的に有効になるわけではありません。

キャッシュされていないクエリの **Response details → DNS server** に `(ECH)` が付いていれば、選択された上流が実際に ECH を受け入れたことを確認できます。キャッシュ応答とローカル応答には表示されません。まだキャッシュされていないドメインを問い合わせるか、Web 画面で DNS キャッシュを消去してからテストしてください。

## 6. ロールバック

確認済みのパス変数と、先ほど出力された `agh_backup` ディレクトリを使用します。SSH に再接続した場合は、変数を再設定してから実行してください。

```sh
"$agh_service" stop
cp -p "$agh_backup/AdGuardHome" "$agh_bin.rollback"
mv "$agh_bin.rollback" "$agh_bin"
cp -p "$agh_backup/adguardhome.yaml" "$agh_config"
"$agh_service" start
"$agh_bin" --version
nslookup example.org 127.0.0.1
```

古い公式実行ファイルがこの改変版の設定を理解できない場合があるため、実行ファイルと YAML の両方を復元します。サービススクリプトや UCI も別途変更した場合は、そのバックアップも利用できます。この更新手順自体はそれらを変更しません。

## 今後の更新

各リリースでアーキテクチャ確認、チェックサム検証、バックアップ、事前動作確認を繰り返します。公式のアプリ内更新、パッケージの再インストールや更新、ファームウェア更新によって、この実行ファイルが公式版に置き換えられたり、サービス連携が変わったりする場合があります。更新後はバージョンと ECH の動作を再確認し、独自の実行ファイルが OpenWrt 更新後も保持されるとは考えないでください。永続バックアップと、重要な設定の外部コピーを保管してください。

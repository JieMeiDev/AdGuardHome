# AdGuardHome

[![English](https://img.shields.io/badge/Language-English-lightgrey)](README.md) [![Deutsch](https://img.shields.io/badge/Language-Deutsch-lightgrey)](README.de.md) [![日本語](https://img.shields.io/badge/Language-%E6%97%A5%E6%9C%AC%E8%AA%9E-blue)](README.ja.md) [![繁體中文](https://img.shields.io/badge/Language-%E7%B9%81%E9%AB%94%E4%B8%AD%E6%96%87-lightgrey)](README.zh-TW.md) [![简体中文](https://img.shields.io/badge/Language-%E7%AE%80%E4%BD%93%E4%B8%AD%E6%96%87-lightgrey)](README.zh-CN.md)

公式の **AdGuard Home v0.107.79** をベースに、HTTP/2・HTTP/3 経由の DoH 上流接続に **ECH（Encrypted Client Hello）** 対応を追加した個人改変版です。

最新リリース：**[`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2)**。最新の GitHub タグ、実行ファイル、Web 画面のバージョン表記は、いずれも `v0.107.79.2` です。

AI の支援を受けて開発・個人管理している改変版であり、AdGuard の公式リリースではありません。公式プロジェクトのソースコード、著作権表示、ライセンスは維持しています。

## 既存環境の更新

1. 機器のアーキテクチャに合うパッケージをダウンロードし、`SHA256SUMS` を確認して展開します。
2. 現在の実行ファイルと設定をバックアップして、AdGuard Home を停止します。
3. パッケージ内の `AdGuardHome/AdGuardHome` で実行ファイルを置き換えます。既存の設定、作業ディレクトリ、サービス設定は維持します。
4. サービスを起動し、表示バージョンと DNS 名前解決を確認します。

Linux パッケージの展開とバージョン確認の例：

```sh
tar -xzf AdGuardHome_linux_amd64.tar.gz
./AdGuardHome/AdGuardHome --version
# AdGuard Home, version v0.107.79.2
```

OpenWrt 向けのダウンロードは単体実行ファイルのアーカイブであり、`.ipk` ではありません。既存環境では現在のサービススクリプトを引き続き使用します。新規インストールについては、[AdGuard Home 公式の説明](AdGuardHome/README.md)を参照してください。

## 変更内容

- **上流接続の ECH：** 指定したホスト名の `https://`・`h3://` 上流に対して、ECH を明示的に有効にします。
- **独立した設定取得：** 別途指定したリゾルバーを通じて HTTPS DNS レコードの ECH 設定を取得します。DNS TTL に従ってキャッシュし、期限切れ後は必要に応じて更新します。
- **ハンドシェイク検証：** 通常の証明書検証を維持し、ECH が受け入れられてから DNS クエリを送信します。認証済みの新しい設定を受信した場合、ハンドシェイクを最大 1 回再試行します。
- **失敗時の処理：** ECH を必須とする上流で失敗した場合は上流エラーを返し、既存の fallback 機構に処理を委ねます。ECH を自動的に無効にして再試行することはありません。各 fallback は自身のポリシーを維持します。
- **接続の再利用：** HTTP/2・HTTP/3 接続の再利用を維持します。ECH 接続では TLS セッション再開と H3 の 0-RTT を無効にしています。
- **クエリの状態表示：** 実際に ECH を使用した場合、既存の DNS server アドレスの末尾に `(ECH)` を表示します。

対象は AdGuard Home から上流への DoH/H3 クライアント接続です。下流向けのサーバー側 ECH、DoT/DoQ の ECH、SVCB の代替接続先を使う完全なルーティングは含まれません。ECH はハンドシェイク内の実際のサービスホスト名を隠しますが、接続先 IP アドレスや通信メタデータは引き続き見えます。

## ECH を有効にする

既存設定の `dns` セクションに次の内容を追加し、`dns.example.net` を実際の上流ホスト名に置き換えます。

```yaml
dns:
  upstream_ech:
    dns.example.net:
      - https://1.1.1.1/dns-query
```

これは設定の一部です。既存の `upstream_dns`、`bootstrap_dns`、`fallback_dns` などは維持してください。`upstream_ech` のキーには小文字のホスト名を、値には ECH 設定を取得する独立したリゾルバーの一覧を指定します。

設定取得用のリゾルバーは、利用中のネットワークから到達でき、ECH パラメーターを含む HTTPS レコードを返し、エンドポイントに IP アドレスが含まれている必要があります。IP アドレスを内包する DNS stamp も使用できます。上記アドレスは書式の例にすぎません。名前解決の循環依存を避けるため、AdGuard Home 自身を設定取得先にしないでください。

設定変更後に AdGuard Home を再起動します。現在は YAML で設定する方式で、Web 画面に新しいスイッチは追加していません。ホスト名に対するポリシーを削除すると、その上流は従来の動作に戻ります。コメント付きの[設定例](ech-config.example.yaml)も参照してください。

## ECH の使用を確認する

クエリログの **Response details → DNS server** を開きます。実際に ECH を使用したクエリには、次のように表示されます。

```text
https://dns.example.net:443/dns-query (ECH)
```

この表示は、成功した HTTP 応答に使用された接続の TLS 状態に基づきます。接続を再利用した場合も機能し、最終的な回答を返したメイン上流または fallback の状態を示します。キャッシュ応答、ローカル応答、ECH メタデータのない古い記録には表示しません。

ログ保存と API では、独立した任意フィールドに状態を記録します。元の上流アドレスは変更せず、詳細行や設定項目も追加していません。

## バージョンと公式ベース

| 個人リリース | プログラムの表示バージョン | 変更内容 |
| --- | --- | --- |
| [`v0.107.79.1`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.1) | `v0.107.79-ech.1`（旧表記） | 上流 ECH 対応の追加 |
| [`v0.107.79.2`](https://github.com/JieMeiDev/AdGuardHome/releases/tag/v0.107.79.2) | `v0.107.79.2` | クエリごとの ECH 状態表示と最新バージョン表記の統一 |

両リリースは以下の公式ソースをベースにしています。個人リリースの番号は、同じ番号の公式版が公開されたことを意味しません。

| コンポーネント | 公式バージョン | ベースコミット |
| --- | --- | --- |
| [AdGuardHome](https://github.com/AdguardTeam/AdGuardHome) | `v0.107.79` | `05ba17b282da1c4393d6a4ba4db0cf519194a362` |
| [dnsproxy](https://github.com/AdguardTeam/dnsproxy) | `v0.83.2` | `acf2b30e05202e171bbbaf4a3176ea07edb964b3` |

リポジトリには両モジュールの完全なソースを含めています。モジュールのインポートパスと依存バージョンは維持し、ルートの `go.work` で同梱の dnsproxy を選択します。[SOURCE-MANIFEST.json](SOURCE-MANIFEST.json) に変更したソースファイルの SHA-256 を記録しています。

## ソースからビルドする

Go **1.27.1**、Node.js **26.3.0** でビルドを検証しています。Go、Node.js/npm、`shasum` を用意し、リポジトリのルートで次を実行します。

```sh
./build.sh
```

スクリプトはフロントエンドをビルドして組み込み、Linux amd64・macOS amd64 の実行ファイルと SHA-256 チェックサムを `artifacts/` に出力します。プログラムのバージョンはルートの [VERSION](VERSION) から読み取ります。Go の実行ファイルを指定する場合は `GO=/path/to/go` を設定してください。

## 検証状況

ローカルの TLS/H2/H3 エンドポイントを使った ECH テスト、関連する race テスト、実際の上流との H2/H3 ECH 通信を検証済みです。OpenWrt への導入でも名前解決と Web 画面の状態表示を確認しています。

テスト対象には、ECH の受け入れ、設定の更新、拒否、レコードの欠落・破損、証明書エラー、設定取得先の切り替え、キャッシュ、並行処理、fallback が含まれます。`.2` では接続再利用、最終回答の上流への帰属、ログの永続化、古い記録との互換性、フロントエンド表示も検証しています。

```sh
go -C dnsproxy test -race ./upstream -run '^(TestECH|TestUpstreamDoH)' -count=1
go -C dnsproxy test -race ./proxy -run '^(TestECHQuery|TestFallback$|TestProxy_Exchange)' -count=1
go -C AdGuardHome test -race ./internal/querylog ./internal/dnsforward ./internal/client -count=1
```

フロントエンドの表示テスト 5 件、TypeScript チェック、変更ファイルの ESLint チェックは通過しています。`AdGuardHome/client` で `npm ci` を実行した後、ルートから次を実行できます。

```sh
npm --prefix AdGuardHome/client test -- src/__tests__/upstream.test.ts
npm --prefix AdGuardHome/client run typecheck
```

公式のテストスイート全体は、まだすべて通過していません。既存テストの一部は `8.8.8.8:53` や `1.1.1.1:53` などの公開サービスに依存し、テスト環境ではタイムアウトします。`make go-check` を含む公式 CI 全体の検証は未完了です。

## 公式プロジェクトへの提案

[AdGuardHome issue #1681](https://github.com/AdguardTeam/AdGuardHome/issues/1681#issuecomment-6260302573) に設計の議論を投稿しています。まず dnsproxy に通信機能を統合し、その後 AdGuard Home の設定と依存関係を更新する方針を提案しており、メンテナーの返答を待っています。PR は提出しておらず、この実装が公式に採用されたわけではありません。

## ライセンス

AdGuard Home とこの組み合わせ配布は GPL-3.0 ライセンスを維持しています。[LICENSE](LICENSE) と[元のライセンス](AdGuardHome/LICENSE.txt)を参照してください。dnsproxy は [Apache-2.0 ライセンス](dnsproxy/LICENSE)を維持しています。各ファイルの著作権表示と第三者のライセンス表示も保持しています。

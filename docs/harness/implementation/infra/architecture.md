# インフラのアーキテクチャ

- A-I01: `infra/bin/` は起動、`lib/config/` は環境設定、`lib/stacks/` は構成の組立、`lib/constructs/` はnetwork・database・container・albなどの責務を持つ。
- A-I02: DockerImageAssetは既存 `server/Dockerfile` とserverのビルドコンテキストを使う。別backendを作らない。
- A-I03: 現行AWS経路はALB → private subnetのECS Fargate → privateなRDS PostgreSQL。DB接続と外部モデル用Secretはサーバーへ渡し、mobileへ渡さない。
- A-I04: ローカルはserverのComposeでPostgreSQL、ホストでGoを実行する。CDKとComposeは実行環境が異なるが、同じAPIとPostgreSQL adapterを使う。
- A-I05: ヘルスチェック、公開ポート、環境変数はサーバー側の契約と一致させる。設計変更はサーバー・インフラ両方を確認する。

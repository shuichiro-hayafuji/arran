# インフラのアーキテクチャ

- A-I01: `infra/bin/` は起動、`lib/config/` は必須入力と東京production制約、`lib/stacks/` はLightsail・static IP・S3・backup IAMの組立、`operations/` はホストのComposeと運用を担当する。旧ECS用constructは標準配備から除去する。
- A-I02: APIは既存 `server/Dockerfile` をlinux/amd64でビルドしてホストへ搬入する。CDK synthでAPIをビルドせず、別backendを作らない。
- A-I03: AWS経路は公開Caddy HTTPS → 非公開API → 内部networkのPostgreSQL 16。公開は80/443と明示管理CIDRのSSHだけ。API/DBのホストportを公開しない。NAT・ALB・ECS・RDSは作らない。APIは外部モデルへ接続できるがDBは内部networkのみ。
- A-I04: ローカル開発はserverのComposeとGo、本番はinfra/operationsのComposeで同じAPI/adapterを使う。本番通常起動はruntime DBロールで既知スキーマを確認するだけ。移行者の秘密はone-off migrateだけに渡す。
- A-I05: ヘルスチェック、公開ポート、環境変数はサーバー側の契約と一致させる。設計変更はサーバー・インフラ両方を確認する。
- A-I06: LightsailはEC2 instance profileを使えない。S3書込み専用IAMユーザーの鍵をホストrootだけで保持し、コンテナへ渡さない。CDKでは鍵を作らず、秘密をuserdata/outputへ含めない。
- A-I07: ホスト/static IP/S3は保持、S3はprivate・暗号化・versioning・postgres/の現行版7日expiry/旧version1日・期限切れdelete marker除去。日次snapshotとpg_dumpは別の復旧経路。restoreは新規DBのみ、公開切替は確認後に別途行う。

詳細・料金・検証境界は [構成概要](overview.md) と [運用手順](../../infra/operations/README.md)。

- A-I08: 初回権限は [専用bootstrap](../../infra/bootstrap/README.md) を管理者が作成する。通常配備roleは本番stackだけ、CF実行roleは本番資源だけを扱い、backup writerには管理者所有のpermissions boundaryを必須とする。生成元とJSON、公式権限照合、逸脱防止テストを同じ変更で更新する。

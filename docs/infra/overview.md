# Infraの構成概要

東京のLightsail 2GB 1台に、既存Go APIとPostgreSQL 16、HTTPS入口のCaddyを配置します。公開DNS名は必須です。static IPを固定し、日次snapshotとprivate S3へのpg_dumpで復旧に備えます。旧ECS・ALB・RDS構成は標準配備では使いません。

構成図・費用・容量制約・検証コマンドの正本は [インフラREADME](../../infra/README.md)、初回・更新・復旧は [運用手順](../../infra/operations/README.md) です。

- [インフラ業務ルール](business.md)
- [インフラ実装ルール](architecture.md)
- [インフラ実装時の作法](conventions.md)
- [AWS配備の残課題](../../infra/REMAINING_TASKS.md)

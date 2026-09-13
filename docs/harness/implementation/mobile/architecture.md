# モバイルのアーキテクチャ

- A-M01: 機能は `mobile/lib/features/<feature>/` に配置する。`screens/<screen>/` にScreen・ViewModel・State・Intent、`providers/` に状態やRepositoryの組立、`repository/` にAPI操作、`repository/dto/` にJSON変換、`domain/` に画面で使うモデルを置く。sessionの既存 `provider/` は例外として維持する。
- A-M02: 通常経路はScreen → MVI ViewModel → Repository → ApiClient。ScreenからHTTPやDTO変換を直接実行しない。Providerは組立のためRepository・ViewModel・ApiClientを参照できる。
- A-M03: RepositoryはDTOをDomainへ変換して返す。Domainへ画面・Provider・ネットワークの依存を入れない。multipart等の転送型を扱う既存DTOは許容する。
- A-M04: 通常のHTTP処理は `core/network/` に集約し、認証付きAPIはauthorized clientを使う。認証専用のDioは [EX-M01](../../WORKFLOW.md) に従う。
- A-M05: `app/` はルーター・テーマ・アプリ構成、`shared/` は機能に依存しない共通部品。共有処理へ特定機能の業務を移さない。既存coreのセッション参照を一律禁止するルールは導入しない。
- A-M06: 新しい機能間参照は必要性と更新経路を明確にする。他機能の画面内部への依存を増やさず、既存例外は [EX-M02](../../WORKFLOW.md) として扱う。

# モバイルのアーキテクチャ

本書はmobileを変更する際の設計制約。「必須・禁止」は新規実装と変更部分に適用する。現行との差異は [例外・未整備事項](../harness/WORKFLOW.md) に記録し、無関係な一括移行はしない。

## 上位構成と意思決定

**Clean Architecture + feature-first** を採用する。各featureがDomain、DTO/Repository（Data層）、Presentation層を所有する。Presentationは **Riverpod + MVI**、デザインシステムは **app/constants** に統一する。

| 決定 | 理由 | 制約 |
| --- | --- | --- |
| DEC-M01: feature内に各層を置く | 機能変更の影響と所有者を限定する | lib直下に全機能共通のdomain/repository/presentationを新設しない |
| DEC-M02: Domainを通信・UIから分離 | API変更と画面変更の影響を隔離する | JSONはDTO、画面状態はState、通信はRepository実装へ置く |
| DEC-M03: RiverpodでDI・寿命、MVIで状態遷移 | 操作から表示までを追跡・検証可能にする | 通常機能へBloc、別のChangeNotifier、グローバル可変storeを追加しない |
| DEC-M04: 現行の軽量なClean Architectureを維持 | 中継だけの層を増やさずテスト境界を確保する | Repository interfaceとImplはrepository内で同居可能。全操作へのUseCaseクラス追加やinterfaceのdomainへの一律移動はしない |
| DEC-M05: デザインをconstantsへ集約 | 色・文字・余白の一貫性を保つ | 新しい画面独自のテーマやデザイン定数群を作らない |

DEC-M04は教科書的な「Repository interfaceもDomain層に置く」配置とは異なる。Domainモデルは外部層を知らず、PresentationはRepositoryの抽象契約を利用し、Providerが実装を組み立てる。これを本リポジトリの基準とする。新しい層が必要なら再利用する処理・境界・代替案を示して設計判断を更新する。

## A-M01: featureと配置の粒度

featureは業務概念・一連の操作で区切る。画面ひとつ、APIひとつを理由にfeatureを増やさない。現在のprofile、transactions、dashboard、consultation、review、login、session、startupを出発点とする。memoriesはprofile内に属する。

```text
mobile/lib/
  features/<feature>/
    domain/                      # 業務モデル・値
    repository/
      <feature>_repository.dart  # 抽象契約・Impl・Repository Provider
      dto/                       # 通信形式とDomain変換
    providers/                   # 読取query・派生状態
    screens/<screen>/            # Presentationの実体
      <screen>_screen.dart
      <screen>_view_model.dart    # ViewModelと公開Provider
      <screen>_state.dart
      <screen>_intent.dart
      widgets/                   # 必要な画面固有部品
  app/constants/                 # 色・文字・余白・テーマ
  app/router/                    # go_router・NavigationShell
  app/exception/                 # アプリ例外の境界
  core/network/                  # 共通HTTP・認証interceptor
  shared/presentation/           # MviIntent / MviViewModel
  shared/widgets/                # 業務非依存の共通UI
  shared/utils/                  # 共通の純粋な変換等
  config/                        # ビルド時設定
```

`screens/` と並立する `presentation/` を作らない。通信不要の機能へ空のRepository/DTOを作らない。複合Stateは専用ファイル、単純な読取結果だけなら既存reviewのようにAsyncValueを直接使える。既存ImportStateのViewModel同居、sessionの単数 `provider/` は移動を必須としない。

### loginとsessionの責務

- loginはログイン画面・入力検証・送信中とエラーのMVI状態・認証APIのRepository/DTOを所有する。RepositoryにはpublicApiClientProviderのApiClientを注入し、Dioの生成・設定・破棄と通信例外変換は共通HTTPへ集約する。
- sessionはSession Domain・安全な保存と復元・期限切れ・失効・ログアウト・generationによるユーザー切替を所有する。ユーザー名・パスワードやログイン画面を扱わない。
- LoginViewModelはLoginRepositoryから受け取ったSessionをSessionController.activateへ渡す。保存失敗時のサーバー失効はsessionが担当する。sessionからloginへは依存しない。
- loginのProviderはautoDisposeとし、送信中の重複操作を抑止する。破棄またはユーザー切替後の認証応答は公開せず失効させる。Session DomainはJSONを持たず、ログイン応答DTOと保存形式はそれぞれのfeatureが所有する。

## A-M02: 依存方向と実行フロー

```text
利用者 → Screen → Intent → ViewModel → Repository interface
                                      ↓ ProviderがImplを注入
                                 RepositoryImpl → ApiClient → API
                                      ↓ DTO.toDomain()
Screen ← Riverpodで購読 ← 不変State ← Domain
```

これは実行フローであり、DomainがScreenをimportする意味ではない。

| 要素 | 許可する参照・処理 | 禁止する参照・処理 |
| --- | --- | --- |
| Domain | Dart型、関連Domain、純粋なルール | Flutter、Riverpod、Dio、DTO、画面、JSON入出力 |
| DTO | JSON解析、Domain変換、必要なmultipart型 | Provider、UI状態、Widget、画面遷移 |
| Repository interface | Domain・Dart型で操作契約を表現 | 公開APIにResponse、FormData、DTO、Ref、BuildContextを露出 |
| RepositoryImpl | ApiClient、DTO、Domain、共通解析 | UI描画、状態通知、Router、他画面更新 |
| Provider | DI、ViewModel公開、読取query、派生状態 | JSON変換、業務更新フロー、Widget構築 |
| ViewModel | Intent、State、Domain、Provider経由のRepository抽象 | Dio/ApiClientの直接使用、DTO、Widget、BuildContext、Navigator |
| Screen | State購読、Intent送信、Domain表示、テーマ、UI副作用 | Repository/HTTPの直接実行、JSON解析、金融判断 |

Providerはcomposition rootとしてApiClientとRepositoryImplを知ってよい。RepositoryのDI定義は対応する`*_repository.dart`へ抽象契約・Implと集約し、構築と契約の参照先を揃える。利用側は同ファイルからProviderを参照するが、RepositoryImplを直接生成せず抽象契約を利用する。Repositoryの操作契約・ImplにはRefや状態通知の責務を追加しない。ViewModelのRef利用は現行方針として許可し、純粋Domainと区別する。共通HTTPは認証付きApiClientを原則とし、認証専用経路はEX-M01に限定する。

## A-M03: Domain・DTO・Repositoryの責務

### Domain

- 業務上の意味とライフサイクルが一単位になる型を作る。Profile、TransactionItem、Consultation等が例。CategoryAmountは値、Dashboardは集計と取引を束ねる読取モデルでよい。
- API・DBとの一対一対応を強制しない。同じ意味のProfileを画面ごとに複製しない。JSONキーを持つMapをDomain代わりに流通させない。
- loading、saving、選択タブ、入力エラーをDomainへ入れない。これらはPresentationのStateに置く。
- 不変として扱い、finalフィールドと新しい値への置換を基本とする。公開List/Mapも破壊的変更しない。必要な境界でコピー・unmodifiable化する。
- 共通モデル化は意味の所有者を確認して判断する。二箇所で使うという理由だけでsharedへ移さない。
- 表示変換・入力検証と金融判断を区別する。支出可否の正本は [B-S01](../server/business.md) でありモバイルに再実装しない。

### DTO

- APIキー、型、null/欠損、送受信項目の差異をfromJson/toJson/toDomain/必要時のfromDomainに閉じ込める。
- サーバー管理項目を更新リクエストへ機械的に送り返さない。ProfileのupdatedAtの扱いを参考にする。
- デフォルト値は契約上の意味がある場合だけ使う。不正・欠損と0円を混同しない。既存の寛容なparse関数を新規項目へ無検討にコピーしない。
- 通信実行、キャッシュ、UIメッセージ、業務判断を置かない。multipart構築はData層の転送処理として許容する。

### Repository

- featureの取得・保存をDomain中心の契約として提供する。呼出し元はAPIパス・JSONキー・Dio型を知らずに操作できること。
- interfaceで差し替え可能にし、ImplへApiClientを注入する。fake RepositoryでViewModelを検証できる構造を維持する。
- DTO変換はこの境界で完結させる。画面の状態更新・Provider invalidation・画面遷移を担当しない。
- 独自キャッシュ、再試行、別DataSource層は必要性なしに追加しない。更新の自動再試行は冪等性・重複処理を検討してから導入する。

## A-M04: Riverpod + MVI

| 要素 | 責務 |
| --- | --- |
| Model（State） | 描画に必要な不変状態。Domain結果、入力、読込・更新状態、操作エラー |
| View（Screen） | ref.watchで描画し、操作をIntentとしてdispatchへ送る |
| Intent | Load/Save/Refresh等と必要な入力。BuildContextやcallbackを持たない |
| ViewModel | MviViewModelを継承し、dispatchで処理し、Repositoryを呼び、Stateを更新 |
| Riverpod | DI、状態公開、依存追跡、ライフサイクル、ユーザー切替時の破棄 |

[MviViewModel](../../mobile/lib/shared/presentation/mvi.dart) はStateNotifier実装を使用する。通常はStateNotifierProviderで公開する。ViewModelを公開するProviderと生成関数は、対応する`*_view_model.dart`に集約する。利用側は同ファイルからProviderを参照し、読取query・派生状態のProviderは`providers/`に置く。現行のannotationと手書きProviderの併用を、annotationがあるという理由だけで生成Providerへ置換しない。

- Provider<Repository>: 抽象契約へImplを注入する。
- StateNotifierProvider<ViewModel, State>: 画面の操作・状態を公開する。
- FutureProvider<Domain>: 読取query用。保存・削除をProvider再評価で発火させない。
- family: consultationId等の識別引数で状態を分離する。
- autoDispose等: 再入場時の保持要件を決めて選択する。一律付与・削除は禁止。

ref.watchは描画・依存追跡、ref.readは操作時参照、ref.listenは遷移・Snackbar等の一回限りのUI副作用に用いる。同じ取得結果の正本をStateと別Providerへ無目的に二重保持しない。

ユーザー切替時は [main.dart](../../mobile/lib/main.dart) のgenerationに基づくProviderScope再生成を維持する。session固有のアプリ寿命ProviderContainer/ChangeNotifier/Secure StorageはEX-M01であり、通常featureの見本にしない。

## A-M05: 横断基盤とデザイン

ApiClient/interceptorはHTTP・認証・エラー変換、app/routerはgo_router・認証redirect・NavigationShellを担当する。金融業務を共通通信へ移さず、ViewModelへRouterを注入しない。

デザインシステムはapp/constantsに集約する。[デザインシステム](design-system.md) の色・文字・余白・Theme・Widget境界を必ず守る。constantsへAPIやカテゴリ等の業務定数を混ぜない。sharedは特定featureに依存しない共通部品とする。既存coreのセッション参照は認証境界として維持する。

## A-M06: feature間の協調

別featureの公開Domainは所有者を明確にして参照できる。例: DashboardがtransactionsのTransactionItemを含む。循環参照を作らない。別featureのDTO・RepositoryImpl・Screen内部を参照しない。

更新後の再取得対象を明示する。既存の相談結果→memories、CSV確定→dashboardのProvider/Intent連携はEX-M02。新規連携は公開query再取得等を検討し、他画面内部への依存を増やす場合は理由・影響を例外表へ記録する。汎用event busやservice locatorを先回りで導入しない。

## 参照実装

[Profileモデル](../../mobile/lib/features/profile/domain/profile.dart)、[DTO](../../mobile/lib/features/profile/repository/dto/profile_dto.dart)、[Repository](../../mobile/lib/features/profile/repository/profile_repository.dart)、[ViewModelと公開Provider](../../mobile/lib/features/profile/screens/profile/profile_view_model.dart)、[State](../../mobile/lib/features/profile/screens/profile/profile_state.dart) を各責務の例とする。全ルール適合済みのテンプレートではない。既存との差異はWORKFLOWを確認する。

## A-M07: 共通HTTPの再利用

- HTTP変更前に [ApiClient](../../mobile/lib/core/network/api_client.dart) と既存Providerを確認する。Dioの生成・設定・破棄はこのファイルへ集約し、他の `mobile/lib` ではDioの型参照・生成やApiClientの直接生成を追加しない。ApiClient型の注入とOptions/FormData等の転送用型は許可する。
- 通常の認証付き通信はauthorizedApiClientProvider、ログインや明示したトークンの失効はpublicApiClientProviderから注入する。後者の401と対象トークンの照合は呼び出し元が管理する。
- sessionのEX-M01は独自Dioを許可しない。画面のProviderScopeより長い寿命が必要なら、既存のアプリ寿命Containerから共通Providerを利用する。寿命の違いを理由に通信設定を複製しない。
- [チェックtool](../../tools/CHECKS.md) のmobile-architectureを実行する。構成変更が必要なら理由・影響、ルール、検出範囲と回帰テストを同時に更新し、違反を通すだけの除外を追加しない。

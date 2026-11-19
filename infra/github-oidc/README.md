# GitHub ActionsからのCDK配備

この構成はCDKによる `arran-production` インフラ更新を対象とします。Lightsail上のAPIイメージ搬入・DB migration・アプリの更新は別手順です。GitHubから直接SSHアクセスする権限や、AWSの長期アクセスキーは作りません。

## 初回設定

管理者が対象アカウント `435013325489` と東京リージョンを確認してから実施します。各CloudFormationの変更セットを確認し、想定外の削除・置換を実行しません。

1. IAMのOIDCプロバイダー一覧で `token.actions.githubusercontent.com` の有無を確認します。既存の場合はclient IDに `sts.amazonaws.com` があることを確認します。新規作成が必要か不明な場合は設定を進めません。
2. [OIDCテンプレート](arran-github-oidc.template.json) で別のCloudFormation stackを作成します。`ProviderMode` は未作成なら `Create`、既存なら `Reuse` を指定し、`CAPABILITY_NAMED_IAM` を指定します。Createで作ったプロバイダーにはRetainが付きます。既存のOIDCプロバイダーを置換・更新しません。
3. 生成された `ArranGitHubCdkDeployerTokyo` の信頼先が指定repositoryのimmutable subjectと `sts.amazonaws.com` のみ、許可先が専用deploy/file-publishing roleだけであることを読み戻します。次に [専用CDKToolkit](../bootstrap/arran-toolkit.template.json) の更新用変更セットを作り、信頼先の追加だけで既存の資源・権限を変更しないことを確認して適用します。既存brokerや本番stackを削除しません。ロールを先に作らないと、IAMが信頼先を解決できず更新に失敗します。
4. GitHub repository variablesに `ARRAN_DOMAIN_NAME`、`ARRAN_ADMIN_CIDR`、`ARRAN_LIGHTSAIL_KEY_PAIR_NAME` を、現在の本番stackと一致する値で設定します。誤ったCIDRや鍵名はアクセス制御変更やホスト置換を招くため、実stackと照合してください。
5. PRをレビューしてmainへ反映します。GitHub Actionsの `Deploy production CDK` からmainを選び手動実行し、OIDC一致、差分、対象stackの変更セットと完了状態を確認します。最初の実行はOIDC・権限・実環境を検証するもので、ローカルテストだけでは成功扱いにしません。workflow_dispatchでは入力時に選んだbranchの権限を使うため、main以外はjob自体が実行されません。

synthでは `--no-lookups` を指定し、差分の既存テンプレート読取りには本番stack限定のdeploy roleを使います。VPCなどのcontext lookupに必要な広範囲の読取り権限は含みません。

公開されたmainのHEADと実行commitが一致することを、開始時とdeploy直前に確認します。直前確認後にもmainが更新される競合は完全には防げないため、本番実行中にmainへ変更を入れない運用が必要です。CDK diffは同じ実行内に表示されますが、差分を人が確認してから同じjobが停止する仕組みではありません。`--require-approval never` はCDKの対話待ちを無効にするため、PRのレビューと設定値照合が必要です。更新失敗時は自動で旧状態に戻すことを保証しません。

OIDCは [GitHubのimmutable subject](https://docs.github.com/en/actions/reference/security/oidc#immutable-subject-claims) を前提にし、audience、repositoryと所有者のID、mainへのrefを固定します。実行時にもJWTのsubjectとIDをログへトークンを出さず照合します。JWTの署名はAWS STSが検証します。GitHubのsubject設定変更やリポジトリの移転・名前変更時は失敗するため、信頼ポリシーを広げる前に原因を確認します。

この信頼はmain上でID tokenの権限を持つ全workflowに適用されるため、mainにworkflowを追加・変更できる権限は本番配備権限に相当します。mainの保護、Actions設定、管理者・書込権限、依存関係を定期確認してください。GitHub管理者やAWS管理者自体の権限を制限する仕組みではありません。

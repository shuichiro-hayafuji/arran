# Arran専用のCDK初期設定

管理者が東京で `CDKToolkit` を一度作成し、その後は `ArranCdkDeployerTokyo` または `ArranGitHubCdkDeployerTokyo` を入口として限定roleで `arran-production` だけを配備します。標準bootstrap version 32の命名・SSM version・file asset契約を残した専用版です。AdministratorAccessへのfallback、アカウント全体へのtrust、lookup/image-publishing role、ECR、独自KMSキーは含みません。

## 管理者に渡すファイル

[アップロード用JSON](arran-toolkit.template.json) が完成テンプレートです。秘密・個別アカウントIDを含みません。`AWS::AccountId` / `AWS::Region` から対象を組み立てます。[生成元](../lib/bootstrap/template.ts) の変更後は `cd infra && npm run bootstrap:template` で再生成します。通常infraテストで生成元と配布物の一致を検査します。生成コマンドはAWSへアクセスしません。

管理者の操作手順:

1. MFA付き管理者セッションで東京 `ap-northeast-1` のCloudFormationを開き、対象アカウントを確認する。既存 `CDKToolkit` がある場合は新規作成せず、現在の構成と利用者を調べる。
2. 新しいスタックを「標準」で作成し、上記JSONをアップロードする。stack名は **CDKToolkit**。
3. Parametersは `BootstrapVariant=Arran limited production v1`、`OperatorUserName=arran-operator`。既存の `ArranCdkDeployerTokyo` roleとoperator userがあることを確認する。先に [GitHub OIDC手順](../github-oidc/README.md) でGitHub roleを作成する。両roleが存在しない場合、IAMはtrustを作成できない。
4. サービスロールは空欄。今回のbootstrap管理用権限を、本番配備用execution roleで代用しない。削除保護（termination protection）を有効にする。
5. IAM資源の作成を確認し **CAPABILITY_NAMED_IAM** に相当する確認欄をチェックして作成する。作成するのは下表のbootstrap資源で、本番Lightsailホストではない。
6. CREATE_COMPLETE後に親作業でrole trust、inline policies、boundary、S3保護、version32、削除保護を読み取り確認する。失敗した場合はイベントを確認し、AdministratorAccess追加で突破しない。

通常の `cdk bootstrap` はこの専用構成を上書きするおそれがあるため使いません。CLI更新時も標準テンプレートの差分をレビューし、生成元とテストを更新した専用テンプレートを管理者が適用します。

## 権限の分担

| 主体 | 許可対象 | 許可しない範囲 |
| --- | --- | --- |
| 既存ArranCdkDeployerTokyo | deploy/file-publishing roleの引受け | bootstrap変更、直接の本番サービス操作 |
| deploy role | arran-productionのCloudFormation変更と参照、専用execution roleのPassRole、toolkit file/version参照 | 他stack変更、bootstrap変更、任意roleのPassRole、stack削除 |
| file-publishing role | toolkitの専用S3 bucketへのfile読み書き | 他bucket、ECR、KMS管理 |
| CloudFormation execution role | 東京Lightsail、名前を固定したbackup bucket、backup writer user、専用bootstrap versionのGetParameters | IAM role管理、任意user、アクセスキー作成、boundary変更/除去、operator policy編集 |
| backup writer boundary | 専用bucketのpostgres/*へのPutObject/AbortMultipartUploadのみ | 他bucket、Get/DeleteObject、IAM操作 |
| operator参照ManagedPolicy | CDKToolkit、3role、boundary、versionの参照とDB backupのpostgres/*読取り | bootstrap作成/更新、権限変更 |

operatorの既存inline policy合計上限へ影響させないため、参照権限は別のManagedPolicyを付与します。既存ポリシーは上書きしません。3roleは1時間セッションです。deploy/file-publishingのtrustはアカウントrootではなく既存brokerとGitHub OIDC roleだけです。

CloudFormationの通常stack service roleは公式標準例どおり `cloudformation.amazonaws.com` のtrustです。`SourceArn` / `SourceAccount` が通常stackのAssumeRoleへ伝わるという裏付けがないため、extension用例を転用しません。代わりにPassRoleを専用roleへ限定し、stack変更もarran-productionだけに限定します。既存の別主体が持つ管理者権限やPassRoleを制限するSCPではありません。

## 制約と残る権限範囲

- Lightsail CreateInstances・AllocateStaticIp・add-on・Get系は公式SARにresource指定がないため、`Resource:*` と東京条件が必要です。Instanceの停止/開始/削除/port変更/tag変更は同アカウント・東京の `Project=arran` / `Environment=production` 付きInstanceに限定します。StaticIpのIDは作成前に決まらずtagを持たないCF資源なので、attachは東京の同account Instance/StaticIp全体、detach/releaseは東京の同account StaticIp全体が対象です。IAMで1台・2GB・月額上限を強制するものではありません。
- backup bucketは `arran-production-backups-ACCOUNT-REGION` の決定名です。別の既存bucketを流用しません。まだ未配備の構成に適用する変更です。配備済みbucket名を変更すると置換となるので、その場合は移行を別途計画します。
- S3 bucket policy設定権限は専用bucket内にあります。boundaryはidentity policyの権限上限であり、user ARNへ直接許可するresource policyを含むすべての迂回を防ぐものではありません。bucket policy/public access blockの変更もレビューします。
- backup userには境界付きCreate/PutUserPermissionsBoundaryと必要なread/tag/inline policyだけを許可します。login profile、group参加、managed policy付与、鍵作成、boundary削除、user削除を許可しません。RETAIN設定を維持します。作成失敗でuserが残る場合は管理者が対象と保持状態を確認して復旧します。
- S3のread handlerが参照する設定読み取りは専用bucketに許可します。replication、metadata tables、website等の未使用機能の書込みや任意PassRoleは許可しません。Lightsailの追加disk attach/detachも未対応です。機能を追加した場合はschemaとポリシーを同時に見直します。
- 取得した公開S3 schemaには `s3:GetBucketDefaultStorageClass` が記載されていますが、同時点の公式SARでは定義がなくcfn-lintも不明actionと判定するため付与していません。未知のpermissionを捏造せず、実配備時のproviderイベントで必要性を確認します。
- template/version32は未実装のECRやlookup対応を意味しません。現アプリにはcontext lookupもDockerImageAssetもなく、APIは別途搬入します。synthのmissing contextが空、docker assetが空、file destinationが専用file-publishing roleであることをテストします。CDKのdiffによる既存stack template読取りは、lookup roleのARNに本番stack限定deploy roleを指定して実行します。一般的なcontext lookup権限は付与せず、Actionsのsynthは `--no-lookups` で実行します。
- bootstrap bucketはSSE-S3・versioning・private・TLS必須・保持、旧version30日と未完了multipart1日で期限切れです。bootstrap削除やdefault版への更新は行いません。IAMにはtermination protectionのON/OFFだけを区別する条件がないため、deploy roleのUpdateTerminationProtectionは本番stackだけに限定し、ONを維持する運用です。

## 配備用ログインと確認

当面は既存のCLI signinセッション `arran-production` からSTSでbroker roleを引き受け、得た短期資格情報をCDKの子プロセス環境だけへ渡します。キーはファイルへ永続化せず、標準出力やログにも出しません。古いSDKが `login_session` を自動解決できるという前提にはしません。管理者セッションをCDKへ渡しません。

デプロイ前にdomainName・adminCidr・keyPairNameを確定し、正規check → synth → `cdk diff --no-change-set` → 対象を確認したdeployの順で進めます。広いlookup read権限や通常bootstrapは追加しません。ARNにaccount/regionの組み立てが一致し、brokerのidentity policyとbootstrap trustの両方が揃う必要があります。

```bash
python3 tools/harness.py check infra
python3 tools/harness.py check docs
cd infra
npm run bootstrap:template
# cfn-lint導入済み環境で実行
cfn-lint -t bootstrap/arran-toolkit.template.json
```

policy guardはtrust、PassRole、boundary、非admin、Resource:*例外、公式action名、生成同期とCDK file asset契約を検査します。AWS IAM評価の完全な再実装ではありません。cfn-guardは未導入のため未実行です。cfn-lintとテスト成功は、実アカウントでのCloudFormation/provider認可成功を保証しません。

## 公式照合資料

2026-11-15に公式 [Service Authorization Reference JSON](https://docs.aws.amazon.com/service-authorization/latest/reference/service-reference.html) と [CloudFormation公開schema](https://schema.cloudformation.us-east-1.amazonaws.com/CloudformationSchema.zip) を読み取り、今回使うactionの対応とhandler permissionsを [照合記録](authorization-reference.json) へ保存しました。未使用property向けのhandler permissionを無条件に全付与するものではありません。

- [CDK bootstrapのカスタマイズ](https://docs.aws.amazon.com/cdk/v2/guide/bootstrapping-customizing.html)
- [CloudFormation service role](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-iam-servicerole.html)
- [CloudFormation最小権限のservice role](https://docs.aws.amazon.com/prescriptive-guidance/latest/least-privilege-cloudformation/service-roles-for-cloudformation.html)
- [IAM permissions boundaryの評価](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_boundaries.html)

## 既存bootstrapの修正: version参照権限

2026-11-16の本番CreateChangeSetで、CloudFormation execution roleに `ssm:GetParameters` がないため、テンプレート内のbootstrap versionを解決できないことを確認しました。deploy roleの参照権限だけでは不足します。修正版はexecution roleに `/cdk-bootstrap/hnb659fds/version` の正確なARNへの `ssm:GetParameters` だけを追加します。他parameterの読取り・パス列挙・書込みは許可しません。

既にCDKToolkitがある場合、管理者が上記の生成JSONで**既存スタックを更新**します。新規スタック作成や通常の `cdk bootstrap` は実行しません。既存Parameters、サービスロール空欄、削除保護を維持し、変更セットがexecution roleのinline policy追加だけであることを確認して適用します。更新後に限定roleで本番CreateChangeSetを再試行します。ローカルの検証成功とAWSでの再試行成功は区別して記録します。

## 復元用operatorのbackup参照

復元に使う短期operatorへ、専用backup bucketの `postgres/*` に限るGetObject/GetObjectVersionと、`s3:prefix=postgres/*` 条件付きListBucketを追加します。`s3://BUCKET/postgres/` 配下を指定して一覧・取得し、bucket rootの一覧は許可しません。他bucketや削除・書込み・ListBucketVersionsは追加しません。同じbackup bucketのGetLifecycleConfiguration/GetBucketVersioningも別statementで許可し、実保持設定を検証します。これらの設定取得にprefix条件は付けません。ホストのbackup writerとそのboundaryは書込み専用のままです。

この参照権限はbootstrap ManagedPolicyの変更なので、管理者による既存CDKToolkitへの生成テンプレート更新が必要です。アプリ側の7日保持へのCDK更新とは別の変更として確認します。必要なbackup取得権限を管理者/rootの長期キーで代用しません。

## GitHub Actions経由のCDK配備

GitHub OIDCの導入手順は [専用手順](../github-oidc/README.md) を参照します。更新後のdeploy/file-publishing roleは、既存brokerに加えて `ArranGitHubCdkDeployerTokyo` だけを信頼します。後者はGitHub OIDCの対象リポジトリとmainに限定し、標準のbootstrapや任意のAWSサービス権限は付与しません。導入前のAWS実機検証が必要です。

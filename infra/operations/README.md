# Lightsail運用手順

この手順のAWS作業・秘密入力は認証した管理者が行います。コードだけでは配備完了になりません。設定ファイルはホストの `/opt/arran` に置き、root所有・ディレクトリ0700で管理します。秘密はGit・チャット・Issue・コマンド引数・CloudFormationへ貼りません。`set -x`、`docker inspect` 全量や環境変数全量のログ保存を避けてください。

## 1. 配備前

1. 所有するAPI用DNS名を決めます。CDKの `domainName` とComposeの `DOMAIN` に同じ値を使います。ドメイン未確定のままIPだけで本番公開しません。
2. AWS CLIの短期ログインで対象アカウントを確認します。東京 `ap-northeast-1` のLightsail鍵ペアを作成/選択し、秘密鍵は管理者端末だけで保管します。SSH管理CIDRは管理者の固定IP `/32` を基本にします。開放目的で `0.0.0.0/0` に変更しません。
3. `aws lightsail get-bundles --region ap-northeast-1` と `get-blueprints` で `small_3_0`（2GB、IPv4、$12/月）と `ubuntu_24_04` が利用可能か確認します。利用不可なら別プランを暗黙選択せず、構成と費用を見直します。
4. 旧スタックやRDS/ALB/ECS/NAT等が残っていないか確認します。本CDKは名前の異なるproductionスタックなので、旧リソースの停止・移行・削除は自動ではありません。
5. リポジトリルートで `python3 tools/harness.py check infra`、`check docs`、`python3 infra/operations/verify-compose.py` を実行します。[DB権限回帰](../../tools/CHECKS.md) は専用DBを使う `check postgres-roles` で別途確認します。
6. `infra` で `npm run cdk -- synth -c domainName=... -c adminCidr=... -c keyPairName=...`、同じcontextで `diff` を確認後に `deploy` します。初回は [専用bootstrap手順](../bootstrap/README.md) の限定テンプレートを管理者がCloudFormationへアップロードします。通常の `cdk bootstrap` は使いません。deployのIAM権限にはCloudFormation、Lightsail、S3、IAMユーザー/ポリシー等が必要です。権限不足をrootアクセスキー作成で回避しません。

CloudFormationはホスト・static IP・backup bucket・書込み専用IAMユーザーを作成します。秘密鍵やAWS access keyは作りません。ホストとIPとbucketはRETAIN、スタックはtermination protectionです。ただしLightsailコンソールからの直接削除まで防止するものではありません。

## 2. ホストの準備

1. Outputsのstatic IPへDNS Aレコードを向けます。IPv6公開は未設定なのでAAAAは作りません。Lightsail firewallは80/443全体、SSHは指定CIDRだけを確認します。ホストにも独自の公開DB/API portを追加しません。Docker公開portはUFW設定だけに依存しないため、Composeのport定義を確認します。
2. SSHで接続しOS更新を行い、[Docker公式Ubuntu手順](https://docs.docker.com/engine/install/ubuntu/) でDocker EngineとCompose plugin、[AWS公式手順](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) でAWS CLI v2を導入します。提供元の署名確認を行います。Docker権限はroot相当なので一般ユーザーへ無条件に配布しません。OS再起動/更新の保守時間を確保します。
3. 本ディレクトリの追跡対象ファイルだけを `/opt/arran` へコピーしroot所有にします。`secrets/`、`deploy.env`、`backups/` は端末から共有しません。スクリプトは実行可能、設定はroot以外書込み不可にします。`state/` はこのホストのDB適用状態なので、更新時に削除・上書き・別ホストからのコピーをしません。
4. 開発端末で同じ `server/Dockerfile` をビルドします。例: `docker build --platform linux/amd64 -t arran-api:COMMIT_SHA server`。検証済みイメージを `docker save` → SSH転送 → ホストで `docker load` します。ホスト上の2GBでコンパイルしません。タグを上書きしない運用とし、`docker image inspect --format '{{.Id}}' arran-api:COMMIT_SHA` をrelease記録へ残します。
5. `deployment.conf.example` を `deploy.env` へコピーします。APIイメージ、所有DOMAIN、backup bucketを設定し、PostgreSQL16とCaddy2は検証済みlinux/amd64の `repository@sha256:...` を指定します。適当なdigestを捏造せず、取得元とplatformを確認してpullします。設定ファイルはshellとして読み込むためroot所有0600にします。秘密はここへ記載しません。
6. rootで `./init-secrets.py` を一度だけ実行します。既存秘密の上書きを拒否します。`secrets/` は0700、API用URLとOpenAIキーはuid10001・0400。PostgreSQL用パスワードはイメージのUID差を許容するためroot所有0444ですが、rootのみ入れる親ディレクトリで保護し、必要なファイルだけをDBへマウントします。Composeのfile secretは暗号化保管ではなくbind mountです。
7. OpenAIを利用する場合は安全なホスト側入力で `secrets/openai_key` を埋めます。ファイルのuid10001/0400を維持し、キーをshell履歴や引数へ含めません。`init-secrets.py` はDB初期化後に再実行しません。パスワード変更はDBのALTER ROLEと対応URLファイルを保守時間に同期変更します。APIへ移行者/管理者のパスワードを渡しません。

## 3. 外部バックアップの資格情報

LightsailにEC2 instance profileは付けられません。この構成はCDKが作成した `arran-production-backup-writer` だけにアクセスキーを発行し、ホストrootの `~/.aws/credentials`（0600、親0700）へ `aws configure --profile arran-backup` の対話入力で保存します。IAMコンソールログイン権限は付けません。個人の管理者/rootキーを置きません。

ポリシーは対象bucketの `postgres/*` へのPutObject/AbortMultipartUploadのみです。読取り・一覧・削除は許可しません。versioningにより同名上書きから旧versionを保持しますが、Object Lockによる不変保証ではありません。書込みを悪用した容量増加は防げないため費用監視が必要です。S3復元は別の短期operator資格情報で行い、backup writerへ読取権限を追加しません。

鍵は定期ローテーションし、新キーでbackupを成功確認後に旧キーを無効化・削除します。snapshotにはホスト秘密も含まれるため、復元先の管理権限を絞り、漏えい疑い時はAWSキー・OpenAIキー・DBパスワードを更新します。バックアップ本体も個人データなので公開しません。

## 4. 初回配備と日次運用

rootで `/opt/arran/deploy.sh initial` を実行します。新しいDB volumeだけにbootstrap roleがDBと2つのLOGINロールを作ります。migrateは移行用URLだけを受け取り、API通常起動はDDL権限のないruntime用URLだけを受け取ります。移行後にruntimeへDML権限を付け、移行履歴はSELECTだけに戻します。外部backupが成功するまでは公開API/Caddyを起動しません。APP_ENVIRONMENT=productionなのでAPIは未適用/未知スキーマを拒否します。

初回backup成功後にAPIのhealthyを待ってCaddyを起動し、DNSの80/443経由で証明書を取得します。初回backupと公開 `https://DOMAIN/health` の確認まで成功して初回スクリプトが完了します。ACME障害やDNS未反映の場合は公開完了としません。稼働開始後に次を確認します。

- 外部から443でhealth、認証なし業務API拒否、別ユーザーのデータ拒否、モバイルの主要操作。
- 外部から5432/8080に接続できず、SSHは管理CIDRだけ。DB/APIの `docker compose ps` に公開portがないこと。
- Linuxの空きメモリ・ディスク、Lightsail CPU burst capacity、API/DB再起動回数。負荷によるOOM、disk full、継続CPU使用を確認するまで収容人数を保証しないこと。
- S3にdump/checksum/release manifestが揃い、下記restoreに成功すること。dump内容の一覧検査だけで復元確認済みとしないこと。

rootでtimerを導入します。

```bash
install -m 0644 /opt/arran/arran-backup.service /etc/systemd/system/
install -m 0644 /opt/arran/arran-backup.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now arran-backup.timer
systemctl list-timers arran-backup.timer
```

日次18:30 UTC（日本時間03:30）にpg_dumpを取得します。整合したcustom形式dump・SHA256・image情報をS3へアップロードし、全成功後だけ `backups/last-success-at` を更新します。失敗はservice非0とjournalに残ります。**毎日最終成功時刻と `systemctl --failed` / `journalctl -u arran-backup.service` を確認し、24時間超の遅延や失敗を放置しません。自動PagerDuty通知はまだありません。** ホスト停止時はtimerも実行できません。日次snapshotは18:00 UTCで別に動き、Lightsail側で成功状態を確認します。

S3の `postgres/` は現行objectを作成から7日で論理的に期限切れにし、旧versionは旧版になってから1日で削除対象にします。期限切れdelete markerも別ruleで除去します。日次取得した直近7日分を対象とする方針ですが、versioningとS3の非同期削除があるため、全データが厳密に7×24時間以内に物理削除される保証ではありません。短縮前の古いbackupも適用後は削除対象になるため、必要な復旧点は事前確認します。自動snapshotの保持仕様/最新成功をコンソールで確認し、必要な長期保管は追加料金込みで別途決めます。S3 lifecycleのため「保持=永久」ではありません。

## 5. 更新

1. 新APIイメージをローカル検証・搬入し `deploy.env` のAPI_IMAGEを新しい不変tagにします。旧imageとrelease記録を保持します。DB/Caddy更新もdigest単位で検証し、PostgreSQL majorをこの手順で変更しません。
2. 保守時間を案内し `./deploy.sh update`。公開CaddyとAPIを止め、現DBをS3 backup → one-off migrate → runtime権限付与 → backup → API/Caddy起動 → HTTPS healthの順に進みます。
3. backup/migration失敗時は停止状態を調べます。DDL適用後に旧APIを盲目的に起動しません。移行は冪等ですが、未知/欠損履歴は失敗させます。修正済みimageで再実行するか、下記の別DB復元を確認して切り替えます。

### CSVプレビュー永続化の初回切替（#31）

旧APIは未確定プレビューをプロセス内に保持するため、新APIと並行稼働しても共有できません。初回切替では保守時間を設け、旧APIへの新規アクセスを止め、処理中のリクエストが終わったことを確認してから旧APIを停止し、移行後に新APIだけを公開します。複数インスタンスを使う構成では、すべての旧APIが停止したことを確認します。現在の `deploy.sh update` はCaddyとAPIを停止してから移行しますが、リクエストの完了を検証する仕組みや複数ホストの停止制御はありません。運用者による停止・確認が必要です。

利用者には、切替前に作成した未確定プレビューは切替後にCSVを再選択して作り直すことを案内します。確定操作の応答が途切れた場合は、同じプレビューIDで確定を再送します。再送が成功して確定結果を受け取った場合は処理を終え、再取込しません。取込履歴で見つからない場合や再送がエラーになった場合は、未保存と決めつけず運用者に確認します。新しいプレビューによる取込は、元の確定が未保存と確認できた場合に限ります。旧版の0明細の確定結果は永続化されないため、成否を復元できません。旧版と新版の混在時はプレビューの共有を保証せず、切替や旧imageへの復帰を自動的な互換運用とみなさないでください。

この手順はコードと構成に基づく切替案です。実際の本番DB・AWSでの停止、通信途絶時の挙動、端末への案内は未検証です。

### 途中失敗からの再開とrelease記録

`deploy.env` のAPI_IMAGEは次回予定、実行中コンテナは現在のプロセス、`state/applied-release` はDBに移行を適用済みのreleaseです。この3つを混同しません。日次backupの `release.txt` は `state/applied-release` をコピーし、予定値や旧コンテナから推測しません。API tagと実image ID、PostgreSQL imageを記録します。

配備はschema変更前に `state/pending-release` へ実行モードと対象image IDを保存し、移行成功直後に `state/applied-release` を更新します。権限付与・S3 backup・公開healthのいずれかが失敗してもpendingを保持します。初回にmigration後のbackupが失敗した場合も、DB volumeや秘密を作り直さず、AWS権限等の原因を直し、**同じ設定・同じimageで `./deploy.sh initial` を再実行**します。更新途中なら同様に `./deploy.sh update` です。再開には以前のAPIコンテナは不要です。対象tagの上書きやmode/imageの変更は拒否します。別imageが必要になった場合はDB履歴と移行の適合性を調べる復旧作業に切り替え、pendingを単に削除して突破しません。

移行途中のDBを古いrelease名で保存しないよう、日次backupと配備は共通lockを取り、pendingが残る間は日次backupを拒否します。配備内では同じlockを継承し、適用済みreleaseを書いた後のbackupだけを許可します。途中失敗はサービス停止だけでなく日次backupにも影響するため、次の日まで放置せず再開します。`--deployment-lock-held` は配備スクリプト内部用で、手動実行には使いません。既存DBに対応記録がない場合も、予定設定からreleaseを自動作成せず失敗します。

停止前に新DB imageのpull/互換性確認を終え、同時にmajor変更をしないでください。リリース直前のbackupがアップロードできない状態ではschemaを変更しません。

## 6. 復旧・復元訓練

### CloudFormation rollback後の保持資源

失敗stackを削除してもRETAINの資源は残るため、同名資源を通常deployで作り直す手順にはしません。保持資源・削除されたpolicy・logical IDと物理IDを確認し、手動IMPORTで管理へ戻します。削除保護解除や失敗stack削除は、保持対象と復旧手順を確認した管理者が行います。

2026-11-16の復旧では、新stackへのIMPORT時にRoleArn/Tagsを同時追加すると拒否されました（資源変更なし）。同名 `arran-production` に一時的な `AWS::CloudFormation::WaitConditionHandle`（`ImportScaffold`）だけのstackを、既存execution role・tags・削除保護付きで先に作成しました。同じscaffoldを残したテンプレートで保持Host/S3/Userの3資源だけをIMPORTし、変更セットが3Importのみであることを確認しました。IMPORT_COMPLETE後、保持3資源に変更がないCDK差分を確認し、固定IP・bucket policy・user inline policy追加とscaffold除去だけを通常更新しました。

任意のstackを自動修復する手順ではありません。IMPORTに通常の追加・更新・削除を混ぜず、対象と差分を都度確認します。UPDATE_COMPLETE・削除保護ON・SSH指紋一致はインフラ復旧の確認であり、API/DB起動やHTTPS公開完了とは分けて扱います。

### DBの復元

**復元DBへの切替前から外部AI送信を停止し、照合が完了するまで再開しません。** 過去時点のDBには、その後の同意・撤回や未完了・結果不明の送信記録が反映されず、撤回済みの同意や結果不明markerも巻き戻り得ます。復旧点以降の同意・撤回・未完了情報を、復元DBとは独立した信頼できる証跡と照合し、安全な状態を確認してください。照合に必要な証跡や根拠がない場合は送信停止を維持します。旧APIには送信前の同意gateがないため、旧APIの起動や切戻しでこの停止を迂回しません。独立証跡の確保・照合方法は未整備であり、この手順は復元後の外部送信の再開を保証しません。

復元は本番DBを直接上書きしません。初回公開前・変更後・定期的に行い、所要時間と復元データ時点を記録します。ホスト喪失時はsnapshot復元で新ホストを用意するか、同じOS/Composeと新しい秘密でホストを再構築します。既存ホスト/IPをCDKで無断置換せず、静的IP切替は復旧確認後に行います。

1. operator短期資格情報でS3の成功した同一prefixから `database.dump`、`database.sha256`、`release.txt` を管理者端末に取得し、復元先のroot専用ディレクトリへ転送します。backup writer資格情報では取得できません。bucket、日時、3ファイルの揃い、対象releaseを確認します。データ内容をログ出力しません。
2. 復元先でPostgreSQL16を起動し、新ホストの場合はdb-initで新しいロールと空のarran DBを作ります。dumpはrole秘密を含まないため、新ホストの秘密とロールを使います。
3. rootで `./restore.sh /ABSOLUTE/BACKUP_DIR arran_restore_YYYYMMDD`。checksum確認後、存在しないDBのみ作り、single transactionでrestoreします。同名DBがあれば失敗します。失敗した練習用DBの削除も対象を確認して別途行います。
4. スキーマ履歴・行数/所有者・主要データを本番の期待値と照合します。対応するreleaseのAPIを、復元DB専用runtime URLで検証します。外部AI送信を停止したまま検証します。runtimeで通常起動でき、DDLと移行履歴書込みが拒否されることを確認します。
5. **ここまででは本番接続先を変更しません。** 公開切替はデータ時点・欠損許容量・利用者影響を確認した管理者が保守時間に行います。API/Caddy/timerを停止し、postgresへ接続中のアプリがないことを確認します。既存 `arran` は削除せず一意な退避名へrenameし、検証済み復元DBを `arran` へrenameします。既存名がある場合は止めます。SQLの対象名を事前レビューし、postgres DBへの管理接続で実行します。通常runtime URLのDB名とbackup対象は `arran` 固定なので、名前を変えたまま起動しません。切替後のDBと対応imageを検証してから、選んだbackupの `release.txt`（BACKUP_UTC行を除く）を `state/applied-release` に保存します。退避DBの古いapplied/pending記録は別途保管し、復元先のDBと混在させません。pendingを解除するのは復旧内容を確認した管理者だけです。
6. 対応するAPI imageで起動し、外部AI送信の停止を維持したままHTTPSと主要操作、backupを再確認してtimerを再開します。旧DB/旧ホストの削除は保管期間・請求を確認した別作業にします。旧imageへの単純切戻しはschema互換性を確認できる場合だけにします。

全ホストdisk snapshotは補助です。DB整合性を確認したpg_dump restoreを省略しません。ホスト/DB/S3へのアクセスを失った状態、期限切れ後のbackupは復旧を保証できません。

## 7. ローカルでの検証

`verify-compose.py` はdaemon不要でComposeを実際に解決し、公開portがCaddyだけ、APIとmigrateの同一image、秘密の分離、内部DBnetworkを検査します。AWSへのアクセスはしません。

DBの実行検証は本番secretを使わず、別の一時ディレクトリにoperationsファイルをコピーし、別のCompose project名を `-p arran-lightsail-test` で指定します。ダミー秘密でdbだけ起動 → migrate → grant-runtime → runtime URLのAPIでhealth → runtimeによるCREATE拒否 → pg_dump → 新規DBへのpg_restore → 履歴/件数比較を実行します。本番のdeploy.envやAWS profileは持ち込みません。終了時は**検証projectだけ** `down -v` します。本番で `down -v` は実行しません。

Linux/amd64 API・PostgreSQL・Caddyの組合せ、TLS/ACME、S3実書込み・snapshot・systemd timerはそれぞれ実環境での検証が必要です。ARMの開発機で代替imageを使ったDB試験をamd64本番起動の証拠にはしません。

OpenAIキー未設定時はサーバーの既存fallback/unavailable動作を保ちます。既定モデル `gpt-5.6-terra` はサーバー設定と同じですが、契約アカウントでの提供可否・単価を配備時に確認します。AWSの月5,000円目安にOpenAI API課金は含みません。キー設定後はサーバー側相談上限とプロバイダー側予算を確認してから利用を開始します。

外部AIへの送信には、審査済みの説明本文と版を管理者権限で `ai_consent_policy` に同時に設定し、全APIの `AI_CONSENT_VERSION` を同じ版にする必要があります。版や本文の既定値はありません。未設定・不一致・未同意・DB障害では外部送信を拒否します。版と本文の組は初回登録時に履歴へ固定され、同じ版の本文書換えや異なる本文での再利用はできません。設定前に送信内容・事業者・処理地域・保存期間の確認と公開説明を完了してください。

旧APIには送信前の同意チェックがないため、初回導入時は旧APIをすべて停止してから新版を起動します。旧版と混在させたまま同意管理を有効にしません。送信結果不明の記録は自動削除されず、撤回や説明版の変更が保留されます。外部側の処理終了を安全に確認する方法と、権限・監査を含む解消手順は未整備のため、公開運用前に整備してください。

### 同意撤回が保留された場合の確認

撤回APIの応答が503または通信途絶になった場合、撤回完了と判断しません。利用者には完了を確認できないことを伝え、本人に同じアカウントで撤回を再実行してもらいます。状態のGETだけでは撤回保留は解除されません。再実行後も保留が残る場合は管理者へ引き継ぎ、完了と未確認を区別します。運用者は承認されたDB読取権限で、未完了記録の件数と最も古い開始時刻、撤回保留の件数を確認できます。次のSQLは集計の読取りだけで、相談内容や利用者IDは出力しません。

```sql
SELECT count(*) AS inflight_count, min(started_at) AS oldest_started_at
FROM ai_consent_inflight;
SELECT count(*) AS pending_withdrawal_count
FROM ai_user_consents WHERE withdrawal_pending = TRUE;
```

これらの値は取得時点の集計で、進行中の正常な送信も含みます。時刻が古いことやAPIプロセスが終了したことだけでは、外部事業者の処理完了を証明できません。現在の記録には外部事業者の状態を照会するIDがなく、結果不明の送信を後から安全に完了と判定する手順はありません。記録の手動削除、期限による自動解除、撤回完了の断定は行わず、事象と対応状況を管理者へ引き継ぎます。具体的な利用者を調べる場合は、個人情報を含む運用情報としてアクセス権と記録先を限定します。

上記の集計は運用上の切り分けであり、未完了記録の解消や外部処理の終了確認ではありません。専用運用権限、監査、解消時の根拠と手順は未整備です。

配備再開・manifestの回帰は通常 `check infra` からPythonで実shellを実行します。Docker/AWSのコマンドは分離したダブルで、初回backup失敗、変更image拒否、予定imageとDB適用imageの差、更新後backup失敗、移行途中失敗、競合lockを確認します。これはS3実通信やDB実移行の代用ではありません。

### 退会申し込みの確認（暫定）

申し込みは本人の認証済みAPIでDBに記録されます。記録は運営者への自動通知や削除完了を意味しません。運営者は権限を限定した接続で `SELECT user_id, requested_at FROM withdrawal_requests ORDER BY requested_at;` を定期的に確認し、取得内容を個人情報として保護してください。具体的な運用頻度、本人への連絡手段、確認済み管理、削除手順とバックアップからの復元後の再適用は未整備であり、一般公開前に整備してください。申し込みだけでユーザーの無効化やデータ削除を実行しません。誤削除や再出現を避けるため、記録を手動削除して完了と扱わないでください。

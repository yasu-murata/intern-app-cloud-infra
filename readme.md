# クラウドインフラの世界に触れよう

## 全般
各所でAPIの有効化が求められるため、「y」で回答する
```shell
# 以下のような表記の場合、API有効化が必要
API [compute.googleapis.com] not enabled on project [plucky-avatar-469606-r4]. Would you like to enable and retry (this will take a few minutes)? (y/N)?
```

## Cloud SQLの構築

### プライベート接続用のネットワークを作成
VPCネットワークとGoogleサービス間のプライベート接続用のIP範囲を予約
```shell
gcloud compute addresses create google-managed-services-default \
    --global \
    --purpose=VPC_PEERING \
    --prefix-length=16 \
    --network=default
```
作成されたものを確認
```shell
gcloud compute addresses list
```
Peering 接続を作成
```shell
gcloud services vpc-peerings connect \
    --service=servicenetworking.googleapis.com \
    --ranges=google-managed-services-default \
    --network=default
```
コンソールにて出来上がったサービス群を確認する

### SQLインスタンスを作成
環境変数を設定
```shell
export INSTANCE_NAME="intern-db-instance"
export REGION="asia-northeast1"
export NETWORK="default"
export PROJECT_ID=$(gcloud config get-value project)
```
SQLインスタンスを作成
```shell
gcloud sql instances create ${INSTANCE_NAME} \
    --database-version=POSTGRES_17 \
    --edition=ENTERPRISE \
    --region=${REGION} \
    --tier=db-g1-small \
    --storage-type=SSD \
    --storage-size=20GB \
    # --database-flags=cloudsql.iam_authentication=on \
    --network=projects/${PROJECT_ID}/global/networks/${NETWORK} \
    --no-assign-ip
```

### データベースの初期設定
データベースを作成
```shell
export DATABASE_NAME="appdb"
gcloud sql databases create ${DATABASE_NAME} \
    --instance=${INSTANCE_NAME}
```
ユーザを作成
```shell
export DB_USER="appuser"
# 本番ではもちろんこんな文字列はNG
export DB_PASS="YourSecurePassword123"
gcloud sql users create ${DB_USER} \
    --instance=${INSTANCE_NAME} \
    --password=${DB_PASS}
```

### 初期データ投入

ソースコードを取得
```shell
git clone https://github.com/mura123yasu/summer-intern-app.git
```
投入データ配置用のGCSバケットを作成
```shell
export REGION="asia-northeast1"
export PROJECT_ID=$(gcloud config get-value project)
export BUCKET_NAME="sql-import-bucket-${PROJECT_ID}"
gsutil mb -l ${REGION} gs://${BUCKET_NAME}
```
※このタイミングで auth login を求められることがある
```shell
gcloud auth login
```
データ投入用SQL2つをGCSにアップロード
```shell
gsutil cp database/01_schema.sql gs://${BUCKET_NAME}/
gsutil cp database/02_data.sql gs://${BUCKET_NAME}/
```
サービスアカウントにオブジェクト参照権限を付与
```shell
export INSTANCE_NAME="intern-db-instance"
export BUCKET_NAME="sql-import-bucket-$(gcloud config get-value project)"
export SA_EMAIL=$(gcloud sql instances describe ${INSTANCE_NAME} --format='value(serviceAccountEmailAddress)')
gsutil iam ch serviceAccount:${SA_EMAIL}:objectViewer gs://${BUCKET_NAME}
```
スキーマを作成
```shell
gcloud sql import sql ${INSTANCE_NAME} gs://${BUCKET_NAME}/01_schema.sql --database=appdb --user=appuser
```
初期データを投入
```shell
gcloud sql import sql ${INSTANCE_NAME} gs://${BUCKET_NAME}/02_data.sql --database=appdb --user=appuser
```
CloudSQL Studioにて投入データが存在することを確認する

## バックエンドの構築

### Cloud Shell 上での動作確認
環境変数を設定
```shell
export INSTANCE_NAME="intern-db-instance"
export DB_NAME="appdb"
export DB_USER="appuser"
export DB_PASS="YourSecurePassword123"
export PROJECT_ID=$(gcloud config get-value project)
export REGION="asia-northeast1"
export INSTANCE_CONNECTION_NAME="${PROJECT_ID}:${REGION}:${INSTANCE_NAME}"
```
goのサーバを起動
```shell
cd backend
go run main.go 
```
Webプレビュー機能を利用し、APIアクセスを実行してみる
（今回の構成ではCloudSQLがPrivateIPアクセスのみ許容する形のためAPI自体は失敗する）

### アプリケーションのコンテナ化
ArtifactRegistryを作成
```shell
 gcloud artifacts repositories create intern-app-repo \
    --repository-format=docker \
    --location=${REGION} \
    --description="Repository for intern app images"
```
先にAPIを有効化しておく
```shell
gcloud services enable cloudbuild.googleapis.com
```
DockerビルドしてPush
```shell
gcloud builds submit --tag ${REGION}-docker.pkg.dev/${PROJECT_ID}/intern-app-repo/backend-service:v1
```
CloudRunがCloudSQLに接続するためのサービスアカウントを作成
```shell
export RUN_SA_NAME="cloudrun-sa"
gcloud iam service-accounts create ${RUN_SA_NAME} \
    --display-name="Cloud Run SA"
```
作成したサービスアカウントにCloudSQLクライアントのロールをアタッチ
```shell
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
    --member="serviceAccount:${RUN_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/cloudsql.client"
```
VPCアクセスコネクタを作成
```shell
export VPC_CONNECTOR_NAME="intern-connector"
gcloud compute networks vpc-access connectors create ${VPC_CONNECTOR_NAME} \
    --region=${REGION} \
    --range=10.8.0.0/28
```
SecretManagerにDBアクセスのパスワードを保管する
```shell
export SECRET_NAME="intern-db-password"
echo -n "${DB_PASS}" | gcloud secrets create ${SECRET_NAME} \
    --data-file=-
```
CloudRunにて利用するサービスアカウントにSecretManagerのアクセス権を付与
```shell
export PROJECT_ID=$(gcloud config get-value project)
gcloud secrets add-iam-policy-binding ${SECRET_NAME} \
    --member="serviceAccount:${RUN_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
```
CloudRunへデプロイ
```shell
export SERVICE_NAME="backend-service"
gcloud run deploy ${SERVICE_NAME} \
    --image ${REGION}-docker.pkg.dev/${PROJECT_ID}/intern-app-repo/backend-service:v1 \
    --platform=managed \
    --region=${REGION} \
    --allow-unauthenticated \
    --service-account=${RUN_SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
    --vpc-connector=${VPC_CONNECTOR_NAME} \
    --set-env-vars="INSTANCE_CONNECTION_NAME=${INSTANCE_CONNECTION_NAME}" \
    --set-env-vars="DB_NAME=${DB_NAME}" \
    --set-env-vars="DB_USER=${DB_USER}" \
    --set-secrets="DB_PASS=${SECRET_NAME}:latest"
```
コンテナのログを確認してみる
```shell
export SERVICE_NAME="backend-service"
export REGION="asia-northeast1"
gcloud beta run services logs tail ${SERVICE_NAME} --region=${REGION}
# おそらく以下の実行を求められる
# sudo apt-get install google-cloud-cli-log-streaming
```
サービスのエンドポイントURLからAPIの疎通確認を実施

CloudRunのサービス設定を「内部+ロードバランシング」へ変更（初期構築時からこの設定を入れることも可能だが、本ハンズオンでは一度APIの動作確認の手順を設けるために後入れとしている）
```shell
 gcloud run services update ${SERVICE_NAME} \
    --region=${REGION} \
    --ingress=internal-and-cloud-load-balancing
```

## フロントエンドの構築
環境変数を設定
```shell
export PROJECT_ID=$(gcloud config get-value project)
export REGION="asia-northeast1"
export FRONTEND_BUCKET_NAME="intern-frontend-bucket-${PROJECT_ID}"
```
本番用モジュールをビルド
```shell
cd frontend
npm install
```
GCSバケットを作成
```shell
gsutil mb -l ${REGION} gs://${FRONTEND_BUCKET_NAME}
```
GCSへモジュールをアップロード
```shell
cd ../
gsutil -m rsync -r frontend/dist gs://${FRONTEND_BUCKET_NAME}
```
トップページのファイルを設定
```shell
gsutil web set -m index.html gs://${FRONTEND_BUCKET_NAME}
```
バケットのオブジェクト参照権限を付与（このハンズオンではバケットを全公開するが本番では非推奨）
```shell
 gsutil iam ch allUsers:objectViewer gs://${FRONTEND_BUCKET_NAME}
```

## ロードバランサーの構築と全体疎通
環境変数の設定
```shell
export PROJECT_ID=$(gcloud config get-value project)
export REGION="asia-northeast1"
export SERVICE_NAME="backend-service"
export FRONTEND_BUCKET_NAME="intern-frontend-bucket-${PROJECT_ID}"
export LB_IP_NAME="intern-lb-ip"
```

### LBと各種サービスを接続する
Network Endpoint Group(NEG)を作成
```shell
 gcloud compute network-endpoint-groups create ${SERVICE_NAME}-neg \
    --region=${REGION} \
    --network-endpoint-type=serverless \
    --cloud-run-service=${SERVICE_NAME}
```
Backend Service(BES)を作成
```shell
gcloud compute backend-services create ${SERVICE_NAME}-bes \
    --load-balancing-scheme=EXTERNAL_MANAGED \
    --global
```
NEGとBESを繋げる
```shell
gcloud compute backend-services add-backend ${SERVICE_NAME}-bes \
    --global \
    --network-endpoint-group=${SERVICE_NAME}-neg \
    --network-endpoint-group-region=${REGION}
```
Backend Bucketを作成(CDNを有効化しつつ)
```shell
gcloud compute backend-buckets create ${FRONTEND_BUCKET_NAME}-beb \
    --gcs-bucket-name=${FRONTEND_BUCKET_NAME} \
    --enable-cdn
```

### LBを作成する
ルーティングルールを作成
```shell
gcloud compute url-maps create intern-app-lb \
    --default-backend-bucket=${FRONTEND_BUCKET_NAME}-beb
gcloud compute url-maps add-path-matcher intern-app-lb \
    --default-backend-bucket=${FRONTEND_BUCKET_NAME}-beb \
    --path-matcher-name=api-matcher \
    --path-rules="/api/*=${SERVICE_NAME}-bes"
```
HTTPターゲットプロキシを作成
```shell
 gcloud compute target-http-proxies create intern-app-http-proxy \
    --url-map=intern-app-lb
```
グローバルIPアドレスを予約
```shell
gcloud compute addresses create ${LB_IP_NAME} --global
```
HTTP転送ルールを作成
```shell
gcloud compute forwarding-rules create intern-app-http-rule \
    --address=${LB_IP} \
    --target-http-proxy=intern-app-http-proxy \
    --global \
    --ports=80
```
エンドポイントURLを確認
```shell
export LB_IP=$(gcloud compute addresses describe ${LB_IP_NAME} --global --format='value(address)')
echo "You can now access your application at http://${LB_IP}"
```
確認したURLにアクセスして、ちゃんと繋がるか確認する（確認の際は、CloudRunのログも出力しておくと良い）

## Terraformによるインフラ変更
まずはinit
```shell
cd ../terraform/backend
terraform init
```

### Terraform import
インポートに使う変数を読み込む
```shell
source <(grep = terraform.tfvars | sed 's/ *= */=/g' | xargs -I {} echo export TF_VAR_{})
```
VPCアクセスコネクタをインポート
```shell
export VPC_CONNECTOR_NAME="intern-connector"
terraform import google_vpc_access_connector.connector "projects/${TF_VAR_project_id}/locations/${TF_VAR_region}/connectors/${VPC_CONNECTOR_NAME}"
```
サービスアカウントをインポート
```shell
export RUN_SA_NAME="cloudrun-sa"
terraform import google_service_account.run_sa "projects/${TF_VAR_project_id}/serviceAccounts/${RUN_SA_NAME}@${TF_VAR_project_id}.iam.gserviceaccount.com"
```
CloudRunをインポート
```shell
export SERVICE_NAME="backend-service"
terraform import google_cloud_run_v2_service.backend_service "projects/${TF_VAR_project_id}/locations/${TF_VAR_region}/services/${SERVICE_NAME}"
```
Planしてみる（差分出るかも。でたらapply）
```shell
terraform plan
```
Terraformのソースコードを修正（envのコメントアウトを外す）してPlan
```shell
terraform plan
```
Plan結果の差分を確認してapply
```shell
terraform apply
```
コンソールにて修正内容が反映されていることを確認する
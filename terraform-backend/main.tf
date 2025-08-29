terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 4.50.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# 1. サーバーレスVPCアクセスコネクタ
resource "google_vpc_access_connector" "connector" {
  name  = "intern-connector"
  ip_cidr_range = "10.8.0.0/28"
  network       = "default"
  region        = var.region
}

# 2. Cloud Run用のサービスアカウント
resource "google_service_account" "run_sa" {
  account_id   = "cloudrun-sa"
  display_name = "Cloud Run SA"
}

# Secret Managerに作成済みのシークレットの情報を参照
data "google_secret_manager_secret" "db_password_secret" {
  secret_id = "intern-db-password"
}

# 3. Cloud Runサービス
resource "google_cloud_run_v2_service" "backend_service" {
  name     = "backend-service"
  location = var.region
  
  template {
    service_account = google_service_account.run_sa.email
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "PRIVATE_RANGES_ONLY"
    }
    containers {
      image = "gcr.io/${var.project_id}/intern-app-backend:v1"
      env {
        name  = "INSTANCE_CONNECTION_NAME"
        value = var.instance_connection_name
      }
      env {
        name  = "DB_NAME"
        value = var.db_name
      }
      env {
        name  = "DB_USER"
        value = var.db_user
      }
      env {
        name = "DB_PASS"
        value_source {
          secret_key_ref {
            secret  = var.db_pass
            version = "latest"
          }
        }
      }
    # 追加時にコメントアウトを外す
    # ここから
    #   env {
    #     name  = "APP_VERSION"
    #     value = "1.0.0-intern-label"
    #   }
    # ここまで
    }
  }
  
  # Ingress設定と認証設定もコードで定義
  ingress = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
  launch_stage = "GA"

  traffic {
    percent         = 100
    type            = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
  }
}
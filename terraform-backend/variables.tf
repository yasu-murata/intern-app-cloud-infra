variable "project_id" {
  description = "The GCP project ID."
  type        = string
}

variable "region" {
  description = "The GCP region for resources."
  type        = string
  default     = "asia-northeast1"
}

variable "instance_connection_name" {
  description = "The connection name of the Cloud SQL instance."
  type        = string
}

variable "db_name" {
  description = "The database name."
  type        = string
}

variable "db_user" {
  description = "The database user."
  type        = string
}

variable "db_pass" {
  description = "The database password."
  type        = string
}
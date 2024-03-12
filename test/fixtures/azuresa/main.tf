terraform {
  required_providers {
    azurerm = {
      version = ">=3.42.0, < 4.0.0"
    }
  }
}

provider "azurerm" {
  # Required for when we disable shared access keys.
  storage_use_azuread = true
  features {}
}

resource "random_string" "deploy_token" {
  length  = 8
  special = false
  upper   = false
  lower   = true
}

data "azurerm_client_config" "current" {}

data "azurerm_resource_group" "rg" {
  name = var.resource_group_name
}

locals {
  deploy_token = random_string.deploy_token.result
  location     = var.location == null ? data.azurerm_resource_group.rg.location : var.location
}

resource "azurerm_storage_account" "sa" {
  name                            = local.deploy_token
  location                        = local.location
  resource_group_name             = data.azurerm_resource_group.rg.name
  account_tier                    = "Standard"
  account_replication_type        = "LRS"
  enable_https_traffic_only       = true
  min_tls_version                 = "TLS1_2"
  shared_access_key_enabled       = true // needed to test both auth styles
  tags                            = var.tags
  account_kind                    = "StorageV2"
  public_network_access_enabled   = true
}

locals {
  sa_roles_for_test_user = [
    "Contributor", "Storage Blob Data Owner"
  ]
}

resource "azurerm_role_assignment" "role_assignment" {
  for_each             = { for i, v in local.sa_roles_for_test_user: v => v}
  scope                = azurerm_storage_account.sa.id
  role_definition_name = each.value
  principal_id         = data.azurerm_client_config.current.object_id
}
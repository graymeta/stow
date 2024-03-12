output "sa_name" {
  value = azurerm_storage_account.sa.name
}

output "primary_sa_key" {
  value     = azurerm_storage_account.sa.primary_access_key
  sensitive = true
}


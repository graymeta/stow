terraform {
  source = ".//."
}

inputs = {
  resource_group_name = get_env("AZ_RG_NAME")
}
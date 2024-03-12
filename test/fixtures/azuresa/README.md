# Fixture: azuresa 

---

This fixture sets up an Azure Storage Account which can be used to test the azure implementation.

## Pre-reqs

#### Workstation
- `tgenv` (or install the version of terragrunt in `.terragrunt-version`)
- `tfenv` (or, install the version of terraform in `.terraform-version`)
- `jq` (needed for instructions below)

#### Azure Account

You will need an identity & resource group to use this module. The identity must have sufficient access
to the resource group to both create resources and assign roles. This is commonly done with either the
`Owner` role or a combination of the `Contributor` role and the `User Access Administrator` role.

## Using it


#### Set up

```shell
export AZ_RG_NAME="YOUR_RG_NAME"
terragrunt apply
```

#### Capturing the fixture values for testing

```shell
export AZUREACCOUNT="$(terragrunt output -json | jq -r '.sa_name.value')"
export AZUREKEY="$(terragrunt output -json | jq -r '.primary_sa_key.value')"
```

#### Running the Azure tests

```shell
go test -v ../../../... -run TestStowWithSharedKeyAuth -count=1
go test -v ../../../... -run TestStowWithDefaultADAuth -count=1
AZUREBIGFILETESTSIZEMB=10 go test -v ../../../... -run TestBigFileUpload -count=1
```

#### Tearing down

```shell
terragrunt destroy
```
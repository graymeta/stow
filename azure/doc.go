/*
Package azure provides an abstraction for the Microsoft Azure Storage service. In this package, an Azure Resource of type "Storage account" is represented by a Stow Location and an Azure blob is represented by a Stow Item.

# Usage and Credentials

stow.Dial requires both a string value of the particular Stow Location Kind ("azure") and a stow.Config instance. The list below outlines all configuration values:

- a key of azure.ConfigAccount with a value of the Azure Resource Name
- a key of azure.ConfigKey with a value of the Azure Access Key (only used with shared key authentication)
- an optional key of azure.ConfigDomainSuffix to specify the Azure API domain. Defaults to `core.windows.net` (public Azure)
- an optional key of azure.ConfigUploadConcurrency to specify the upload concurrency to use. Defaults to `4`.

## Authentication Methods

There are two ways to authenticate to a Microsoft Azure Storage account:

- Shared Key Authentication ([Discouraged by MSFT](https://tinyurl.com/3umn4hpn))
- Azure AD Authentication (requires Role Assignments)

### Shared Key Authentication

To perform shared key authentication, the configuration must include the `azure.ConfigKey` property and the storage account must not prevent the use of shared keys ([MSFT reference here](https://tinyurl.com/3dxkzh99))

### Azure AD Authentication

Azure AD authentication is resolved using the Default credential, which performs a hunt for credentials
in the environment in a cross-SDK and cross-platform way. The documentation for the resolution process
[can be found here](https://tinyurl.com/5ajy83c9).

# Location

There are azure.location methods which allow the retrieval of a single Azure Storage Service. A stow.Item representation of an Azure Storage Blob can also be retrieved based on the Object's URL (ItemByURL).

Additional azure.location methods provide capabilities to create and remove Azure Storage Containers.

# Container

Methods of an azure.container allow the retrieval of an Azure Storage Container's:

- name (ID or Name)
- blob or complete list of blobs (Item or Items)

Additional azure.container methods allow Stow to :

- remove a Blob (RemoveItem)
- update or create a Blob (Put)

# Item

Methods of azure.Item allow the retrieval of an Azure Storage Container's:
- name (ID or name)
- URL
- size in bytes
- Azure Storage blob specific metadata (information stored within the Azure Cloud Service)
- last modified date
- Etag
- content
*/
package azure

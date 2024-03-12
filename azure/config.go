package azure

import (
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"net/url"
	"strconv"

	"github.com/flyteorg/stow"
)

// ConfigAccount should be the name of your storage account in the Azure portal
// ConfigKey should be an access key
// ConfigDomainSuffix the domain suffix to use for storage account communication. The default is the Azure Public cloud
// ConfigUploadConcurrency the upload concurrency to use when uploading. Default is 4.
// ConfigBaseUrlDepreciated Kept for backwards compatability, use ConfigDomainSuffix instead
const (
	ConfigAccount            = "account"
	ConfigKey                = "key"
	ConfigDomainSuffix       = "domain_suffix"
	ConfigUploadConcurrency  = "upload_concurrency"
	ConfigBaseUrlDepreciated = "base_url"
)

// Removed configuration values, will cause failures if used.
const (
	ConfigUseHttpsRemoved   = "use_https"
	ConfigApiVersionRemoved = "api_version"
)

var removedConfigKeys = []string{ConfigUseHttpsRemoved, ConfigApiVersionRemoved}

// Kind is the kind of Location this package provides.
const Kind = "azure"

// defaultDomainSuffix is the domain suffix for the Azure Public Cloud
const defaultDomainSuffix = "core.windows.net"

// defaultUploadConcurrency is the default upload concurrency
const defaultUploadConcurrency = 4

func init() {
	validatefn := func(config stow.Config) error {
		_, ok := config.Config(ConfigAccount)
		if !ok {
			return errors.New("missing account id")
		}
		for _, removedConfigKey := range removedConfigKeys {
			_, ok = config.Config(removedConfigKey)
			if ok {
				return fmt.Errorf("removed config option used [%s]", removedConfigKey)
			}
		}
		return nil
	}
	makefn := func(config stow.Config) (stow.Location, error) {
		acctName, ok := config.Config(ConfigAccount)
		if !ok {
			return nil, errors.New("missing account id")
		}

		var uploadConcurrency int
		var err error
		uploadConcurrencyStr, ok := config.Config(ConfigUploadConcurrency)
		if !ok || len(uploadConcurrencyStr) == 0 {
			uploadConcurrency = defaultUploadConcurrency
		} else {
			uploadConcurrency, err = strconv.Atoi(uploadConcurrencyStr)
			if err != nil {
				return nil, fmt.Errorf("invalid upload concurrency [%v]", uploadConcurrency)
			}
		}
		l := &location{
			accountName:       acctName,
			uploadConcurrency: uploadConcurrency,
		}

		l.client, l.preSigner, err = makeAccountClient(config)
		if err != nil {
			return nil, err
		}

		// test the connection
		_, _, err = l.Containers("", stow.CursorStart, 1)
		if err != nil {
			return nil, err
		}

		return l, nil
	}
	kindfn := func(u *url.URL) bool {
		return u.Scheme == Kind
	}
	stow.Register(Kind, makefn, kindfn, validatefn)
}

// makeAccountClient is a factory function for producing client instances
func makeAccountClient(cfg stow.Config) (*azblob.Client, RequestPreSigner, error) {
	accountName, ok := cfg.Config(ConfigAccount)
	if !ok {
		return nil, nil, errors.New("missing account id")
	}

	domainSuffix := resolveAzureDomainSuffix(cfg)
	serviceUrl := fmt.Sprintf("https://%s.blob.%s", accountName, domainSuffix)

	key, ok := cfg.Config(ConfigKey)
	if ok && key != "" {
		return newSharedKeyClient(accountName, key, serviceUrl)
	}
	return newDefaultAzureIdentityClient(serviceUrl)
}

// newSharedKeyClient creates client objects for working with a storage account
// using shared keys.
func newSharedKeyClient(accountName, key, serviceUrl string) (*azblob.Client, RequestPreSigner, error) {
	sharedKeyCred, err := azblob.NewSharedKeyCredential(accountName, key)
	if err != nil {
		return nil, nil, err
	}
	client, err := azblob.NewClientWithSharedKeyCredential(
		serviceUrl,
		sharedKeyCred,
		nil)
	if err != nil {
		return nil, nil, err
	}
	preSigner, err := NewSharedKeyRequestPreSigner(accountName, key)
	if err != nil {
		return nil, nil, err
	}
	return client, preSigner, nil
}

// newDefaultAzureIdentityClient creates client objects for working with a storage
// account using Azure AD auth, resolved using the default Azure credential chain.
func newDefaultAzureIdentityClient(serviceUrl string) (*azblob.Client, RequestPreSigner, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, nil, err
	}
	client, err := azblob.NewClient(serviceUrl, cred, nil)
	if err != nil {
		return nil, nil, err
	}
	preSigner, err := NewDelegatedKeyPreSigner(client.ServiceClient())
	return client, preSigner, nil
}

// resolveAzureDomainSuffix returns the Azure domain suffix to use
func resolveAzureDomainSuffix(cfg stow.Config) string {
	domainSuffix, ok := cfg.Config(ConfigDomainSuffix)
	if ok && domainSuffix != "" {
		return domainSuffix
	}

	domainSuffix, ok = cfg.Config(ConfigBaseUrlDepreciated)
	if ok && domainSuffix != "" {
		return domainSuffix
	}
	return defaultDomainSuffix
}

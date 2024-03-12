package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/pkg/errors"
	"time"
)

// BlobProps are the only blob properties required for stow.
type BlobProps struct {
	ETag          azcore.ETag
	LastModified  time.Time
	ContentLength int64
}

// RequestPreSigner is a facade for pre-signing blob requests, regardless of
// how authentication was performed.
type RequestPreSigner func(ctx context.Context, values sas.BlobSignatureValues) (sas.QueryParameters, error)

// NewSharedKeyRequestPreSigner will create a RequestPreSigner when a shared key
// is used as the authentication method.
func NewSharedKeyRequestPreSigner(accountName, key string) (RequestPreSigner, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, key)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, values sas.BlobSignatureValues) (sas.QueryParameters, error) {
		return values.SignWithSharedKey(cred)
	}, nil
}

// Azure recommends a 15 minute buffer for SAS tokens (and most tokens) in order to account
// for clock skew between their systems and clients. From their docs...
//
// > remember that you may observe up to 15 minutes of clock skew in either direction
// > on any request
//
// https://learn.microsoft.com/en-us/azure/storage/common/storage-sas-overview
var clockSkewBuffer = time.Minute * 15

// NewDelegatedKeyPreSigner will create a RequestPreSigner that worked with delegated
// credentials, necessary when identity-based authentication (AD auth) is the used with
// the SDK.
func NewDelegatedKeyPreSigner(serviceClient *service.Client) (RequestPreSigner, error) {
	return func(ctx context.Context, values sas.BlobSignatureValues) (sas.QueryParameters, error) {
		// Create the delegate key with a time buffer, since the blob key's lifetime
		// must fit within the delegate key's lifetime.
		delegateCredsStartTime := values.StartTime.UTC().Add(-1 * clockSkewBuffer)
		delegateCredsEndTime := values.ExpiryTime.UTC().Add(clockSkewBuffer)

		udc, err := serviceClient.GetUserDelegationCredential(
			ctx,
			service.KeyInfo{
				Start:  to.Ptr(delegateCredsStartTime.Format(sas.TimeFormat)),
				Expiry: to.Ptr(delegateCredsEndTime.Format(sas.TimeFormat)),
			},
			nil)

		if err != nil {
			return sas.QueryParameters{}, err
		}
		return values.SignWithUserDelegation(udc)
	}, nil
}

// makeAzureCompatMetadataMap converts a stow-style metadata map into an Azure compatible
// metadata map.
func makeAzureCompatMetadataMap(md map[string]interface{}) (map[string]*string, error) {
	azcompatMap := make(map[string]*string, len(md))
	for k, v := range md {
		vStr, ok := v.(string)
		if ok {
			azcompatMap[k] = &vStr
		} else {
			// TODO: This is debatable. A fmt.Sprintf("%v", v) would be more flexible. However
			//    this is inline with the previous behavior
			return nil, errors.Errorf(`value of key '%s' in metadata must be of type string`, k)
		}
	}
	return azcompatMap, nil
}

// makeStowCompatMetadataMap converts an Azure SDK metadata map into one that is stow
// compatible.
func makeStowCompatMetadataMap(azureMap map[string]*string) map[string]interface{} {
	stowMap := make(map[string]interface{}, len(azureMap))
	for k, v := range azureMap {
		stowMap[k] = *v
	}
	return stowMap
}

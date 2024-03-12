package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	azcontainer "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/flyteorg/stow"
	"github.com/pkg/errors"
)

// timeFormat is the time format for azure.
var timeFormat = "Mon, 2 Jan 2006 15:04:05 MST"

type container struct {
	id                string
	properties        *BlobProps
	client            *azcontainer.Client
	preSigner         RequestPreSigner
	uploadConcurrency int
}

var _ stow.Container = (*container)(nil)

func (c *container) ID() string {
	return c.id
}

func (c *container) Name() string {
	return c.id
}

func (c *container) PreSignRequest(ctx context.Context, method stow.ClientMethod, key string,
	params stow.PresignRequestParams) (response stow.PresignResponse, err error) {
	containerName := c.id
	blobName := key
	var requestHeaders map[string]string
	permissions := sas.BlobPermissions{}
	switch method {
	case stow.ClientMethodGet:
		permissions.Read = true
	case stow.ClientMethodPut:
		permissions.Add = true
		permissions.Write = true

		requestHeaders = map[string]string{"Content-Length": strconv.Itoa(len(params.ContentMD5)), "Content-MD5": params.ContentMD5}
		requestHeaders["x-ms-blob-type"] = "BlockBlob" // https://learn.microsoft.com/en-us/rest/api/storageservices/put-blob?tabs=microsoft-entra-id#remarks

		if params.AddContentMD5Metadata {
			requestHeaders[fmt.Sprintf("x-ms-meta-%s", stow.FlyteContentMD5)] = params.ContentMD5
		}
	}

	sasQueryParams, err := c.preSigner(ctx, sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().UTC().Add(-1 * clockSkewBuffer),
		ExpiryTime:    time.Now().UTC().Add(params.ExpiresIn + clockSkewBuffer),
		ContainerName: containerName,
		BlobName:      blobName,
		Permissions:   permissions.String(),
	})

	if err != nil {
		return stow.PresignResponse{}, err
	}

	// Create the SAS URL for the resource you wish to access, and append the SAS query parameters.
	qp := sasQueryParams.Encode()

	return stow.PresignResponse{Url: fmt.Sprintf("%s/%s?%s", c.client.URL(), blobName, qp), RequiredRequestHeaders: requestHeaders}, nil
}

func (c *container) Item(id string) (stow.Item, error) {
	cleanedId := strings.Replace(id, " ", "+", -1)
	items, _, err := c.Items(cleanedId, "", 1)
	if err != nil {
		return nil, err
	} else if len(items) == 0 {
		return nil, stow.ErrNotFound
	} else {
		return items[0], nil
	}

	// Why not use this code? Because the Azure SDK/API will return metadata with the
	// first character of each key upper-cased. Unfortunately, the MSFT position is
	// that this is a golang upstream problem and they are not attempting to fix it.
	//
	// See: https://github.com/Azure/azure-sdk-for-go/issues/16791#issuecomment-1011518946
	//
	// The workaround used here is to just use their listing API, which doesn't rely on
	// retrieving metadata as HTTP headers.
	//
	// Another reasonable alternative would be to just lower case all metadata keys, as is
	// the case with s3: https://github.com/aws/aws-sdk-go/issues/445
	//
	//ctx := context.Background()
	//blobClient := c.client.NewBlobClient(id)
	//resp, err := blobClient.GetProperties(ctx, nil)
	//if err != nil {
	//	if strings.Contains(err.Error(), "404") {
	//		return nil, stow.ErrNotFound
	//	}
	//	return nil, err
	//}
	//item := &item{
	//	id:        id,
	//	container: c,
	//	client:    blobClient,
	//	metadata:  makeStowCompatMetadataMap(resp.Metadata),
	//	properties: &BlobProps{
	//		ETag:          *resp.ETag,
	//		LastModified:  *resp.LastModified,
	//		ContentLength: *resp.ContentLength,
	//	},
	//}
	//
	//return item, nil
}

func (c *container) Items(prefix, cursor string, count int) ([]stow.Item, string, error) {
	ctx := context.Background()
	options := azcontainer.ListBlobsFlatOptions{
		Prefix:     &prefix,
		MaxResults: to.Ptr(int32(count)),
		Include:    azcontainer.ListBlobsInclude{Metadata: true},
	}
	if cursor != "" {
		options.Marker = &cursor
	}

	listResp, err := c.client.NewListBlobsFlatPager(&options).NextPage(ctx)
	if err != nil {
		return nil, "", err
	}
	items := make([]stow.Item, len(listResp.Segment.BlobItems))
	for i, blob := range listResp.Segment.BlobItems {
		items[i] = &item{
			id:        *blob.Name,
			container: c,
			client:    c.client.NewBlobClient(*blob.Name),
			metadata:  makeStowCompatMetadataMap(blob.Metadata),
			properties: &BlobProps{
				ETag:          *blob.Properties.ETag,
				LastModified:  *blob.Properties.LastModified,
				ContentLength: *blob.Properties.ContentLength,
			},
		}
	}
	return items, *listResp.NextMarker, nil
}

func (c *container) Put(name string, r io.Reader, size int64, metadata map[string]interface{}) (stow.Item, error) {
	ctx := context.Background()
	mdParsed, err := makeAzureCompatMetadataMap(metadata)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create or update Item, preparing metadata")
	}

	name = strings.Replace(name, " ", "+", -1)
	blockClient := c.client.NewBlockBlobClient(name)
	var blobProps = &BlobProps{ContentLength: size}
	f, match := r.(*os.File)
	if match {
		resp, err := blockClient.UploadFile(ctx, f, &blockblob.UploadFileOptions{
			Concurrency: uint16(c.uploadConcurrency),
			Metadata:    mdParsed,
		})
		if err != nil {
			return nil, errors.Wrap(err, "file upload")
		}
		blobProps.ETag = *resp.ETag
		blobProps.LastModified = *resp.LastModified
	} else {
		resp, err := blockClient.UploadStream(ctx, r, &blockblob.UploadStreamOptions{
			Concurrency: c.uploadConcurrency,
			Metadata:    mdParsed,
		})
		if err != nil {
			return nil, errors.Wrap(err, "stream upload")
		}
		blobProps.ETag = *resp.ETag
		blobProps.LastModified = *resp.LastModified
	}

	item := &item{
		id:         name,
		container:  c,
		metadata:   metadata,
		client:     c.client.NewBlobClient(name),
		properties: blobProps,
	}
	return item, nil
}

func (c *container) SetItemMetadata(itemName string, md map[string]string) error {
	ctx := context.Background()
	azCompatMap := make(map[string]*string, len(md))
	for k, v := range md {
		azCompatMap[k] = &v
	}
	_, err := c.client.NewBlobClient(itemName).SetMetadata(
		ctx, azCompatMap, nil)
	return err
}

func (c *container) RemoveItem(id string) error {
	ctx := context.Background()
	_, err := c.client.NewBlobClient(id).Delete(ctx, nil)
	return err
}

// Remove quotation marks from beginning and end. This includes quotations that
// are escaped. Also removes leading `W/` from prefix for weak Etags.
//
// Based on the Etag spec, the full etag value (<FULL ETAG VALUE>) can include:
// - W/"<ETAG VALUE>"
// - "<ETAG VALUE>"
// - ""
// Source: https://tools.ietf.org/html/rfc7232#section-2.3
//
// Based on HTTP spec, forward slash is a separator and must be enclosed in
// quotes to be used as a valid value. Hence, the returned value may include:
// - "<FULL ETAG VALUE>"
// - \"<FULL ETAG VALUE>\"
// Source: https://www.w3.org/Protocols/rfc2616/rfc2616-sec2.html#sec2.2
//
// This function contains a loop to check for the presence of the three possible
// filler characters and strips them, resulting in only the Etag value.
func cleanEtag(etag string) string {
	for {
		// Check if the filler characters are present
		if strings.HasPrefix(etag, `\"`) {
			etag = strings.Trim(etag, `\"`)

		} else if strings.HasPrefix(etag, `"`) {
			etag = strings.Trim(etag, `"`)

		} else if strings.HasPrefix(etag, `W/`) {
			etag = strings.Replace(etag, `W/`, "", 1)

		} else {

			break
		}
	}

	return etag
}

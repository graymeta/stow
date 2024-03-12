package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/flyteorg/stow"
)

type item struct {
	id         string
	container  *container
	client     *blob.Client
	properties *BlobProps
	url        url.URL
	metadata   map[string]interface{}
	infoOnce   sync.Once
	infoErr    error
}

var (
	_ stow.Item       = (*item)(nil)
	_ stow.ItemRanger = (*item)(nil)
)

func (i *item) ID() string {
	return i.id
}

func (i *item) Name() string {
	return i.id
}

func (i *item) URL() *url.URL {
	url, _ := url.Parse(i.client.URL())
	url.Scheme = "azure"
	return url
}

func (i *item) Size() (int64, error) {
	return i.properties.ContentLength, nil
}

func (i *item) Open() (io.ReadCloser, error) {
	ctx := context.Background()
	dlResp, err := i.client.DownloadStream(ctx, nil)
	if err != nil {
		return nil, err
	}
	return dlResp.Body, nil
}

func (i *item) ETag() (string, error) {
	return cleanEtag(string(i.properties.ETag)), nil
}

func (i *item) LastMod() (time.Time, error) {
	return i.properties.LastModified, nil
}

func (i *item) Metadata() (map[string]interface{}, error) {
	return i.metadata, nil
}

// OpenRange opens the item for reading starting at byte start and ending
// at byte end.
func (i *item) OpenRange(start, end uint64) (io.ReadCloser, error) {
	ctx := context.Background()
	resp, err := i.client.DownloadStream(ctx, &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{
			Offset: int64(start),
			Count:  int64(end) - int64(start) + 1,
		},
	})

	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

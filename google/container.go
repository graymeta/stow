package google

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"

	"github.com/flyteorg/stow"
)

const googleMetadataPrefix = "x-goog-meta-"

type Container struct {
	// Name is needed to retrieve items.
	name string

	// Client is responsible for performing the requests.
	client *storage.Client

	// ctx is used on google storage API calls
	ctx context.Context
}

// ID returns a string value which represents the name of the container.
func (c *Container) ID() string {
	return c.name
}

// Name returns a string value which represents the name of the container.
func (c *Container) Name() string {
	return c.name
}

// Bucket returns the google bucket attributes
func (c *Container) Bucket() *storage.BucketHandle {
	return c.client.Bucket(c.name)
}

func (c *Container) PreSignRequest(_ context.Context, clientMethod stow.ClientMethod, id string,
	params stow.PresignRequestParams) (response stow.PresignResponse, err error) {
	headers := make([]string, 0, 3)
	var requestHeaders map[string]string
	if len(params.HttpMethod) == 0 {
		switch clientMethod {
		case stow.ClientMethodGet:
			params.HttpMethod = http.MethodGet
		case stow.ClientMethodPut:
			params.HttpMethod = http.MethodPut
			requestHeaders = map[string]string{"Content-Length": strconv.Itoa(len(params.ContentMD5)), "Content-MD5": params.ContentMD5}
			if params.AddContentMD5Metadata {
				headers = append(headers, fmt.Sprintf("%s%s: %s", googleMetadataPrefix, stow.FlyteContentMD5, params.ContentMD5))
				requestHeaders[fmt.Sprintf("%s%s", googleMetadataPrefix, stow.FlyteContentMD5)] = params.ContentMD5
			}
		}
	}

	url, error := c.Bucket().SignedURL(id, &storage.SignedURLOptions{
		Method:  params.HttpMethod,
		Expires: time.Now().Add(params.ExpiresIn),
		MD5:     params.ContentMD5,
		Headers: headers,
	})

	return stow.PresignResponse{Url: url, RequiredRequestHeaders: requestHeaders}, error
}

// Item returns a stow.Item instance of a container based on the
// name of the container
func (c *Container) Item(id string) (stow.Item, error) {
	item, err := c.Bucket().Object(id).Attrs(c.ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, stow.ErrNotFound
		}
		return nil, err
	}

	return c.convertToStowItem(item)
}

// Items retrieves a list of items that are prepended with
// the prefix argument. The 'cursor' variable facilitates pagination.
func (c *Container) Items(prefix string, cursor string, count int) ([]stow.Item, string, error) {
	query := &storage.Query{Prefix: prefix}
	call := c.Bucket().Objects(c.ctx, query)

	p := iterator.NewPager(call, count, cursor)
	var results []*storage.ObjectAttrs
	nextPageToken, err := p.NextPage(&results)
	if err != nil {
		return nil, "", err
	}

	var items []stow.Item
	for _, item := range results {
		i, err := c.convertToStowItem(item)
		if err != nil {
			return nil, "", err
		}

		items = append(items, i)
	}

	return items, nextPageToken, nil
}

// RemoveItem will delete a google storage Object
func (c *Container) RemoveItem(id string) error {
	return c.Bucket().Object(id).Delete(c.ctx)
}

// Put sends a request to upload content to the container. The arguments
// received are the name of the item, a reader representing the
// content, and the size of the file.
func (c *Container) Put(name string, r io.Reader, size int64, metadata map[string]interface{}) (stow.Item, error) {
	obj := c.Bucket().Object(name)

	mdPrepped, err := prepMetadata(metadata)
	if err != nil {
		return nil, err
	}

	w := obj.NewWriter(c.ctx)
	w.ObjectAttrs.Metadata = merge(w.ObjectAttrs.Metadata, mdPrepped)
	if _, err := io.Copy(w, r); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}

	return c.convertToStowItem(w.Attrs())
}

func merge(metadata ...map[string]string) map[string]string {
	res := map[string]string{}
	for _, mt := range metadata {
		for k, v := range mt {
			res[k] = v
		}
	}
	return res
}

func (c *Container) convertToStowItem(attr *storage.ObjectAttrs) (stow.Item, error) {
	u, err := prepUrl(attr.MediaLink)
	if err != nil {
		return nil, err
	}

	mdParsed, err := parseMetadata(attr.Metadata)
	if err != nil {
		return nil, err
	}

	return &Item{
		name:         attr.Name,
		container:    c,
		client:       c.client,
		size:         attr.Size,
		etag:         attr.Etag,
		hash:         string(attr.MD5),
		lastModified: attr.Updated,
		url:          u,
		metadata:     mdParsed,
		object:       attr,
		ctx:          c.ctx,
	}, nil
}

func parseMetadata(metadataParsed map[string]string) (map[string]interface{}, error) {
	metadataParsedMap := make(map[string]interface{}, len(metadataParsed))
	for key, value := range metadataParsed {
		metadataParsedMap[key] = value
	}
	return metadataParsedMap, nil
}

func prepMetadata(metadataParsed map[string]interface{}) (map[string]string, error) {
	returnMap := make(map[string]string, len(metadataParsed))
	for key, value := range metadataParsed {
		str, ok := value.(string)
		if !ok {
			return nil, errors.Errorf(`value of key '%s' in metadata must be of type string`, key)
		}
		returnMap[key] = str
	}
	return returnMap, nil
}

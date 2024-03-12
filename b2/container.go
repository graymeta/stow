package b2

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/flyteorg/stow"
	"github.com/pkg/errors"
	"gopkg.in/kothar/go-backblaze.v0"
)

type container struct {
	bucket *backblaze.Bucket
}

var _ stow.Container = (*container)(nil)

// ID returns the name of a bucket
func (c *container) ID() string {
	// Although backblaze does give an ID for buckets, some operations deal with bucket
	// names instead of the ID (specifically the B2.Bucket method). For that reason,
	// return name instead of ID. We can still use the id field internally when necessary
	return c.bucket.Name
}

func (c *container) PreSignRequest(_ context.Context, _ stow.ClientMethod, _ string,
	_ stow.PresignRequestParams) (url stow.PresignResponse, err error) {
	return stow.PresignResponse{}, fmt.Errorf("unsupported")
}

// Name returns the name of the bucket
func (c *container) Name() string {
	return c.bucket.Name
}

// Item returns a stow.Item given the item's ID
func (c *container) Item(id string) (stow.Item, error) {
	return c.getItem(id)
}

// Items retreives a list of items from b2. Since the b2 ListFileNames operation
// does not natively support a prefix, we fake it ourselves
func (c *container) Items(prefix, cursor string, count int) ([]stow.Item, string, error) {
	items := make([]stow.Item, 0, count)
	for {
		response, err := c.bucket.ListFileNames(cursor, count)
		if err != nil {
			return nil, "", err
		}

		for _, obj := range response.Files {
			if prefix != stow.NoPrefix && !strings.HasPrefix(obj.Name, prefix) {
				continue
			}
			items = append(items, &item{
				id:           obj.ID,
				name:         obj.Name,
				size:         int64(obj.Size),
				lastModified: time.Unix(obj.UploadTimestamp/1000, 0),
				bucket:       c.bucket,
			})
			if len(items) == count {
				break
			}
		}

		cursor = response.NextFileName

		if prefix == "" || cursor == "" {
			return items, cursor, nil
		}

		if len(items) == count {
			break
		}

		if !strings.HasPrefix(cursor, prefix) {
			return items, "", nil
		}
	}

	if cursor != "" && cursor != items[len(items)-1].Name() {
		// append a space because that's a funny quirk of backblaze's implementation
		cursor = items[len(items)-1].Name() + " "
	}

	return items, cursor, nil
}

// Put uploads a file
func (c *container) Put(name string, r io.Reader, size int64, metadata map[string]interface{}) (stow.Item, error) {
	// Convert map[string]interface{} to map[string]string
	mdPrepped, err := prepMetadata(metadata)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create or update item, preparing metadata")
	}

	file, err := c.bucket.UploadFile(name, mdPrepped, r)
	if err != nil {
		return nil, err
	}

	return &item{
		id:     file.ID,
		name:   file.Name,
		size:   file.ContentLength,
		bucket: c.bucket,
	}, nil
}

// RemoveItem identifies the file by it's ID, then removes all versions of that file
func (c *container) RemoveItem(id string) error {
	item, err := c.getItem(id)
	if err != nil {
		return err
	}

	// files can have multiple versions in backblaze. You have to delete
	// files one version at a time.
	for {
		response, err := item.bucket.ListFileNames(item.Name(), 1)
		if err != nil {
			return err
		}

		var fileStatus *backblaze.FileStatus
		for i := range response.Files {
			if response.Files[i].Name == item.Name() {
				fileStatus = &response.Files[i]
				break
			}
		}
		if fileStatus == nil {
			// we've deleted all versions of the file
			return nil
		}

		if _, err := c.bucket.DeleteFileVersion(item.name, response.Files[0].ID); err != nil {
			return err
		}
	}
}

func (c *container) getItem(id string) (*item, error) {
	file, err := c.bucket.GetFileInfo(id)
	if err != nil {
		lowered := strings.ToLower(err.Error())
		if (strings.Contains(lowered, "not") && strings.Contains(lowered, "found")) || (strings.Contains(lowered, "bad") && strings.Contains(lowered, "fileid")) {
			return nil, stow.ErrNotFound
		}
		return nil, err
	}

	return &item{
		id:     file.ID,
		name:   file.Name,
		size:   file.ContentLength,
		bucket: c.bucket,
	}, nil
}

// prepMetadata parses a raw map into the native type required by b2 to set metadata (map[string]string).
// This function also assumes that the value of a key value pair is a string.
func prepMetadata(md map[string]interface{}) (map[string]string, error) {
	m := make(map[string]string, len(md))
	for key, value := range md {
		strValue, valid := value.(string)
		if !valid {
			return nil, errors.Errorf(`value of key '%s' in metadata must be of type string`, key)
		}
		m[key] = strValue
	}
	return m, nil
}

// parseMetadata transforms a map[string]string to a map[string]interface{}
func parseMetadata(md map[string]string) map[string]interface{} {
	m := make(map[string]interface{}, len(md))
	for key, value := range md {
		m[key] = value
	}
	return m
}

package b2

import (
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/flyteorg/stow"

	"github.com/pkg/errors"
	"gopkg.in/kothar/go-backblaze.v0"
)

type item struct {
	id           string
	name         string
	size         int64
	lastModified time.Time
	bucket       *backblaze.Bucket

	metadata map[string]interface{}
	infoOnce sync.Once
	infoErr  error
}

var (
	_ stow.Item       = (*item)(nil)
	_ stow.ItemRanger = (*item)(nil)
)

// ID returns this item's ID
func (i *item) ID() string {
	return i.id
}

// Name returns this item's name
func (i *item) Name() string {
	return i.name
}

// URL returns the stow url for this item
func (i *item) URL() *url.URL {
	str, err := i.bucket.FileURL(i.name)
	if err != nil {
		return nil
	}

	url, _ := url.Parse(str)
	url.Scheme = Kind

	return url
}

// Metadata returns additional item metadata fields that were set when the file was uploaded
func (i *item) Metadata() (map[string]interface{}, error) {
	if err := i.ensureInfo(); err != nil {
		return nil, errors.Wrap(err, "retrieving item metadata")
	}
	return i.metadata, nil
}

// size returns the file's size, in bytes
func (i *item) Size() (int64, error) {
	return i.size, nil
}

// Open downloads the item
func (i *item) Open() (io.ReadCloser, error) {
	_, r, err := i.bucket.DownloadFileByName(i.name)
	return r, err
}

// OpenRange opens the item for reading starting at byte start and ending
// at byte end.
func (i *item) OpenRange(start, end uint64) (io.ReadCloser, error) {
	_, r, err := i.bucket.DownloadFileRangeByName(
		i.name,
		&backblaze.FileRange{Start: int64(start), End: int64(end)},
	)
	return r, err
}

// ETag returns an etag for an item. In this implementation we use the file's last modified timestamp
func (i *item) ETag() (string, error) {
	if err := i.ensureInfo(); err != nil {
		return "", errors.Wrap(err, "retreiving etag")
	}
	return i.lastModified.String(), nil
}

// LastMod returns the file's last modified timestamp
func (i *item) LastMod() (time.Time, error) {
	if err := i.ensureInfo(); err != nil {
		return time.Time{}, errors.Wrap(err, "retrieving Last Modified information of Item")
	}
	return i.lastModified, nil
}

func (i *item) ensureInfo() error {
	if i.metadata == nil || i.lastModified.IsZero() {
		i.infoOnce.Do(func() {
			f, err := i.bucket.GetFileInfo(i.id)
			if err != nil {
				i.infoErr = err
				return
			}

			i.lastModified = time.Unix(f.UploadTimestamp/1000, 0)
			i.metadata = parseMetadata(f.FileInfo)
		})
	}
	return i.infoErr
}

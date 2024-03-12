package sftp

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/flyteorg/stow/local"
)

type item struct {
	container *container
	path      string
	size      int64
	modTime   time.Time
	md        map[string]interface{}
}

// ID returns a string value that represents the name of a file.
func (i *item) ID() string {
	return i.path
}

// Name returns a string value that represents the name of the file.
func (i *item) Name() string {
	return i.path
}

// Size returns the size of an item in bytes.
func (i *item) Size() (int64, error) {
	return i.size, nil
}

// URL returns a formatted string identifying this asset.
// Format is: sftp://<username>@<host>:<port>/<container>/<path to file>
func (i *item) URL() *url.URL {
	genericURL := fmt.Sprintf("/%s/%s", i.container.Name(), i.Name())
	return &url.URL{
		Scheme: Kind,
		User:   url.User(i.container.location.config.sshConfig.User),
		Path:   genericURL,
		Host:   i.container.location.config.Host(),
	}
}

// Open retrieves specic information about an item based on the container name
// and path of the file within the container. This response includes the body of
// resource which is returned along with an error.
func (i *item) Open() (io.ReadCloser, error) {
	return i.container.location.sftpClient.Open(
		filepath.Join(
			i.container.location.config.basePath,
			i.container.Name(),
			i.Name(),
		),
	)
}

// LastMod returns the last modified date of the item.
func (i *item) LastMod() (time.Time, error) {
	return i.modTime, nil
}

// ETag returns the ETag value from the properies field of an item.
func (i *item) ETag() (string, error) {
	return i.modTime.String(), nil
}

// Metadata returns some item level metadata about the item.
func (i *item) Metadata() (map[string]interface{}, error) {
	return i.md, nil
}

func getFileMetadata(info os.FileInfo) map[string]interface{} {
	return map[string]interface{}{
		// Reuse the constants from the local package for consistency.
		local.MetadataIsDir: info.IsDir(),
		local.MetadataName:  info.Name(),
		local.MetadataMode:  fmt.Sprintf("%o", info.Mode()),
		local.MetadataModeD: fmt.Sprintf("%v", uint32(info.Mode())),
		local.MetadataPerm:  info.Mode().String(),
		local.MetadataSize:  info.Size(),
	}
}

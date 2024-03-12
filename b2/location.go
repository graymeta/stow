package b2

import (
	"errors"
	"net/url"
	"strings"

	"github.com/flyteorg/stow"
	"gopkg.in/kothar/go-backblaze.v0"
)

type location struct {
	config stow.Config
	client *backblaze.B2
}

// Close closes the interface. It's a Noop for this B2 implementation
func (l *location) Close() error {
	return nil // nothing to close
}

// CreateContainer creates a new container (bucket)
func (l *location) CreateContainer(name string) (stow.Container, error) {
	bucket, err := l.client.CreateBucket(name, backblaze.AllPrivate)
	if err != nil {
		return nil, err
	}
	return &container{
		bucket: bucket,
	}, nil
}

// Containers lists all containers in the location
func (l *location) Containers(prefix string, cursor string, count int) ([]stow.Container, string, error) {
	response, err := l.client.ListBuckets()
	if err != nil {
		return nil, "", err
	}

	containers := make([]stow.Container, 0, len(response))
	for _, cont := range response {
		// api and/or library don't seem to support prefixes, so do it ourself
		if strings.HasPrefix(cont.Name, prefix) {
			containers = append(containers, &container{
				bucket: cont,
			})
		}
	}

	return containers, "", nil
}

// Container returns a stow.Contaner given a container id. In this case, the 'id'
// is really the bucket name
func (l *location) Container(id string) (stow.Container, error) {
	bucket, err := l.client.Bucket(id)
	if err != nil || bucket == nil {
		return nil, stow.ErrNotFound
	}

	return &container{
		bucket: bucket,
	}, nil
}

// ItemByURL returns a stow.Item given a b2 stow url
func (l *location) ItemByURL(u *url.URL) (stow.Item, error) {
	if u.Scheme != Kind {
		return nil, errors.New("not valid b2 URL")
	}

	// b2://f001.backblaze.com/file/<container_name>/<path_to_object>
	pieces := strings.SplitN(u.Path, "/", 4)

	c, err := l.Container(pieces[2])
	if err != nil {
		return nil, err
	}

	filename, err := url.QueryUnescape(pieces[3])
	if err != nil {
		return nil, err
	}
	response, err := c.(*container).bucket.ListFileNames(filename, 1)
	if err != nil {
		return nil, stow.ErrNotFound
	}

	if len(response.Files) != 1 {
		return nil, errors.New("unexpected number of responses from ListFileNames")
	}

	return c.Item(response.Files[0].ID)
}

// RemoveContainer removes the specified bucket. In this case, the 'id'
// is really the bucket name
func (l *location) RemoveContainer(id string) error {
	stowCont, err := l.Container(id)
	if err != nil {
		return err
	}

	return stowCont.(*container).bucket.Delete()
}

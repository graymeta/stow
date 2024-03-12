package sftp

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flyteorg/stow"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type location struct {
	// we keep config here so that we can access the server information (username/host)
	// when constructing the item urls.
	config     *conf
	sshClient  *ssh.Client
	sftpClient *sftp.Client
}

// CreateContainer creates a new container, in this case a directory on the remote server.
func (l *location) CreateContainer(containerName string) (stow.Container, error) {
	if err := l.sftpClient.Mkdir(filepath.Join(l.config.basePath, containerName)); err != nil {
		return nil, err
	}

	return &container{
		location: l,
		name:     containerName,
	}, nil
}

// Containers returns a slice of the Container interface, a cursor, and an error.
func (l *location) Containers(prefix, cursor string, count int) ([]stow.Container, string, error) {
	infos, err := l.sftpClient.ReadDir(l.config.basePath)
	if err != nil {
		return nil, "", err
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name() < infos[j].Name() })

	var cont []stow.Container
	for i, v := range infos {
		if !v.IsDir() {
			continue
		}

		if !strings.HasPrefix(v.Name(), prefix) {
			continue
		}

		if v.Name() <= cursor {
			continue
		}

		cont = append(cont, &container{
			location: l,
			name:     v.Name(),
		})

		if len(cont) == count {
			// Check if any of the remaining entries are directories. If they are, then return
			// v.Name() as the cursor. This prevents us from serving up an empty page of
			// containers if all of the entries after v.Name() are files.
			for j := i + 1; j < len(infos); j++ {
				if infos[j].IsDir() {
					return cont, v.Name(), nil
				}
			}
			return cont, "", nil
		}
	}

	return cont, "", nil
}

// Close closes the underlying sftp/ssh connections.
func (l *location) Close() error {
	var errs error

	if l.sftpClient != nil {
		if err := l.sftpClient.Close(); err != nil {
			errs = multierror.Append(errs, errors.Wrap(err, "closing sftp conn"))
		}
	}

	if l.sshClient != nil {
		if err := l.sshClient.Close(); err != nil {
			errs = multierror.Append(errs, errors.Wrap(err, "closing ssh conn"))
		}
	}

	return errs
}

// Container retrieves a stow.Container based on its name which must be exact.
func (l *location) Container(id string) (stow.Container, error) {
	fi, err := l.sftpClient.Stat(filepath.Join(l.config.basePath, id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stow.ErrNotFound
		}
		return nil, err
	}
	if !fi.IsDir() {
		return nil, stow.ErrNotFound
	}
	return &container{
		location: l,
		name:     id,
	}, nil
}

// RemoveContainer removes a container by name.
func (l *location) RemoveContainer(id string) error {
	return recurseRemove(l.sftpClient, filepath.Join(l.config.basePath, id))
}

// recurseRemove recursively purges content from a path.
func recurseRemove(client *sftp.Client, path string) error {
	infos, err := client.ReadDir(path)
	if err != nil {
		return err
	}

	for _, v := range infos {
		if !v.IsDir() {
			return errors.Errorf("directory not empty - %q", v.Name())
		}
		if err := recurseRemove(client, filepath.Join(path, v.Name())); err != nil {
			return err
		}
	}

	return client.RemoveDirectory(path)
}

// ItemByURL retrieves a stow.Item by parsing the URL.
func (l *location) ItemByURL(u *url.URL) (stow.Item, error) {
	// expect sftp://<username>@<host>:<port>/<container>/<path to file>
	// example: sftp://someuser@example.com:22/foo/blah/baz.txt

	urlParts := strings.Split(u.Path, "/")
	if len(urlParts) < 3 {
		return nil, errors.New("parsing ItemByURL unexpected length")
	}

	containerName := urlParts[1]
	itemName := strings.Join(urlParts[2:], "/")

	c, err := l.Container(containerName)
	if err != nil {
		return nil, errors.Wrapf(err, "ItemByURL, getting container %q", containerName)
	}

	i, err := c.Item(itemName)
	if err != nil {
		return nil, errors.Wrapf(err, "ItemByURL, getting item %q", itemName)
	}

	return i, nil
}

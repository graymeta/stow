package remote

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"github.com/graymeta/stow"
)

type location struct {
	// config is the configuration for this location.
	config stow.Config
	// pagesize is the number of items to return
	// per page (for Containers and Items).
	pagesize int
}

func (l *location) Close() error {
	return nil // nothing to close
}

func (l *location) ItemByURL(u *url.URL) (stow.Item, error) {
	i := &item{}
	i.path = u.Path
	return i, nil
}

func (l *location) RemoveContainer(id string) error {
	return os.RemoveAll(id)
}

func (l *location) CreateContainer(name string) (stow.Container, error) {
	path, ok := l.config.Config(ConfigKeyTarget)
	if !ok {
		return nil, errors.New("missing " + ConfigKeyTarget + " configuration")
	}
	fullpath := filepath.Join(path, name)
	if err := os.Mkdir(fullpath, 0777); err != nil {
		return nil, err
	}
	abspath, err := filepath.Abs(fullpath)
	if err != nil {
		return nil, err
	}
	return &container{
		pagesize: l.pagesize,
		name:     name,
		path:     abspath,
	}, nil
}

func (l *location) Containers(prefix string, cursor string) ([]stow.Container, string, error) {
	path, ok := l.config.Config(ConfigKeyTarget)
	if !ok {
		return nil, "", errors.New("missing " + ConfigKeyTarget + " configuration")
	}
	files, err := filepath.Glob(filepath.Join(path, prefix+"*"))
	if err != nil {
		return nil, "", err
	}
	cs, err := l.filesToContainers(path, files...)
	if err != nil {
		return nil, "", err
	}
	if cursor != stow.CursorStart {
		// seek to the cursor
		ok := false
		for i, c := range cs {
			if c.ID() == cursor {
				ok = true
				cs = cs[i:]
				break
			}
		}
		if !ok {
			return nil, "", stow.ErrBadCursor
		}
	}
	if len(cs) > l.pagesize {
		cursor = cs[l.pagesize].ID()
		cs = cs[:l.pagesize] // limit cs to pagesize
	} else if len(cs) <= l.pagesize {
		cursor = ""
	}

	return cs, cursor, err
}

func (l *location) Container(id string) (stow.Container, error) {
	path, ok := l.config.Config(ConfigKeyTarget)
	if !ok {
		return nil, errors.New("missing " + ConfigKeyTarget + " configuration")
	}
	containers, err := l.filesToContainers(path, id)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, stow.ErrNotFound
		}
		return nil, err
	}
	if len(containers) == 0 {
		return nil, stow.ErrNotFound
	}
	return containers[0], nil
}

// filesToContainers takes a list of files and turns it into a
// stow.ContainerList.
func (l *location) filesToContainers(root string, files ...string) ([]stow.Container, error) {
	cs := make([]stow.Container, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		absroot, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		path, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		name, err := filepath.Rel(absroot, path)
		if err != nil {
			return nil, err
		}
		cs = append(cs, &container{
			pagesize: l.pagesize,
			name:     name,
			path:     path,
		})
	}
	return cs, nil
}
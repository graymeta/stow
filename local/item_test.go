package local_test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cheekybits/is"
	"github.com/flyteorg/stow"
	"github.com/flyteorg/stow/local"
)

func TestItemReader(t *testing.T) {
	is := is.New(t)
	testDir, teardown, err := setup()
	is.NoErr(err)
	defer teardown()

	cfg := stow.ConfigMap{"path": testDir}
	l, err := stow.Dial(local.Kind, cfg)
	is.NoErr(err)
	is.OK(l)
	containers, cursor, err := l.Containers("t", stow.CursorStart, 10)
	is.NoErr(err)
	is.OK(containers)
	is.Equal(cursor, "")
	three, err := l.Container(containers[0].ID())
	is.NoErr(err)

	items, cursor, err := three.Items("", stow.CursorStart, 10)
	is.NoErr(err)
	is.Equal(cursor, "")
	item1 := items[0]

	rc, err := item1.Open()
	is.NoErr(err)
	defer rc.Close()
	b, err := ioutil.ReadAll(rc)
	is.NoErr(err)
	is.Equal("3.1", string(b))

}

func TestHardlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}
	is := is.New(t)
	testDir, teardown, err := setup()
	is.NoErr(err)
	defer teardown()

	cfg := stow.ConfigMap{"path": testDir}
	l, err := stow.Dial(local.Kind, cfg)
	is.NoErr(err)
	is.OK(l)

	containers, cursor, err := l.Containers("z", stow.CursorStart, 10)
	is.NoErr(err)
	is.OK(containers)
	is.Equal(cursor, "")

	links, err := l.Container(containers[0].ID())
	is.NoErr(err)

	items, cursor, err := links.Items("", stow.CursorStart, 10)
	is.NoErr(err)
	is.Equal(cursor, "")

	for _, item := range items {
		if item.Name() == "hardlink" {
			meta, err := item.Metadata()
			is.NoErr(err)
			is.OK(meta)

			is.Equal(meta["is_dir"], false)
			is.True(meta["is_hardlink"])
			is.False(meta["is_symlink"])
			break
		}
	}
}

func TestSymLink(t *testing.T) {
	is := is.New(t)
	testDir, teardown, err := setup()
	is.NoErr(err)
	defer teardown()

	cfg := stow.ConfigMap{"path": testDir}
	l, err := stow.Dial(local.Kind, cfg)
	is.NoErr(err)
	is.OK(l)

	containers, cursor, err := l.Containers("z", stow.CursorStart, 10)
	is.NoErr(err)
	is.OK(containers)
	is.Equal(cursor, "")

	links, err := l.Container(containers[0].ID())
	is.NoErr(err)

	items, cursor, err := links.Items("", stow.CursorStart, 10)
	is.NoErr(err)
	is.Equal(cursor, "")

	for _, item := range items {
		if item.Name() == "symlink" {
			meta, err := item.Metadata()
			is.NoErr(err)
			is.OK(meta)

			is.Equal(meta["is_dir"], false)
			is.False(meta["is_hardlink"])
			is.True(meta["is_symlink"])

			linkStr, ok := meta["link"].(string)
			is.True(ok)
			is.OK(linkStr)

			is.True(strings.Contains(linkStr, "symtarget"))
			break
		}
	}
}

func TestItemFromURL(t *testing.T) {
	//A file-system path /a/b/c can be broken into (container, name)
	//either as (/a, b/c) or as (/a/b,c). This test imposes
	//the convention that the name is the last path component.
	is := is.New(t)
	testDir, teardown, err := setup()
	is.NoErr(err)
	defer teardown()
	os.MkdirAll(filepath.Join(testDir, "a", "b"), 0777)
	ioutil.WriteFile(filepath.Join(testDir, "a", "f2"), []byte("abc"), 0666)
	ioutil.WriteFile(filepath.Join(testDir, "a", "b", "f3"), []byte("abc"), 0666)

	cfg := stow.ConfigMap{"path": testDir}
	l, err := stow.Dial(local.Kind, cfg)
	is.NoErr(err)

	item2, err := l.ItemByURL(&url.URL{
		Scheme: "file",
		Path:   filepath.Join(testDir, "a", "f2"),
	})
	is.NoErr(err)
	is.Equal(item2.Name(), "f2")

	item3, err := l.ItemByURL(&url.URL{
		Scheme: "file",
		Path:   filepath.Join(testDir, "a", "b", "f3"),
	})
	is.NoErr(err)
	is.Equal(item3.Name(), "f3")
}

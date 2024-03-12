package b2

import (
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	isi "github.com/cheekybits/is"
	"github.com/flyteorg/stow"
	"github.com/flyteorg/stow/test"
)

func TestStow(t *testing.T) {
	is := isi.New(t)
	accountID := os.Getenv("B2_ACCOUNT_ID")
	applicationKey := os.Getenv("B2_APPLICATION_KEY")

	if accountID == "" || applicationKey == "" {
		t.Skip("Backblaze credentials missing from environment. Skipping tests")
	}

	cfg := stow.ConfigMap{
		"account_id":      accountID,
		"application_key": applicationKey,
	}

	location, err := stow.Dial("b2", cfg)
	is.NoErr(err)
	is.OK(location)

	t.Run("basic stow interface tests", func(t *testing.T) {
		test.All(t, "b2", cfg)
	})

	// This test is designed to test the container.Items() function. B2 doesn't
	// support listing items in a bucket by prefix, so our implementation fakes this
	// functionality by requesting additional pages of files
	t.Run("Items with prefix", func(t *testing.T) {
		is := isi.New(t)
		container, err := location.CreateContainer("stowtest" + randName(10))
		is.NoErr(err)
		is.OK(container)

		defer func() {
			is.NoErr(location.RemoveContainer(container.ID()))
		}()

		// add some items to the container
		content := "foo"
		item1, err := container.Put("b/a", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)
		item2, err := container.Put("b/bb", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)
		item3, err := container.Put("b/bc", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)
		item4, err := container.Put("b/bd", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)

		defer func() {
			is.NoErr(container.RemoveItem(item1.ID()))
			is.NoErr(container.RemoveItem(item2.ID()))
			is.NoErr(container.RemoveItem(item3.ID()))
			is.NoErr(container.RemoveItem(item4.ID()))
		}()

		items, cursor, err := container.Items("b/b", stow.CursorStart, 2)
		is.NoErr(err)
		is.Equal(len(items), 2)
		is.Equal(cursor, "b/bc ")

		items, cursor, err = container.Items("", stow.CursorStart, 2)
		is.NoErr(err)
		is.Equal(len(items), 2)
		is.Equal(cursor, "b/bb ")
	})

	t.Run("Item Delete", func(t *testing.T) {
		is := isi.New(t)
		container, err := location.CreateContainer("stowtest" + randName(10))
		is.NoErr(err)
		is.OK(container)

		defer func() {
			is.NoErr(location.RemoveContainer(container.ID()))
		}()

		// Put an item twice, creating two versions of the file
		content := "foo"
		i, err := container.Put("foo", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)
		content = "foo_v2"
		_, err = container.Put("foo", strings.NewReader(content), int64(len(content)), nil)
		is.NoErr(err)

		is.NoErr(container.RemoveItem(i.ID()))

		// verify item is gone
		_, err = container.Item(i.ID())
		is.Equal(err, stow.ErrNotFound)
	})
}

func randName(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

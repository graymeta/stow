package sftp

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flyteorg/stow"
	"github.com/flyteorg/stow/test"
	"github.com/stretchr/testify/require"
)

func TestStow(t *testing.T) {
	config := stow.ConfigMap{
		ConfigHost:     os.Getenv("SFTP_HOST"),
		ConfigPort:     os.Getenv("SFTP_PORT"),
		ConfigUsername: os.Getenv("SFTP_USERNAME"),
	}
	if config[ConfigHost] == "" ||
		config[ConfigPort] == "" ||
		config[ConfigUsername] == "" {
		t.Skip("skipping tests because environment isn't configured")
	}

	b, err := ioutil.ReadFile(os.Getenv("SFTP_PRIVATE_KEY_FILE"))
	require.NoError(t, err)
	config[ConfigPrivateKey] = string(b)
	config[ConfigPrivateKeyPassphrase] = os.Getenv("SFTP_PRIVATE_KEY_PASSPHRASE")

	t.Run("stow_tests", func(t *testing.T) {
		test.All(t, Kind, config)
	})

	t.Run("additional_tests", func(t *testing.T) {
		location, err := stow.Dial(Kind, config)
		require.NoError(t, err)
		defer location.Close()

		t.Run("set of files 1", func(t *testing.T) {
			cont, err := location.CreateContainer("stowtest" + randName(10))
			require.NoError(t, err)
			defer location.RemoveContainer(cont.ID())

			files := []string{
				"a.jpg",
				"bar/a.jpg",
				"bar/b.jpg",
				"bar/baz/a.jpg",
				"bar/baz/b.jpg",
				"foo/a.jpg",
				"foo/b.jpg",
				"z.jpg",
			}

			setupFiles(t, cont, files)

			t.Run("no prefix, no cursor, len 4", func(t *testing.T) {
				items, cursor, err := cont.Items("", "", 4)
				require.NoError(t, err)
				require.Len(t, items, 4)
				require.Equal(t, files[0], items[0].ID())
				require.Equal(t, files[1], items[1].ID())
				require.Equal(t, files[2], items[2].ID())
				require.Equal(t, files[3], items[3].ID())
				require.Equal(t, cursor, "bar/baz/a.jpg")
			})

			t.Run("no prefix, no cursor, len 5", func(t *testing.T) {
				items, cursor, err := cont.Items("", "", 5)
				require.NoError(t, err)
				require.Len(t, items, 5)
				require.Equal(t, files[0], items[0].ID())
				require.Equal(t, files[1], items[1].ID())
				require.Equal(t, files[2], items[2].ID())
				require.Equal(t, files[3], items[3].ID())
				require.Equal(t, files[4], items[4].ID())
				require.Equal(t, cursor, "bar/baz/b.jpg")
			})

			t.Run("no prefix, no cursor, len 100", func(t *testing.T) {
				items, cursor, err := cont.Items("", "", 100)
				require.NoError(t, err)
				require.Len(t, items, len(files))
				require.Equal(t, "", cursor)
			})

			t.Run("no prefix, with cursor, len 100", func(t *testing.T) {
				items, cursor, err := cont.Items("", "bar/baz/a.jpg", 100)
				require.NoError(t, err)
				require.Len(t, items, 4)
				require.Equal(t, "", cursor)
			})
		})

		t.Run("set of files 2", func(t *testing.T) {
			cont, err := location.CreateContainer("stowtest" + randName(10))
			require.NoError(t, err)
			defer location.RemoveContainer(cont.ID())

			files := []string{
				"bar/baz/a.jpg",
				"bar/baz/b.jpg",
				"bar/baz/c.jpg",
				"bar/baz/d.jpg",
				"bar/baz/e.jpg",
			}

			setupFiles(t, cont, files)

			t.Run("no prefix, no cursor, len 3", func(t *testing.T) {
				items, cursor, err := cont.Items("", "", 3)
				require.NoError(t, err)
				require.Len(t, items, 3)
				require.Equal(t, files[2], cursor)
			})

			t.Run("no prefix, no cursor, len 5", func(t *testing.T) {
				items, cursor, err := cont.Items("", "", 5)
				require.NoError(t, err)
				require.Len(t, items, 5)
				require.Equal(t, "", cursor)
			})

			t.Run("no prefix, with cursor, len 5", func(t *testing.T) {
				items, cursor, err := cont.Items("", "bar/baz/b.jpg", 5)
				require.NoError(t, err)
				require.Len(t, items, 3)
				require.Equal(t, "", cursor)
			})

			t.Run("no prefix, with cursor (a), len 5", func(t *testing.T) {
				items, cursor, err := cont.Items("", "a", 5)
				require.NoError(t, err)
				require.Len(t, items, 5)
				require.Equal(t, "", cursor)
			})
		})
	})

	t.Run("stow_tests - with base path", func(t *testing.T) {
		basePath := os.Getenv("SFTP_BASEPATH")
		if basePath == "" {
			t.Skip("skipping base paths test due to SFTP_BASEPATH not being set")
		}
		config[ConfigBasePath] = basePath
		test.All(t, Kind, config)
	})
}

func setupFiles(t *testing.T, c stow.Container, files []string) {
	for _, file := range files {
		_, err := c.Put(file, strings.NewReader(""), 0, nil)
		require.NoError(t, err)
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz")

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

func randName(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

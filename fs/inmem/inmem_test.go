package inmem_test

import (
	"io"
	"io/fs"
	"testing"

	"github.com/harveysanders/protohackers/fs/inmem"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		memFS := inmem.FS{}

		file, err := memFS.Open("./test.txt")
		pathError := &fs.PathError{}
		require.ErrorAs(t, err, &pathError)

		require.Nil(t, file)
	})

	t.Run("mkdir, then open", func(t *testing.T) {
		memFS := inmem.New()
		memFS.MkdirAll("test", 0755)

		file, err := memFS.Open("test")
		require.NoError(t, err)

		require.NotNil(t, file)
	})

	t.Run("open file", func(t *testing.T) {
		memFS := inmem.New()
		memFS.MkdirAll("test", 0755)
		memFS.WriteFile("test/test.txt", []byte("hello"), 0644)

		file, err := memFS.Open("test/test.txt")
		require.NoError(t, err)

		require.NotNil(t, file)
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, "hello", string(contents))
	})
}

package inmem_test

import (
	"strings"
	"testing"

	"github.com/harveysanders/protohackers/10-voracious-code-storage/inmem"
	"github.com/stretchr/testify/require"
)

func TestCreateRevision(t *testing.T) {
	t.Run("persists a file", func(t *testing.T) {
		contents := "Hello, World!\n"
		filePath := "/test.txt"
		store := inmem.New()

		nWrote, rev, err := store.CreateRevision(filePath, strings.NewReader(contents))
		require.NoError(t, err)
		require.Equal(t, int64(len(contents)), nWrote)
		require.Equal(t, "r1", rev)
	})
}

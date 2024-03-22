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
		err := memFS.MkdirAll("test", 0755)
		require.NoError(t, err)

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

func TestSplitPaths(t *testing.T) {
	testCases := []struct {
		name         string
		path         string
		wantDirs     []string
		wantFileName string
	}{
		{
			name:         "root",
			path:         "/",
			wantDirs:     []string{"/"},
			wantFileName: "",
		},
		{
			name:         "file",
			path:         "/test.txt",
			wantDirs:     []string{"/"},
			wantFileName: "test.txt",
		},
		{
			name:         "nested file",
			path:         "/test/nested.txt",
			wantDirs:     []string{"/", "test"},
			wantFileName: "nested.txt",
		},
		{
			name:         "deeper nested file",
			path:         "/test/abc/nested.txt",
			wantDirs:     []string{"/", "test", "abc"},
			wantFileName: "nested.txt",
		},
		{
			name:         "relative path",
			path:         "test.txt",
			wantDirs:     []string{""},
			wantFileName: "test.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dirs, fileName := inmem.SplitPath(tc.path)
			require.Equal(t, tc.wantDirs, dirs)
			require.Equal(t, tc.wantFileName, fileName)
		})
	}
}

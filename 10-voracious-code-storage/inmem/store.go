package inmem

import (
	"fmt"
	"io"
	"sync"

	"github.com/harveysanders/protohackers/fs/inmem"
)

var appDirName = "vcs"

type Store struct {
	mu sync.RWMutex
	// Map of public file paths to a map revision IDs to private file paths.
	// Ex:
	//  "/test.txt" -> {"r1": "/tmp/test.txt.r1"}
	revs map[string]map[string]string
	fs   *inmem.FS
}

type Option func(*Store)

// New creates a new Store with the given options.
func New(opts ...Option) *Store {
	s := &Store{
		mu:   sync.RWMutex{},
		revs: make(map[string]map[string]string, 512),
		fs:   inmem.New(),
	}
	for _, o := range opts {
		o(s)
	}

	return s
}

func (s *Store) CreateRevision(filepath string, r io.Reader) (int64, string, error) {
	publicPath := filepath
	var revN int

	s.mu.Lock()
	defer s.mu.Unlock()
	revs, ok := s.revs[filepath]

	if !ok {
		revs = make(map[string]string, 4)
		s.revs[publicPath] = revs
	} else {
		revN = len(revs)
	}

	revisionTag := fmt.Sprintf("r%d", revN+1)
	privatePath := fmt.Sprintf("%s.%s", publicPath, revisionTag)

	contents, err := io.ReadAll(r)
	if err != nil {
		return 0, "", fmt.Errorf("read contents: %w", err)
	}
	err = s.fs.WriteFile(privatePath, contents, 0755)
	if err != nil {
		return 0, revisionTag, fmt.Errorf("io.Copy: %w", err)
	}

	revs[revisionTag] = privatePath
	s.revs[publicPath] = revs

	return int64(len(contents)), revisionTag, nil
}

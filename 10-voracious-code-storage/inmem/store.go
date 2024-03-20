package inmem

import (
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/harveysanders/protohackers/fs/inmem"
)

var appDirName = "vcs"

var ErrNotFound = fmt.Errorf("not found")

type revision struct {
	privatePath string
	id          string
}

type Store struct {
	mu sync.RWMutex
	// Map of public file paths to a map revision IDs to private file paths.
	// Ex:
	//  "/test.txt" -> {"r1": "/tmp/test.txt.r1"}
	revs map[string][]revision
	fs   *inmem.FS
}

type Option func(*Store)

// New creates a new Store with the given options.
func New(opts ...Option) *Store {
	s := &Store{
		mu:   sync.RWMutex{},
		revs: make(map[string][]revision, 512),
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
		revs = make([]revision, 0, 4)
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

	revs = append(revs, revision{id: revisionTag, privatePath: privatePath})
	s.revs[publicPath] = revs

	return int64(len(contents)), revisionTag, nil
}

func (s *Store) GetRevision(filepath, revID string) (fs.File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	revs, ok := s.revs[filepath]
	if !ok || len(revs) == 0 {
		return nil, fmt.Errorf("no revisions for %s", filepath)
	}

	if revID == "" {
		latest := revs[len(revs)-1]
		return s.fs.Open(latest.privatePath)
	}

	for _, r := range revs {
		if r.id == revID {
			return s.fs.Open(r.privatePath)
		}
	}

	return nil, fmt.Errorf("no revision %s for %s", revID, filepath)

}

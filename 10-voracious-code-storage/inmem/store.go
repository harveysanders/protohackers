package inmem

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/harveysanders/protohackers/fs/inmem"
)

var ErrNotFound = fmt.Errorf("not found")

type revision struct {
	privatePath string
	id          string
}

type Store struct {
	fileRevRegex *regexp.Regexp
	// Path to the directory where revisions are stored.
	revisionsDir string
	mu           sync.RWMutex
	// Map of public file paths to a map revision IDs to private file paths.
	// Ex:
	//  "/test.txt" -> {"r1": "/tmp/test.txt.r1"}
	revs map[string][]revision
	fs   *inmem.FS
}

type Option func(*Store)

// New creates a new Store with the given options.
func New(opts ...Option) *Store {
	fileRevRegex := regexp.MustCompile(`\.r\d+$`)
	s := &Store{
		revisionsDir: "/.revisions",
		fileRevRegex: fileRevRegex,
		mu:           sync.RWMutex{},
		revs:         make(map[string][]revision, 512),
		fs:           inmem.New(),
	}
	for _, o := range opts {
		o(s)
	}

	return s
}

// CreateRevision creates a new revision of the file at the given path. The file contains meta information about the revisions. The revisions are stores in the /.revisions directory.
// Ex:
//
//	"/tmp/test.txt" -> "/tmp/test.txt
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
	revisionPath := path.Join(s.revisionsDir, filepath+"."+revisionTag)

	contents, err := io.ReadAll(r)
	if err != nil {
		return 0, "", fmt.Errorf("read contents: %w", err)
	}
	err = s.fs.WriteFile(revisionPath, contents, 0755)
	if err != nil {
		return 0, revisionTag, fmt.Errorf("io.Copy: %w", err)
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := s.fs.OpenFile(publicPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, revisionTag, fmt.Errorf("s.fs.openFile: %q %w", publicPath, err)
	}
	metadata := fmt.Sprintf("%s:%s\n", revisionTag, revisionPath)
	if _, err := f.Write([]byte(metadata)); err != nil {
		f.Close() // ignore error; Write error takes precedence
		return 0, revisionTag, fmt.Errorf("f.Write: %w", err)
	}

	revs = append(revs, revision{id: revisionTag, privatePath: revisionPath})
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

// ListEntries returns a list of entries in the given path.
// If the entry is a file, the list item will contain the file name and the latest revision ID.
// If the entry is a directory, the list item will contain the directory name followed by a space and the string "DIR".
// Ex:
//
//	"/tmp" -> ["dirA/ DIR", "test.txt r2"]
func (s *Store) ListEntries(path string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := s.fs.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("fs.ReadDir: %w", err)
	}
	res := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			if strings.Contains(s.revisionsDir, e.Name()) {
				continue
			}
			res = append(res, fmt.Sprintf("%s/ DIR", e.Name()))
		} else {
			ogFileName := s.fileRevRegex.ReplaceAllString(e.Name(), "")
			revKey := path + ogFileName
			revs, ok := s.revs[revKey]
			if !ok {
				log.Printf("no revisions for %s", revKey)
				continue
			}
			latest := revs[len(revs)-1]
			res = append(res, fmt.Sprintf("%s %s", ogFileName, latest.id))
		}
	}

	slices.Sort(res)
	return res, nil
}

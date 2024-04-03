package inmem

import (
	"errors"
	"time"
)

var (
	ErrSiteNotFound   = errors.New("site not found")
	ErrPolicyNotFound = errors.New("policy not found")
)

type Site struct {
	ID uint32
	// TargetPopulations is a map of target population species names to TargetPopulation structs.
	TargetPopulations map[string]TargetPopulation
	Policies          map[string]*Policy
}

type TargetPopulation struct {
	Species string
	Min     uint32
	Max     uint32
}

type Policy struct {
	ID        uint32
	Species   string
	Action    PolicyAction
	CreatedAt time.Time
	DeletedAt time.Time
}

type PolicyAction byte

const (
	Cull     PolicyAction = 0x90
	Conserve PolicyAction = 0xa0
)

type Store struct {
	// sites is a map of siteID to sites.
	sites map[uint32]Site
}

// NewStore returns a new in-memory store.
func NewStore() *Store {
	return &Store{
		sites: make(map[uint32]Site, 200),
	}
}

func (s *Store) AddSite(site Site) {
	s.sites[site.ID] = site
}

func (s *Store) GetSite(siteID uint32) (Site, error) {
	site, ok := s.sites[siteID]
	if !ok {
		return Site{}, ErrSiteNotFound
	}
	return site, nil
}

func (s *Store) SetPolicy(siteID uint32, species string, action PolicyAction) error {
	site, ok := s.sites[siteID]
	if !ok {
		return ErrSiteNotFound
	}
	policy := &Policy{
		Species:   species,
		Action:    action,
		CreatedAt: time.Now(),
	}
	site.Policies[species] = policy
	s.sites[siteID] = site
	return nil
}

func (s *Store) GetPolicy(siteID uint32, species string) (*Policy, error) {
	site, ok := s.sites[siteID]
	if !ok {
		return nil, ErrSiteNotFound
	}
	policy, ok := site.Policies[species]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

func (s *Store) DeletePolicy(siteID uint32, species string) error {
	site, ok := s.sites[siteID]
	if !ok {
		return ErrSiteNotFound
	}

	p, ok := site.Policies[species]
	if !ok {
		return ErrPolicyNotFound
	}
	p.DeletedAt = time.Now()
	return nil
}

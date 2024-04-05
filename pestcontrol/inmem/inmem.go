package inmem

import (
	"context"
	"time"

	pc "github.com/harveysanders/protohackers/pestcontrol"
)

type Store struct {
	// sites is a map of siteID to sites.
	sites map[uint32]pc.Site
}

// NewStore returns a new in-memory store.
func NewStore() Store {
	return Store{
		sites: make(map[uint32]pc.Site, 200),
	}
}

func (s Store) SetTargetPopulations(ctx context.Context, siteID uint32, pops []pc.TargetPopulation) error {
	site := pc.Site{ID: siteID}
	site.TargetPopulations = make(map[string]pc.TargetPopulation, len(pops))
	site.Policies = make(map[string]pc.Policy, len(pops))

	for _, pop := range pops {
		site.TargetPopulations[pop.Species] = pop
	}
	return s.AddSite(ctx, site)
}

func (s Store) AddSite(ctx context.Context, site pc.Site) error {
	s.sites[site.ID] = site
	return nil
}

func (s Store) GetSite(ctx context.Context, siteID uint32) (pc.Site, error) {
	site, ok := s.sites[siteID]
	if !ok {
		return pc.Site{}, pc.ErrPolicyNotFound
	}
	return site, nil
}

func (s Store) SetPolicy(ctx context.Context, siteID uint32, species string, action pc.PolicyAction) error {
	site, ok := s.sites[siteID]
	if !ok {
		return pc.ErrSiteNotFound
	}
	policy := pc.Policy{
		Species:   species,
		Action:    action,
		CreatedAt: time.Now(),
	}
	site.Policies[species] = policy
	s.sites[siteID] = site
	return nil
}

func (s Store) GetPolicy(ctx context.Context, siteID uint32, species string) (pc.Policy, error) {
	site, ok := s.sites[siteID]
	if !ok {
		return pc.Policy{}, pc.ErrSiteNotFound
	}
	policy, ok := site.Policies[species]
	if !ok {
		return pc.Policy{}, pc.ErrPolicyNotFound
	}
	return policy, nil
}

func (s Store) DeletePolicy(ctx context.Context, siteID uint32, species string) (pc.Policy, error) {
	site, ok := s.sites[siteID]
	if !ok {
		return pc.Policy{}, pc.ErrSiteNotFound
	}

	p, ok := site.Policies[species]
	if !ok {
		return pc.Policy{}, pc.ErrPolicyNotFound
	}
	p.DeletedAt = time.Now()
	return p, nil
}

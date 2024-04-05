package sqlite

import (
	"context"
	"fmt"
	"time"

	pc "github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/sqlite/sqlc"
)

type SiteService struct {
	queries *sqlc.Queries
}

//	type Store interface {
//		AddSite(site Site) error
//		GetSite(siteID uint32) (Site, error)
//		SetPolicy(siteID uint32, species string, action PolicyAction) error
//		GetPolicy(siteID uint32, species string) (Policy, error)
//		DeletePolicy(siteID uint32, species string) (Policy, error)
//		SetTargetPopulations(siteID uint32, pops []TargetPopulation) error
//	}

func NewSiteService(db sqlc.DBTX) *SiteService {
	queries := sqlc.New(db)
	return &SiteService{
		queries: queries,
	}
}

func (s *SiteService) AddSite(ctx context.Context, site pc.Site) error {
	_, err := s.queries.CreateSite(ctx, sqlc.CreateSiteParams{
		ID:        site.ID,
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	// TODO: Batch these inserts
	for _, pop := range site.TargetPopulations {
		species, err := s.queries.GetSpeciesByName(ctx, pop.Species)
		if err != nil {
			return fmt.Errorf("queries.GetSpeciesByName: %w", err)
		}
		if species.Name == "" {
			species, err = s.queries.CreateSpecies(ctx, sqlc.CreateSpeciesParams{
				Name:      pop.Species,
				CreatedAt: time.Now().Format(time.RFC3339)})
			if err != nil {
				return fmt.Errorf("queries.CreateSpecies: %w", err)
			}
		}
		_, err = s.queries.CreateTargetPopulation(ctx, sqlc.CreateTargetPopulationParams{
			SiteID:    site.ID,
			SpeciesID: species.ID,
			Min:       pop.Min,
			Max:       pop.Max,
		})
		if err != nil {
			return fmt.Errorf("queries.CreateTargetPopulation: %w", err)
		}
	}
	return nil
}

func (s *SiteService) GetSite(ctx context.Context, siteID uint32) (pc.Site, error) {
	dbSite, err := s.queries.GetSite(ctx, siteID)
	if err != nil {
		return pc.Site{}, err
	}
	return pc.Site{
		ID:                dbSite.ID,
		Policies:          make(map[string]pc.Policy),
		TargetPopulations: make(map[string]pc.TargetPopulation),
	}, nil
}

func (s *SiteService) SetPolicy(ctx context.Context, siteID uint32, species string, action pc.PolicyAction) error {
	return nil
}

func (s *SiteService) GetPolicy(ctx context.Context, siteID uint32, species string) (pc.Policy, error) {
	return pc.Policy{}, nil
}

func (s *SiteService) DeletePolicy(ctx context.Context, siteID uint32, species string) (pc.Policy, error) {
	return pc.Policy{}, nil
}

func (s *SiteService) SetTargetPopulations(ctx context.Context, siteID uint32, pops []pc.TargetPopulation) error {
	return nil
}

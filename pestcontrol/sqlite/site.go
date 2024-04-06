package sqlite

import (
	"context"
	"database/sql"
	"errors"
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
		return fmt.Errorf("queries.CreateSite: %w", err)
	}
	// TODO: Batch these inserts
	for _, pop := range site.TargetPopulations {
		species, err := s.queries.GetSpeciesByName(ctx, pop.Species)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
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
			CreatedAt: time.Now().Format(time.RFC3339),
		})
		if err != nil {
			return fmt.Errorf("queries.CreateTargetPopulation: %w", err)
		}
	}
	return nil
}

func (s *SiteService) GetSite(ctx context.Context, siteID uint32) (pc.Site, error) {
	pops, err := s.queries.GetSiteTargetPopulations(ctx, siteID)
	if err != nil {
		return pc.Site{}, fmt.Errorf("queries.GetSiteTargetPopulations: %w", err)
	}
	site := pc.Site{
		ID:                siteID,
		Policies:          make(map[string]pc.Policy),
		TargetPopulations: make(map[string]pc.TargetPopulation, len(pops)),
	}
	for _, pop := range pops {
		site.TargetPopulations[pop.Name] = pc.TargetPopulation{
			Species: pop.Name,
			Min:     pop.Min,
			Max:     pop.Max,
		}
	}

	return site, nil
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
	site, err := s.GetSite(ctx, siteID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		site = pc.Site{ID: siteID}
	}

	if site.TargetPopulations == nil || len(site.TargetPopulations) == 0 {
		site.TargetPopulations = make(map[string]pc.TargetPopulation, len(pops))
		for _, pop := range pops {
			site.TargetPopulations[pop.Species] = pop
		}
		return s.AddSite(ctx, site)
	}
	return nil
}

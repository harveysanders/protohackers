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
			return fmt.Errorf("queries.GetSpeciesByName (species: %s): %w", pop.Species, err)
		}
		if species.Name == "" {
			species, err = s.queries.CreateSpecies(ctx, sqlc.CreateSpeciesParams{
				Name:      pop.Species,
				CreatedAt: time.Now().Format(time.RFC3339)})
			if err != nil {
				return fmt.Errorf("queries.CreateSpecies (species: %s): %w", pop.Species, err)
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
			return fmt.Errorf("queries.CreateTargetPopulation (species: %s): %w", pop.Species, err)
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

func (s *SiteService) SetPolicy(ctx context.Context, policyID uint32, siteID uint32, species string, action pc.PolicyAction) error {
	target, err := s.queries.GetSiteSpeciesTargetPopulation(ctx, sqlc.GetSiteSpeciesTargetPopulationParams{
		SiteID: siteID,
		Name:   species,
	})
	if err != nil {
		return fmt.Errorf("queries.GetSiteSpeciesTargetPopulation: %w", err)
	}

	_, err = s.queries.CreatePolicy(ctx, sqlc.CreatePolicyParams{
		ID:           policyID,
		PopulationID: target.ID,
		Action:       uint32(action),
		CreatedAt:    time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("queries.CreatePolicy: %w", err)
	}
	return nil
}

func (s *SiteService) GetPolicy(ctx context.Context, siteID uint32, species string) (pc.Policy, error) {
	p, err := s.queries.GetPolicyBySiteSpecies(ctx, sqlc.GetPolicyBySiteSpeciesParams{
		SiteID: siteID,
		Name:   species,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pc.Policy{}, pc.ErrPolicyNotFound
		}
		return pc.Policy{}, fmt.Errorf("queries.GetPolicyBySiteSpecies: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil {
		return pc.Policy{}, fmt.Errorf("createdAt time.Parse: %w", err)
	}
	return pc.Policy{
		ID:        p.ID,
		Action:    pc.PolicyAction(p.Action),
		CreatedAt: createdAt,
	}, nil
}

func (s *SiteService) DeletePolicy(ctx context.Context, siteID, id uint32) (pc.Policy, error) {
	deleted, err := s.queries.DeletePolicy(ctx, sqlc.DeletePolicyParams{
		ID:     id,
		SiteID: siteID,
		DeletedAt: sql.NullString{
			String: time.Now().Format(time.RFC3339),
			Valid:  true,
		},
	})
	if err != nil {
		return pc.Policy{}, fmt.Errorf("queries.DeletePolicy: %w", err)
	}

	deletedAt, err := time.Parse(time.RFC3339, deleted.DeletedAt.String)
	if err != nil {
		return pc.Policy{}, fmt.Errorf("deletedAt time.Parse: %w", err)
	}
	d := pc.Policy{
		ID:        deleted.ID,
		Action:    pc.PolicyAction(deleted.Action),
		CreatedAt: deletedAt,
		DeletedAt: deletedAt,
	}
	return d, nil
}

func (s *SiteService) SetTargetPopulations(ctx context.Context, siteID uint32, pops []pc.TargetPopulation) error {
	site, err := s.GetSite(ctx, siteID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("service.GetSite (site: %d): %w", siteID, err)
		}
		site = pc.Site{ID: siteID}
	}

	if site.TargetPopulations == nil || len(site.TargetPopulations) == 0 {
		site.TargetPopulations = make(map[string]pc.TargetPopulation, len(pops))
		for _, pop := range pops {
			site.TargetPopulations[pop.Species] = pop
		}
		if err := s.AddSite(ctx, site); err != nil {
			return fmt.Errorf("service.AddSite (site: %d): %w", siteID, err)
		}
		return nil
	}
	return nil
}

func (s *SiteService) RecordObservation(ctx context.Context, obs pc.Observation) error {
	species, err := s.getOrCreateSpecies(ctx, obs.Species)
	if err != nil {
		return fmt.Errorf("getOrCreateSpecies: %w", err)
	}
	_, err = s.queries.CreateObservation(ctx, sqlc.CreateObservationParams{
		SiteID:    obs.Site,
		SpeciesID: species.ID,
		Count:     obs.Count,
		CreatedAt: time.Now().Format(time.RFC3339),
		ClientID:  obs.ClientID,
	})
	if err != nil {
		return fmt.Errorf("queries.CreateObservation: %w", err)
	}
	return nil
}

func (s *SiteService) GetObservation(ctx context.Context, siteID uint32, speciesName string) (pc.Observation, error) {
	species, err := s.getOrCreateSpecies(ctx, speciesName)
	if err != nil {
		return pc.Observation{}, fmt.Errorf("getOrCreateSpecies: %w", err)
	}

	o, err := s.queries.GetObservationBySpecies(ctx, sqlc.GetObservationBySpeciesParams{
		SiteID:    siteID,
		SpeciesID: species.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pc.Observation{}, pc.ErrObservationNotFound
		}
		return pc.Observation{}, fmt.Errorf("queries.GetObservationBySpecies: %w", err)
	}
	return pc.Observation{
		Site:    siteID,
		Species: speciesName,
		Count:   o.Count,
	}, nil
}

func (s *SiteService) getOrCreateSpecies(ctx context.Context, name string) (sqlc.Species, error) {
	species, err := s.queries.GetSpeciesByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			species, err = s.queries.CreateSpecies(ctx, sqlc.CreateSpeciesParams{
				Name:      name,
				CreatedAt: time.Now().Format(time.RFC3339),
			})
			if err != nil {
				return sqlc.Species{}, fmt.Errorf("queries.CreateSpecies: %w", err)
			}
		} else {
			return sqlc.Species{}, fmt.Errorf("queries.GetSpeciesByName: %w", err)
		}
	}
	return species, nil
}

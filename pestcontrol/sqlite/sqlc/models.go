// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0

package sqlc

import (
	"database/sql"
)

type Observation struct {
	ID        uint32
	CreatedAt string
	SiteID    uint32
	SpeciesID uint32
	Count     uint32
	ClientID  uint32
}

type Policy struct {
	ID           uint32
	CreatedAt    string
	DeletedAt    sql.NullString
	Action       uint32
	PopulationID uint32
}

type Site struct {
	ID        uint32
	CreatedAt string
}

type Species struct {
	ID        uint32
	CreatedAt string
	Name      string
}

type TargetPopulation struct {
	ID        uint32
	CreatedAt string
	SiteID    uint32
	SpeciesID uint32
	Min       uint32
	Max       uint32
}
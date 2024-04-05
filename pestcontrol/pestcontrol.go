package pestcontrol

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
	Policies          map[string]Policy
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

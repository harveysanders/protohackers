package pestcontrol

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrObservationNotFound = errors.New("observation not found")
	ErrPolicyNotFound      = errors.New("policy not found")
	ErrSiteNotFound        = errors.New("site not found")
)

type Site struct {
	// ID is the unique identifier for the site.
	ID uint32
	// TargetPopulations is a map of target population species names to TargetPopulation structs.
	TargetPopulations map[string]TargetPopulation
	// Policies is a map of species names to Policy structs.
	Policies map[string]Policy
}

type TargetPopulation struct {
	Species string
	Min     uint32
	Max     uint32
}

type Observation struct {
	Site     uint32
	Species  string
	Count    uint32
	ClientID uint32
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

func (a PolicyAction) String() string {
	switch a {
	case Cull:
		return "Cull"
	case Conserve:
		return "Conserve"
	default:
		return fmt.Sprintf("unknown action: %x", int(a))
	}
}

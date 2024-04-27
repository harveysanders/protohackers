package pestcontrol_test

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

const (
	logKeySiteID  = "siteID"
	logKeyMsgType = "type"
	logKeySpecies = "species"
	logKeyPolicy  = "policy"
)

// MockAuthorityServer is a mock implementation of the Authority server.
type MockAuthorityServer struct {
	l   net.Listener
	log slog.Logger

	// sites is a map of site IDs to sites.
	sites map[uint32]site

	// policyChange is a channel sends a message when a policy is created or deleted.
	policyChange chan struct{}
}

type site struct {
	id                uint32
	targetPopulations map[string]pestcontrol.TargetPopulation
	// policyID is the next policy ID to assign for newly created policies. It's a simple incrementing counter. The are only unique per site.
	policyID uint32
	// policies is a map of species names to a slice of Policy structs. Each species should settle on a single policy, but can have multiple policies in a transient state. We're using the slice to here to ensure that the final state settles on a single policy. A map would force us to overwrite the previous policy, potentially missing a bug in the system.
	policies map[string][]pestcontrol.Policy
}

func NewMockAuthorityServer() *MockAuthorityServer {
	logHandler := slog.NewTextHandler(os.Stderr, nil)
	logger := slog.New(logHandler).With("name", "MockAuthority")

	return &MockAuthorityServer{
		sites:        map[uint32]site{},
		log:          *logger,
		policyChange: make(chan struct{}, 10),
	}
}

func (m *MockAuthorityServer) ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	m.l = l

	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		go m.handleConn(c)
	}
}

func (m *MockAuthorityServer) Close() error {
	return m.l.Close()
}

func (m *MockAuthorityServer) handleConn(conn net.Conn) {
	defer conn.Close()

	var siteID uint32

	for {
		m.log.Info("waiting for message")
		var msg proto.Message
		if _, err := msg.ReadFrom(conn); err != nil {
			m.log.Error(fmt.Sprintf("read message from client: %v", err))
			return
		}

		m.log.Info("incoming msg", "type", msg.Type)

		switch msg.Type {
		case proto.MsgTypeHello:
			resp, _ := proto.MsgHello{}.MarshalBinary()
			if _, err := conn.Write(resp); err != nil {
				m.log.Error(fmt.Sprintf("write hello: %v", err))
			}

		case proto.MsgTypeDialAuthority:
			da, err := msg.ToMsgDialAuthority()
			if err != nil {
				m.log.Error(fmt.Sprintf("to dial authority: %v", err))
				return
			}
			m.handleDialAuthority(conn, da)
			siteID = da.Site
			m.log.Info("connection established", logKeySiteID, da.Site)

		case proto.MsgTypeCreatePolicy:
			cp, err := msg.ToMsgCreatePolicy()
			if err != nil {
				m.log.Error(fmt.Sprintf("to create policy: %v", err))
				return
			}
			m.handleCreatePolicy(conn, siteID, cp)

		case proto.MsgTypeDeletePolicy:
			dp, err := msg.ToMsgDeletePolicy()
			if err != nil {
				m.log.Error(fmt.Sprintf("to delete policy: %v", err))
				return
			}
			m.handleDeletePolicy(conn, siteID, dp.Policy)
		}
	}
}

func (m *MockAuthorityServer) handleDialAuthority(conn net.Conn, da proto.MsgDialAuthority) {
	// Make sure we already have the site in our map
	site, ok := m.sites[da.Site]
	if !ok {
		m.log.Info("site not found", logKeySiteID, da.Site)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Error(fmt.Sprintf("write err resp: %v", err))
		}
	}

	// Respond with the site's target populations
	pop := make([]proto.PopulationTarget, 0, len(site.targetPopulations))
	for species, p := range site.targetPopulations {
		pop = append(pop, proto.PopulationTarget{
			Species: species,
			Min:     p.Min,
			Max:     p.Max,
		})
	}

	msg := proto.MsgTargetPopulations{
		Site:        site.id,
		Populations: pop,
	}
	respData, _ := msg.MarshalBinary()
	if _, err := conn.Write(respData); err != nil {
		m.log.Error(fmt.Sprintf("write target populations: %v", err))
	}
}

func (m *MockAuthorityServer) handleCreatePolicy(conn net.Conn, siteID uint32, cp proto.MsgCreatePolicy) {
	// Make sure we already have the site in our map
	site, ok := m.sites[siteID]
	if !ok {
		m.log.Info("site not found", logKeySiteID, siteID)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Error(fmt.Sprintf("write err resp: %v", err))
			return
		}
	}

	speciesPolicies, ok := site.policies[cp.Species]
	if ok && !isEmpty(speciesPolicies) {
		p := speciesPolicies[len(speciesPolicies)-1]
		m.log.Info("policies already exist", "count", len(speciesPolicies), logKeySpecies, cp.Species)
		resp, _ := proto.MsgError{
			Message: fmt.Sprintf(
				"%q policy already exists for species %q",
				p.Action,
				p.Species),
		}.MarshalBinary()

		if _, err := conn.Write(resp); err != nil {
			m.log.Error(fmt.Sprintf("write err resp: %v", err))
			return
		}
		return
	}

	policyID := site.nextPolicyID()
	site.policies[cp.Species] = []pestcontrol.Policy{{
		ID:        policyID,
		Species:   cp.Species,
		Action:    pestcontrol.PolicyAction(cp.Action),
		CreatedAt: time.Now(),
	}}

	resp, _ := proto.MsgPolicyResult{
		PolicyID: policyID,
	}.MarshalBinary()
	if _, err := conn.Write(resp); err != nil {
		m.log.Error(fmt.Sprintf("write policy result: %v", err))
		return
	}

	// Notify the policy change
	m.policyChange <- struct{}{}
}

func (m *MockAuthorityServer) handleDeletePolicy(conn net.Conn, siteID, policyID uint32) {
	// Make sure we already have the site in our map
	site, ok := m.sites[siteID]
	if !ok {
		m.log.Info("site %d not found", logKeySiteID, siteID)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Error(fmt.Sprintf("write err resp: %v", err))
		}
	}

	// TODO: Fix this structure for lookup by policy ID
	for i, speciesPolicies := range site.policies {
		for j, p := range speciesPolicies {
			if p.ID == policyID && p.DeletedAt.IsZero() {
				p.DeletedAt = time.Now()
				m.log.Info("policy deleted", logKeyPolicy, policyID)
				speciesPolicies[j] = p
				site.policies[i] = speciesPolicies
				resp, _ := proto.MsgOK{}.MarshalBinary()
				if _, err := conn.Write(resp); err != nil {
					m.log.Error(fmt.Sprintf("write ok resp: %v", err))
				}
				return
			}
		}
	}
	resp, _ := proto.MsgError{Message: pestcontrol.ErrPolicyNotFound.Error()}.MarshalBinary()
	if _, err := conn.Write(resp); err != nil {
		m.log.Error(fmt.Sprintf("write err resp: %v", err))
		return
	}

	// Notify the policy change
	m.policyChange <- struct{}{}
}

func (s *site) nextPolicyID() uint32 {
	s.policyID++
	return s.policyID
}

func isEmpty(p []pestcontrol.Policy) bool {
	if len(p) == 0 {
		return true
	}
	for _, v := range p {
		if v.DeletedAt.IsZero() {
			return false
		}
	}
	return true
}

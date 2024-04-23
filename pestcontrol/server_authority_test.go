package pestcontrol_test

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

// MockAuthorityServer is a mock implementation of the Authority server.
type MockAuthorityServer struct {
	name string
	l    net.Listener
	log  log.Logger

	// sites is a map of site IDs to sites.
	sites map[uint32]site
	// policyID is the next policy ID to assign for newly created policies.
	policyID uint32
	// policyChange is a channel sends a message when a policy is created or deleted.
	policyChange chan struct{}
}

type site struct {
	id                uint32
	targetPopulations map[string]pestcontrol.TargetPopulation
	// policies is a map of species names to a slice of Policy structs. Each species should settle on a single policy, but can have multiple policies in a transient state. We're using the slice to here to ensure that the final state settles on a single policy. A map would force us to overwrite the previous policy, potentially missing a bug in the system.
	policies map[string][]pestcontrol.Policy
}

func NewMockAuthorityServer() *MockAuthorityServer {
	return &MockAuthorityServer{
		name:         "Mock Authority",
		sites:        map[uint32]site{},
		log:          *log.Default(),
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
		m.log.Printf("[%s] waiting for message\n", m.name)
		var msg proto.Message
		if _, err := msg.ReadFrom(conn); err != nil {
			m.log.Printf("[%s] read message from client: %v\n", m.name, err)
			return
		}

		m.log.Printf("[%s] incoming %q msg\n", m.name, msg.Type)

		switch msg.Type {
		case proto.MsgTypeHello:
			resp, _ := proto.MsgHello{}.MarshalBinary()
			if _, err := conn.Write(resp); err != nil {
				m.log.Printf("[%s] write hello: %v", m.name, err)
			}

		case proto.MsgTypeDialAuthority:
			da, err := msg.ToMsgDialAuthority()
			if err != nil {
				m.log.Printf("[%s] to dial authority: %v", m.name, err)
				return
			}
			m.handleDialAuthority(conn, da)
			siteID = da.Site
			m.log.Printf("[%s] connection associated with site %d\n", m.name, da.Site)

		case proto.MsgTypeCreatePolicy:
			cp, err := msg.ToMsgCreatePolicy()
			if err != nil {
				m.log.Printf("[%s] to create policy: %v", m.name, err)
				return
			}
			m.handleCreatePolicy(conn, siteID, cp)

		case proto.MsgTypeDeletePolicy:
			dp, err := msg.ToMsgDeletePolicy()
			if err != nil {
				m.log.Printf("[%s] to delete policy: %v", m.name, err)
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
		m.log.Printf("[%s] site %d not found\n", m.name, da.Site)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Printf("[%s] write err resp: %v", m.name, err)
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
		m.log.Printf("[%s] write target populations: %v", m.name, err)
	}
}

func (m *MockAuthorityServer) handleCreatePolicy(conn net.Conn, siteID uint32, cp proto.MsgCreatePolicy) {
	// Make sure we already have the site in our map
	site, ok := m.sites[siteID]
	if !ok {
		m.log.Printf("[%s] site %d not found\n", m.name, siteID)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Printf("[%s] write err resp: %v", m.name, err)
			return
		}
	}

	speciesPolicies, ok := site.policies[cp.Species]
	if ok && !isEmpty(speciesPolicies) {
		p := speciesPolicies[len(speciesPolicies)-1]
		m.log.Printf("[%s] %d policies already exist for species %s\n", m.name, len(speciesPolicies), cp.Species)
		resp, _ := proto.MsgError{
			Message: fmt.Sprintf(
				"%q policy already exists for species %q",
				p.Action,
				p.Species),
		}.MarshalBinary()

		if _, err := conn.Write(resp); err != nil {
			m.log.Printf("[%s] write err resp: %v", m.name, err)
			return
		}
		return
	}

	policyID := m.nextPolicyID()
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
		m.log.Printf("[%s] write policy result: %v", m.name, err)
		return
	}

	// Notify the policy change
	m.policyChange <- struct{}{}
}

func (m *MockAuthorityServer) handleDeletePolicy(conn net.Conn, siteID, policyID uint32) {
	// Make sure we already have the site in our map
	site, ok := m.sites[siteID]
	if !ok {
		m.log.Printf("[%s] site %d not found\n", m.name, siteID)
		resp, _ := proto.MsgError{Message: pestcontrol.ErrSiteNotFound.Error()}.MarshalBinary()
		if _, err := conn.Write(resp); err != nil {
			m.log.Printf("[%s] write err resp: %v", m.name, err)
		}
	}

	// TODO: Fix this structure for lookup by policy ID
	for i, speciesPolicies := range site.policies {
		for j, p := range speciesPolicies {
			if p.ID == policyID && p.DeletedAt.IsZero() {
				p.DeletedAt = time.Now()
				m.log.Printf("[%s] policy %d deleted\n", m.name, policyID)
				speciesPolicies[j] = p
				site.policies[i] = speciesPolicies
				resp, _ := proto.MsgOK{}.MarshalBinary()
				if _, err := conn.Write(resp); err != nil {
					m.log.Printf("[%s] write ok resp: %v", m.name, err)
				}
				return
			}
		}
	}
	resp, _ := proto.MsgError{Message: pestcontrol.ErrPolicyNotFound.Error()}.MarshalBinary()
	if _, err := conn.Write(resp); err != nil {
		m.log.Printf("[%s] write err resp: %v", m.name, err)
		return
	}
}

func (m *MockAuthorityServer) nextPolicyID() uint32 {
	m.policyID++
	return m.policyID
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

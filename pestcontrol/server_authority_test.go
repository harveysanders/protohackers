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
	sites map[uint32]pestcontrol.Site
	// policyID is the next policy ID to assign for newly created policies.
	policyID uint32
	// policyChange is a channel sends a message when a policy is created or deleted.
	policyChange chan struct{}
}

func NewMockAuthorityServer() *MockAuthorityServer {
	return &MockAuthorityServer{
		name:         "Mock Authority",
		sites:        map[uint32]pestcontrol.Site{},
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
	var siteID uint32

	for {
		var msg proto.Message
		if _, err := msg.ReadFrom(conn); err != nil {
			m.log.Printf("[%s] read message from client: %v\n", m.name, err)
			return
		}

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
	pop := make([]proto.PopulationTarget, len(site.TargetPopulations))
	for species, p := range site.TargetPopulations {
		pop = append(pop, proto.PopulationTarget{
			Species: species,
			Min:     p.Min,
			Max:     p.Max,
		})
	}

	msg := proto.MsgTargetPopulations{
		Site:        site.ID,
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
		}
	}

	speciesPolicy, ok := site.Policies[cp.Species]
	if ok {
		m.log.Printf("[%s] policy for species %s already exists\n", m.name, cp.Species)
		resp, _ := proto.MsgError{
			Message: fmt.Sprintf(
				"%q policy already exists for species %q",
				speciesPolicy.Action,
				speciesPolicy.Species),
		}.MarshalBinary()

		if _, err := conn.Write(resp); err != nil {
			m.log.Printf("[%s] write err resp: %v", m.name, err)
			return
		}
		return
	}

	site.Policies[cp.Species] = pestcontrol.Policy{
		ID:        m.nextPolicyID(),
		Species:   cp.Species,
		Action:    pestcontrol.PolicyAction(cp.Action),
		CreatedAt: time.Now(),
	}

	resp, _ := proto.MsgPolicyResult{
		PolicyID: site.Policies[cp.Species].ID,
	}.MarshalBinary()
	if _, err := conn.Write(resp); err != nil {
		m.log.Printf("[%s] write policy result: %v", m.name, err)
		return
	}

	// Notify the policy change
	m.policyChange <- struct{}{}
}

func (m *MockAuthorityServer) nextPolicyID() uint32 {
	m.policyID++
	return m.policyID
}

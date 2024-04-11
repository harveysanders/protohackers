package pestcontrol

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type contextKey string

const ctxKeyConnectionID = contextKey("github.com/harveysanders/protohackers/pestcontrol:connection_ID")

var (
	errServerError = errors.New("server error")
)

type ServerConfig struct {
	AuthServerAddr string
}

// A collection of data for connecting to the Authority Server.
type AuthorityServer struct {
	addr string

	// mu protects the connections map.
	mu sync.RWMutex
	// sites is a map of siteID to Client sites.
	sites map[uint32]Client
}

type Store interface {
	AddSite(ctx context.Context, site Site) error
	GetSite(ctx context.Context, siteID uint32) (Site, error)
	SetPolicy(ctx context.Context, policyID uint32, siteID uint32, species string, action PolicyAction) error
	GetPolicy(ctx context.Context, siteID uint32, species string) (Policy, error)
	DeletePolicy(ctx context.Context, siteID uint32, species string) (Policy, error)
	SetTargetPopulations(ctx context.Context, siteID uint32, pops []TargetPopulation) error
	RecordObservation(ctx context.Context, o Observation) error
	GetObservation(ctx context.Context, siteID uint32, species string) (Observation, error)
}

type Server struct {
	authSrv   *AuthorityServer
	logger    *log.Logger
	siteStore Store
}

func NewServer(logger *log.Logger, config ServerConfig, siteStore Store) *Server {
	authSrv := &AuthorityServer{
		addr: config.AuthServerAddr,
		// TODO: Figure out a good initial capacity for the sites map.
		sites: make(map[uint32]Client, 200),
	}

	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		authSrv:   authSrv,
		logger:    logger,
		siteStore: siteStore,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	return s.Serve(ln)
}

func (s *Server) Serve(l net.Listener) error {
	var connID uint32
	for {
		connID++
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		ctx := context.WithValue(context.Background(), ctxKeyConnectionID, connID)
		go s.handleClient(ctx, conn)
	}
}

func (s *Server) Close() error {
	errs := make([]error, 0, len(s.authSrv.sites))

	s.authSrv.mu.Lock()
	for _, c := range s.authSrv.sites {
		errs = append(errs, c.Close())
	}
	s.authSrv.mu.Unlock()

	return errors.Join(errs...)
}

func (s *Server) handleClient(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	connID, ok := ctx.Value(ctxKeyConnectionID).(uint32)
	if !ok {
		s.logger.Println("invalid connection ID in context")
		return
	}

	s.logger.Printf("client [%d] connected!\n", connID)

	for {
		var msg proto.Message
		if _, err := msg.ReadFrom(conn); err != nil {
			s.logger.Printf("[%d] read message from client: %v\n", connID, err)
			return
		}

		switch msg.Type {
		case proto.MsgTypeHello:
			s.logger.Printf("[%d]: MsgTypeHello\n", connID)
			_, err := msg.ToMsgHello()
			if err != nil {
				s.logger.Printf("msg.ToMsgHello: %v\n", err)
				s.sendErrorResp(conn, errors.New("invalid hello message"))
				return
			}
			if err := s.handleHello(ctx, conn); err != nil {
				s.logger.Printf("handleHello: %v\n", err)
				s.sendErrorResp(conn, errServerError)
				return
			}
		case proto.MsgTypeError:
			s.logger.Printf("[%d]: MsgTypeError\n", connID)
		case proto.MsgTypeOK:
			s.logger.Printf("[%d]: MsgTypeOK\n", connID)
		case proto.MsgTypeSiteVisit:
			s.logger.Printf("[%d]: MsgTypeSiteVisit\n", connID)
			sv, err := msg.ToMsgSiteVisit()
			if err != nil {
				s.logger.Printf("msg.ToMsgSiteVisit: %v\n", err)
				s.sendErrorResp(conn, errors.New("invalid site visit message"))
				return
			}
			if err := s.handleSiteVisit(ctx, sv); err != nil {
				s.logger.Printf("handleSiteVisit: %v\n", err)
				s.sendErrorResp(conn, errServerError)
				return
			}
		default:
			if msg.Type == 0 {
				return
			}
			s.logger.Printf("[%d]: unknown message type %x\n", connID, msg.Type)
			s.sendErrorResp(conn, errors.New("unknown message type"))
			return
		}
	}
}

func (s *Server) sendErrorResp(conn net.Conn, err error) {
	resp := proto.MsgError{Message: err.Error()}
	data, err := resp.MarshalBinary()
	if err != nil {
		s.logger.Printf("error.MarshalBinary: %v\n", err)
		return
	}
	if _, err := conn.Write(data); err != nil {
		s.logger.Printf("error resp write: %v\n", err)
		return
	}
}

func (s *Server) handleHello(ctx context.Context, conn net.Conn) error {
	if ctx.Done() != nil {
		return nil
	}
	resp := proto.MsgHello{}
	data, err := resp.MarshalBinary()
	if err != nil {
		return fmt.Errorf("hello.MarshalBinary: %w", err)

	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write hello resp: %w", err)
	}
	return nil
}

func (s *Server) handleSiteVisit(ctx context.Context, observation proto.MsgSiteVisit) error {
	if ctx.Done() != nil {
		return nil
	}

	clientID, ok := ctx.Value(ctxKeyConnectionID).(uint32)
	if !ok {
		return fmt.Errorf("invalid connection ID in context")
	}

	if err := validateSiteVisit(observation); err != nil {
		return fmt.Errorf("observation.validate: %w", err)
	}

	// Check if the server already has a connection to the specified site.
	s.authSrv.mu.RLock()
	siteClient, ok := s.authSrv.sites[observation.Site]
	s.authSrv.mu.RUnlock()

	// if not, create a new connection and get the target populations.
	if !ok {
		siteClient = Client{}
		resp, err := siteClient.establishSiteConnection(ctx, s.authSrv.addr, observation.Site)
		if err != nil {
			return fmt.Errorf("establishSiteConnection: %w", err)
		}

		s.logger.Printf("received %d target populations from site %d\n", len(resp.Populations), resp.Site)
		pops := make([]TargetPopulation, 0, len(resp.Populations))

		for _, v := range resp.Populations {
			pops = append(pops, TargetPopulation{
				Species: v.Species,
				Min:     v.Min,
				Max:     v.Max,
			})
		}
		if err := s.siteStore.SetTargetPopulations(ctx, observation.Site, pops); err != nil {
			return fmt.Errorf("SetTargetPopulations: %w", err)
		}

		s.authSrv.mu.Lock()
		s.authSrv.sites[observation.Site] = siteClient
		s.authSrv.mu.Unlock()
	}

	// Get the persisted target populations for the site+species.
	site, err := s.siteStore.GetSite(ctx, observation.Site)
	if err != nil {
		return fmt.Errorf("GetSite: %w", err)
	}

	for _, observed := range observation.Populations {
		err := s.siteStore.RecordObservation(ctx, Observation{
			Site:     observation.Site,
			Species:  observed.Species,
			Count:    observed.Count,
			ClientID: clientID,
		})
		if err != nil {
			return fmt.Errorf("RecordObservation: %w", err)
		}
	}

	for speciesName, target := range site.TargetPopulations {
		// Get the latest observation for the species.
		observed, err := s.siteStore.GetObservation(ctx, observation.Site, speciesName)
		if err != nil {
			if !errors.Is(err, ErrObservationNotFound) {
				return fmt.Errorf("GetObservation: %w", err)
			}
			// If the species is not observed, assume count is 0 and create the appropriate policy.
			// Use go's zero value for uint32 as the count.
		}

		// Check if the observed population is within the target range.
		if observed.Count < target.Min {
			s.logger.Printf("(site: %d)\nspecies %q population is below target range\n", observation.Site, speciesName)
			if err := s.setPolicy(ctx, siteClient, speciesName, Conserve); err != nil {
				return fmt.Errorf("setPolicy: %w", err)
			}
		}

		if observed.Count > target.Max {
			s.logger.Printf("(site: %d)\nspecies %q population is above target range\n", observation.Site, speciesName)

			if err := s.setPolicy(ctx, siteClient, speciesName, Cull); err != nil {
				return fmt.Errorf("setPolicy: %w", err)
			}
		}

		if target.Min <= observed.Count && observed.Count <= target.Max {
			s.logger.Printf("(site: %d)\nspecies %q population is within target range\n", observation.Site, speciesName)
			if err := s.deletePolicy(ctx, siteClient, speciesName); err != nil {
				return fmt.Errorf("deletePolicy: %w", err)
			}
		}
	}
	return nil
}

func (s *Server) setPolicy(ctx context.Context, siteClient Client, speciesName string, action PolicyAction) error {
	// Check if we've already set a policy for the species.
	existing, err := s.siteStore.GetPolicy(ctx, *siteClient.siteID, speciesName)
	if err != nil {
		if !errors.Is(err, ErrPolicyNotFound) {
			return fmt.Errorf("GetPolicy: %w", err)
		}
		// Policy not found, create a new one.
		resp, err := siteClient.createPolicy(speciesName, proto.PolicyAction(action))
		if err != nil {
			return fmt.Errorf("createPolicy: %w", err)
		}

		s.logger.Printf("new policy (id, %d) for species %q set to %q\n", resp.Policy, speciesName, action.String())
		if err := s.siteStore.SetPolicy(ctx, resp.Policy, *siteClient.siteID, speciesName, Conserve); err != nil {
			return fmt.Errorf("SetPolicy: %w", err)
		}
		return nil
	}
	// policy exists,
	//  update it.
	if existing.Action != action {
		if err := s.deletePolicy(ctx, siteClient, speciesName); err != nil {
			return fmt.Errorf("deletePolicy: %w", err)
		}
		resp, err := siteClient.createPolicy(speciesName, proto.PolicyAction(action))
		if err != nil {
			return fmt.Errorf("createPolicy: %w", err)
		}
		s.logger.Printf("new policy (id, %d) for species %q set to %q\n", resp.Policy, speciesName, action.String())
		if err := s.siteStore.SetPolicy(ctx, resp.Policy, *siteClient.siteID, speciesName, Conserve); err != nil {
			return fmt.Errorf("SetPolicy: %w", err)
		}
		return nil
	}

	s.logger.Printf("existing policy for species %q, set to %q updating to %q\n", speciesName, existing.Action.String(), action.String())
	return nil
}

func (s *Server) deletePolicy(ctx context.Context, siteClient Client, speciesName string) error {
	p, err := s.siteStore.DeletePolicy(ctx, *siteClient.siteID, speciesName)
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return nil
		}
		return fmt.Errorf("DeletePolicy: %w", err)
	}
	return siteClient.deletePolicy(p.ID)
}

// validateSiteVisit checks if the populations fields contain multiple conflicting counts for the same species. Non-conflicting duplicates are allowed.
func validateSiteVisit(sv proto.MsgSiteVisit) error {
	speciesCounts := make(map[string]uint32, len(sv.Populations))
	for _, p := range sv.Populations {
		if count, ok := speciesCounts[p.Species]; ok {
			if count != p.Count {
				return fmt.Errorf("conflicting counts for species %q, ([%d], [%d])", p.Species, count, p.Count)
			}
		}
		speciesCounts[p.Species] = p.Count
	}
	return nil
}

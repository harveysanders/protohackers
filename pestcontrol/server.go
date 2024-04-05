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
	SetPolicy(ctx context.Context, siteID uint32, species string, action PolicyAction) error
	GetPolicy(ctx context.Context, siteID uint32, species string) (Policy, error)
	DeletePolicy(ctx context.Context, siteID uint32, species string) (Policy, error)
	SetTargetPopulations(ctx context.Context, siteID uint32, pops []TargetPopulation) error
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
	var connID int32
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

	connID, ok := ctx.Value(ctxKeyConnectionID).(int32)
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
	// Check if the server already has a connection to the specified site.
	s.authSrv.mu.RLock()
	siteClient, ok := s.authSrv.sites[observation.Site]
	s.authSrv.mu.RUnlock()

	// if not, create a new connection and get the target populations.
	if !ok {
		siteClient = Client{}
		popResp, err := siteClient.establishSiteConnection(ctx, s.authSrv.addr, observation.Site)
		if err != nil {
			return fmt.Errorf("establishSiteConnection: %w", err)
		}

		s.logger.Printf("received %d target populations from site %d\n", len(popResp.Populations), popResp.Site)
		pops := make([]TargetPopulation, 0, len(popResp.Populations))

		for _, v := range popResp.Populations {
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
		target, ok := site.TargetPopulations[observed.Species]
		if !ok {
			s.logger.Printf("(site: %d)\nspecies %q not found in target populations\n", observation.Site, observed.Species)
			continue
		}

		// Check if the observed population is within the target range.
		if observed.Count < target.Min {
			s.logger.Printf("(site: %d)\nspecies %q population is below target range\n", observation.Site, observed.Species)
			if err := s.siteStore.SetPolicy(ctx, observation.Site, observed.Species, Conserve); err != nil {
				return fmt.Errorf("SetPolicy: %w", err)
			}
			return siteClient.createPolicy(observed.Species, proto.Conserve)
		}

		if observed.Count > target.Max {
			s.logger.Printf("(site: %d)\nspecies %q population is above target range\n", observation.Site, observed.Species)
			if err := s.siteStore.SetPolicy(ctx, observation.Site, observed.Species, Cull); err != nil {
				return fmt.Errorf("SetPolicy: %w", err)
			}
			return siteClient.createPolicy(observed.Species, proto.Cull)
		}

		if target.Min <= observed.Count && observed.Count <= target.Max {
			s.logger.Printf("(site: %d)\nspecies %q population is within target range\n", observation.Site, observed.Species)
			p, err := s.siteStore.DeletePolicy(ctx, observation.Site, observed.Species)
			if err != nil {
				if errors.Is(err, ErrPolicyNotFound) {
					continue
				}
				return fmt.Errorf("DeletePolicy: %w", err)
			}
			return siteClient.deletePolicy(p.ID)
		}
	}
	return nil
}

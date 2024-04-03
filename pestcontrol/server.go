package pestcontrol

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/harveysanders/protohackers/pestcontrol/inmem"
	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type contextKey string

const ctxKeyConnectionID = contextKey("github.com/harveysanders/protohackers/pestcontrol:connection_ID")

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

type Server struct {
	authSrv   *AuthorityServer
	logger    *log.Logger
	siteStore inmem.Store
}

func NewServer(logger *log.Logger, config ServerConfig) *Server {
	authSrv := &AuthorityServer{
		addr: config.AuthServerAddr,
	}

	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		authSrv: authSrv,
		logger:  logger,
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
	connID, ok := ctx.Value(ctxKeyConnectionID).(int32)
	if !ok {
		s.logger.Println("invalid connection ID in context")
		return
	}

	s.logger.Printf("client [%d] connected!\n", connID)
	var msg proto.Message
	if _, err := msg.ReadFrom(conn); err != nil {
		s.logger.Printf("[%d] read message from client: %v\n", connID, err)
	}

	switch msg.Type {
	case proto.MsgTypeHello:
		s.logger.Printf("[%d]: MsgTypeHello\n", connID)
	case proto.MsgTypeError:
		s.logger.Printf("[%d]: MsgTypeError\n", connID)
	case proto.MsgTypeOK:
		s.logger.Printf("[%d]: MsgTypeOK\n", connID)
	case proto.MsgTypeSiteVisit:
		s.logger.Printf("MsgTypeSiteVisit\n")
		sv, err := msg.ToMsgSiteVisit()
		if err != nil {
			s.logger.Printf("msg.ToMsgSiteVisit: %v\n", err)
		}
		s.handleSiteVisit(ctx, sv)
	default:
		s.logger.Printf("[%d]: unknown message type %x\n", connID, msg.Type)
	}
}

func (s *Server) handleSiteVisit(ctx context.Context, observation proto.MsgSiteVisit) {
	if ctx.Done() != nil {
		return
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
			s.logger.Printf("establishSiteConnection: %v\n", err)
			return
		}

		s.logger.Printf("received target populations: %+v\n", popResp)
		// TODO: Persist the target populations for the site.

		s.authSrv.mu.Lock()
		s.authSrv.sites[observation.Site] = siteClient
		s.authSrv.mu.Unlock()
	}

	// Get the persisted target populations for the site+species.
	site, err := s.siteStore.GetSite(observation.Site)
	if err != nil {
		s.logger.Printf("GetSite: %v\n", err)
		return
	}
	for _, observed := range observation.Populations {
		target, ok := site.TargetPopulations[observed.Species]
		if !ok {
			s.logger.Printf("species %q not found in target populations\n", observed.Species)
			continue
		}

		// Check if the observed population is within the target range.
		if observed.Count < target.Min {
			s.logger.Printf("species %q population is below target range\n", observed.Species)
			if err := s.siteStore.SetPolicy(observation.Site, observed.Species, inmem.Conserve); err != nil {
				s.logger.Printf("SetPolicy: %v\n", err)
				continue
			}
		}
		if observed.Count > target.Max {
			s.logger.Printf("species %q population is above target range\n", observed.Species)
			if err := s.siteStore.SetPolicy(observation.Site, observed.Species, inmem.Cull); err != nil {
				s.logger.Printf("SetPolicy: %v\n", err)
				continue
			}
		}
		s.logger.Printf("CreatePolicy: %v\n", err)
		if target.Min <= observed.Count && observed.Count <= target.Max {
			s.logger.Printf("species %q population is within target range\n", observed.Species)
			if err := s.siteStore.DeletePolicy(observation.Site, observed.Species); err != nil {
				s.logger.Printf("DeletePolicy: %v\n", err)
				continue
			}
		}
	}
}

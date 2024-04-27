package pestcontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type contextKey string

const ctxKeyConnectionID = contextKey("github.com/harveysanders/protohackers/pestcontrol:connection_ID")

const (
	logKeyMsgType      = "type"
	logKeyPolicy       = "policy"
	logKeyPolicyAction = "action"
	logKeySiteID       = "site"
	logKeySpecies      = "species"
)

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
	DeletePolicy(ctx context.Context, siteID uint32, policyID uint32) (Policy, error)
	SetTargetPopulations(ctx context.Context, siteID uint32, pops []TargetPopulation) error
	RecordObservation(ctx context.Context, o Observation) error
	GetObservation(ctx context.Context, siteID uint32, species string) (Observation, error)
}

type contextHandler struct {
	slog.Handler
	keys []contextKey
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, k := range h.keys {
		attr, ok := ctx.Value(k).(slog.Attr)
		if !ok {
			continue
		}
		r.AddAttrs(attr)
	}
	return h.Handler.Handle(ctx, r)
}

type Server struct {
	authSrv   *AuthorityServer
	logger    *slog.Logger
	siteStore Store
}

func NewServer(logger *slog.Logger, config ServerConfig, siteStore Store) *Server {
	authSrv := &AuthorityServer{
		addr: config.AuthServerAddr,
		// TODO: Figure out a good initial capacity for the sites map.
		sites: make(map[uint32]Client, 200),
	}

	ctxHandler := contextHandler{
		slog.NewTextHandler(os.Stderr, nil),
		[]contextKey{ctxKeyConnectionID},
	}
	if logger == nil {
		logger = slog.New(ctxHandler).With("name", "PestcontrolServer")
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

	s.logger.InfoContext(ctx, "client connected!")

	for {
		s.logger.InfoContext(ctx, "waiting for message...")
		var msg proto.Message
		if _, err := msg.ReadFrom(conn); err != nil {
			s.logger.ErrorContext(ctx, fmt.Sprintf("read message from client: %v", err))
			return
		}

		msgLogger := s.logger.With(logKeyMsgType, msg.Type)
		switch msg.Type {
		case proto.MsgTypeHello:
			msgLogger.InfoContext(ctx, "msg")
			_, err := msg.ToMsgHello()
			if err != nil {
				msgLogger.ErrorContext(ctx, fmt.Sprintf("msg.ToMsgHello: %v", err))
				s.sendErrorResp(ctx, conn, errors.New("invalid hello message"))
				return
			}
			if err := s.handleHello(ctx, conn); err != nil {
				msgLogger.ErrorContext(ctx, fmt.Sprintf("handleHello: %v", err))
				s.sendErrorResp(ctx, conn, errServerError)
				return
			}
		case proto.MsgTypeError:
			msgLogger.InfoContext(ctx, "msg")
		case proto.MsgTypeOK:
			msgLogger.InfoContext(ctx, "msg")
		case proto.MsgTypeSiteVisit:
			msgLogger.InfoContext(ctx, "msg")
			sv, err := msg.ToMsgSiteVisit()
			if err != nil {
				msgLogger.ErrorContext(ctx, fmt.Sprintf("msg.ToMsgSiteVisit: %v", err))
				s.sendErrorResp(ctx, conn, errors.New("invalid site visit message"))
				return
			}
			if err := s.handleSiteVisit(ctx, sv); err != nil {
				msgLogger.ErrorContext(ctx, fmt.Sprintf("handleSiteVisit: %v", err))
				s.sendErrorResp(ctx, conn, errServerError)
				return
			}
		default:
			if msg.Type == 0 {
				return
			}
			msgLogger.InfoContext(ctx, "unknown message type")
			s.sendErrorResp(ctx, conn, errors.New("unknown message type"))
			return
		}
	}
}

func (s *Server) sendErrorResp(ctx context.Context, conn net.Conn, err error) {
	resp := proto.MsgError{Message: err.Error()}
	data, err := resp.MarshalBinary()
	if err != nil {
		s.logger.ErrorContext(ctx, fmt.Sprintf("error.MarshalBinary: %v", err))
		return
	}
	if _, err := conn.Write(data); err != nil {
		s.logger.ErrorContext(ctx, fmt.Sprintf("error resp write: %v", err))
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

	siteLogger := s.logger.With(logKeySiteID, observation.SiteID)

	if err := validateSiteVisit(observation); err != nil {
		return fmt.Errorf("observation.validate: %w", err)
	}

	// Check if the server already has a connection to the specified site.
	s.authSrv.mu.RLock()
	siteClient, ok := s.authSrv.sites[observation.SiteID]
	s.authSrv.mu.RUnlock()

	// if not, create a new connection and get the target populations.
	if !ok {
		siteClient = Client{}
		resp, err := siteClient.establishSiteConnection(ctx, s.authSrv.addr, observation.SiteID)
		if err != nil {
			return fmt.Errorf("establishSiteConnection: %w", err)
		}

		siteLogger.InfoContext(ctx, fmt.Sprintf("received %d target populations", len(resp.Populations)))
		pops := make([]TargetPopulation, 0, len(resp.Populations))

		for _, v := range resp.Populations {
			pops = append(pops, TargetPopulation{
				Species: v.Species,
				Min:     v.Min,
				Max:     v.Max,
			})
		}
		if err := s.siteStore.SetTargetPopulations(ctx, observation.SiteID, pops); err != nil {
			return fmt.Errorf("SetTargetPopulations: %w", err)
		}

		s.authSrv.mu.Lock()
		s.authSrv.sites[observation.SiteID] = siteClient
		s.authSrv.mu.Unlock()
	}

	// Get the persisted target populations for the site+species.
	site, err := s.siteStore.GetSite(ctx, observation.SiteID)
	if err != nil {
		return fmt.Errorf("GetSite: %w", err)
	}

	clientID, ok := ctx.Value(ctxKeyConnectionID).(uint32)
	if !ok {
		siteLogger.ErrorContext(ctx, "client ID not found in context")
	}
	for _, observed := range observation.Populations {
		err := s.siteStore.RecordObservation(ctx, Observation{
			Site:     observation.SiteID,
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
		observed, err := s.siteStore.GetObservation(ctx, observation.SiteID, speciesName)
		if err != nil {
			if !errors.Is(err, ErrObservationNotFound) {
				return fmt.Errorf("GetObservation: %w", err)
			}
			// If the species is not observed, assume count is 0 and create the appropriate policy.
			// Use go's zero value for uint32 as the count.
		}

		// Check if the observed population is within the target range.
		if observed.Count < target.Min {
			siteLogger.InfoContext(ctx, "population is below target range", logKeySpecies, speciesName)
			if err := s.setPolicy(ctx, siteClient, speciesName, Conserve); err != nil {
				return fmt.Errorf("setPolicy: %w", err)
			}
		}

		if observed.Count > target.Max {
			siteLogger.InfoContext(ctx, "population is above target range", logKeySpecies, speciesName)

			if err := s.setPolicy(ctx, siteClient, speciesName, Cull); err != nil {
				return fmt.Errorf("setPolicy: %w", err)
			}
		}

		if target.Min <= observed.Count && observed.Count <= target.Max {
			siteLogger.InfoContext(ctx, "population is within target range", logKeySpecies, speciesName)
			p, err := s.siteStore.GetPolicy(ctx, *siteClient.siteID, speciesName)
			if err != nil {
				if !errors.Is(err, ErrPolicyNotFound) {
					return fmt.Errorf("siteStore.GetPolicy: %w", err)
				}
				return nil
			}

			if err := s.deletePolicy(ctx, siteClient, p.ID); err != nil {
				return fmt.Errorf("deletePolicy (site: %d, policy: %d): %w", *siteClient.siteID, p.ID, err)
			}
		}
	}
	return nil
}

func (s *Server) setPolicy(ctx context.Context, siteClient Client, speciesName string, action PolicyAction) error {
	siteLogger := s.logger.With(logKeySiteID, *siteClient.siteID)
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

		siteLogger.InfoContext(ctx, "new policy",
			logKeyPolicy, resp.PolicyID,
			logKeySpecies, speciesName,
			logKeyPolicyAction, action.String())
		if err := s.siteStore.SetPolicy(ctx, resp.PolicyID, *siteClient.siteID, speciesName, action); err != nil {
			return fmt.Errorf("SetPolicy: %w", err)
		}
		return nil
	}
	// policy exists,
	//  update it.
	if existing.Action != action {
		if err := s.deletePolicy(ctx, siteClient, existing.ID); err != nil {
			return fmt.Errorf("deletePolicy (site: %d, policy: %d): %w", *siteClient.siteID, existing.ID, err)
		}
		resp, err := siteClient.createPolicy(speciesName, proto.PolicyAction(action))
		if err != nil {
			return fmt.Errorf("createPolicy: %w", err)
		}
		siteLogger.InfoContext(ctx, fmt.Sprintf("switching policy to (%d - %s)", resp.PolicyID, action.String()),
			logKeyPolicy, existing.ID,
			logKeyPolicyAction, existing.Action.String(),
			logKeySpecies, speciesName)

		if err := s.siteStore.SetPolicy(ctx, resp.PolicyID, *siteClient.siteID, speciesName, Conserve); err != nil {
			return fmt.Errorf("SetPolicy: %w", err)
		}
		return nil
	}

	siteLogger.InfoContext(ctx, "existing policy, no action", logKeySpecies, speciesName, logKeyPolicyAction, existing.Action.String())
	return nil
}

// deletePolicy deletes a site's policy from the Authority server and the Pestcontrol DB.
func (s *Server) deletePolicy(ctx context.Context, siteClient Client, id uint32) error {
	s.logger.InfoContext(ctx, "deleting policy",
		logKeySiteID, *siteClient.siteID,
		logKeyPolicy, id)
	if err := siteClient.deletePolicy(id); err != nil {
		return fmt.Errorf("authority server deletePolicy: %w", err)
	}
	if _, err := s.siteStore.DeletePolicy(ctx, *siteClient.siteID, id); err != nil {
		return fmt.Errorf("siteStore.DeletePolicy: %w", err)
	}
	return nil
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

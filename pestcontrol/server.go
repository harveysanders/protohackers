package pestcontrol

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

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
	// Client for connecting to the Authority (not Authz/Authn) server.
	client Client
}

type Server struct {
	authSrv AuthorityServer
	logger  *log.Logger
}

func NewServer(logger *log.Logger, config ServerConfig) *Server {
	authSrv := AuthorityServer{
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

	if err := s.authSrv.client.Connect(s.authSrv.addr); err != nil {
		return fmt.Errorf("connect to Authority server: %w", err)
	}

	if err := s.authSrv.client.sendHello(); err != nil {
		return fmt.Errorf("send HelloMsg: %w", err)
	}

	go func() {
		if err := s.handleAuthServerResponses(); err != nil {
			s.logger.Printf("Authority Server connection: %v", err)
		}
	}()

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
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) Close() error {
	return errors.Join(
		s.authSrv.client.Close(),
	)
}

// handleAuthServerResponses unmarshals response messages from the Authority server and passes them to the appropriate handler, based on message type.
func (s *Server) handleAuthServerResponses() error {
	for {
		msg, err := s.authSrv.client.readMessage()
		if err != nil {
			// TODO Send back error response?
			return err
		}

		// TODO: Process incoming messages
		switch msg.Type {
		case proto.MsgTypeHello:
			s.logger.Printf("MsgTypeHello\n")
			hello, err := msg.ToMsgHello()
			if err != nil {
				s.logger.Printf("hello message error: %v\n", err)
				continue
			}
			s.logger.Println(hello)
		case proto.MsgTypeError:
			s.logger.Printf("MsgTypeError\n")
		case proto.MsgTypeOK:
			s.logger.Printf("MsgTypeOK\n")
		case proto.MsgTypeDialAuthority:
			s.logger.Printf("MsgTypeDialAuthority\n")
		case proto.MsgTypeTargetPopulations:
			s.logger.Printf("MsgTypeTargetPopulations\n")
		case proto.MsgTypeCreatePolicy:
			s.logger.Printf("MsgTypeCreatePolicy\n")
		case proto.MsgTypeDeletePolicy:
			s.logger.Printf("MsgTypeDeletePolicy\n")
		case proto.MsgTypePolicyResult:
			s.logger.Printf("MsgTypePolicyResult\n")
		case proto.MsgTypeSiteVisit:
			s.logger.Printf("MsgTypeSiteVisit\n")
		default:
			s.logger.Printf("unknown message type %x\n", msg.Type)
		}
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	connID, ok := ctx.Value(ctxKeyConnectionID).(int32)
	if !ok {
		s.logger.Println("invalid connection ID in context")
		return
	}

	s.logger.Printf("client [%d] connected!\n", connID)
	// TODO: Handle incoming client connections
}

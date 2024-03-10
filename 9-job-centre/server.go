package jobcentre

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/harveysanders/protohackers/9-job-centre/jcp"
)

type responseStatus string

var (
	statusOK    responseStatus = "ok"
	statusNoJob responseStatus = "no-job"
	statusError responseStatus = "error"
)

type requestType string

var (
	requestTypePut    requestType = "put"
	requestTypeGet    requestType = "get"
	requestTypeDelete requestType = "delete"
	requestTypeAbort  requestType = "abort"
)

type (
	Response struct {
		Status responseStatus   `json:"status"`          // Status message.
		ID     *uint64          `json:"id,omitempty"`    // ID of the job.
		Job    *json.RawMessage `json:"job,omitempty"`   // Job payload.
		Queue  *string          `json:"queue,omitempty"` // Name of the queue from which the job was retrieved.
		Pri    *uint64          `json:"pri,omitempty"`   // Job priority.
		Error  *string          `json:"error,omitempty"` // Error message.
	}

	GenRequest struct {
		Request requestType `json:"request"` // Type of request.
	}

	PutRequest struct {
		clientID uint64          // Unique client ID.
		Queue    string          `json:"queue"` // Queue name.
		Job      json.RawMessage // Job payload.
		Pri      uint64          // Job priority. Higher integer has higher priority.
	}

	GetRequest struct {
		clientID uint64   // Unique client ID.
		Queues   []string `json:"queues"` // Names of queues from which to retrieve the highest priority job.
		Wait     bool     `json:"wait"`   // If true, the server will wait until there is an available job to respond. If false, the server will respond with "no-job" status if there are no available jobs.
	}

	DeleteRequest struct {
		clientID uint64 // Unique client ID.
		ID       uint64 `json:"id"` // ID of the job to delete.
	}

	AbortRequest struct {
		clientID uint64 // Unique client ID.
		ID       uint64 `json:"id"` // ID of the job to abort.
	}
)

type (
	store interface {
		// AddJob adds a job to the queue.
		AddJob(ctx context.Context, clientID uint64, queueName string, pri uint64, id *uint64, payload json.RawMessage) (inmem.Job, error)

		NextJob(ctx context.Context, clientID uint64, queueNames []string) (inmem.Job, string, error)

		DeleteJob(ctx context.Context, clientID uint64, id uint64) error

		AbortJob(ctx context.Context, clientID uint64, id uint64) error
	}

	Server struct {
		log   *log.Logger
		store store
	}
)

func NewServer(store store) *Server {
	return &Server{
		store: store,
		log:   log.Default(),
	}
}

func (s *Server) ServeJCP(ctx context.Context, w jcp.JCPResponseWriter, r *jcp.Request) {
	var body GenRequest

	var bodyRdr bytes.Buffer
	tr := io.TeeReader(r.Body, &bodyRdr)
	jd := json.NewDecoder(tr)
	je := json.NewEncoder(w)

	err := jd.Decode(&body)
	if err != nil {
		errResp := errorResponse(err, "failed to decode request")
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}

	clientID, ok := ctx.Value(jcp.ContextKeyConnID).(uint64)
	if !ok {
		errMsg := "failed to get client ID from context"
		errResp := errorResponse(errors.New("internal"), errMsg)
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}
	switch requestType(body.Request) {
	case requestTypePut:
		s.log.Println("put request")
		var req PutRequest
		if err := json.Unmarshal(bodyRdr.Bytes(), &req); err != nil {
			errResp := errorResponse(err, "failed to decode PUT request")
			if err = je.Encode(errResp); err != nil {
				s.log.Printf("failed to encode error response: %v", err)
			}
			return
		}

		req.clientID = clientID
		s.put(ctx, w, &req)

	case requestTypeGet:
		s.log.Println("get request")
		var req GetRequest
		if err := json.Unmarshal(bodyRdr.Bytes(), &req); err != nil {
			errResp := errorResponse(err, "failed to decode GET request")
			if err = je.Encode(errResp); err != nil {
				s.log.Printf("failed to encode error response: %v", err)
			}
			return
		}

		req.clientID = clientID
		s.get(ctx, w, &req)

	case requestTypeDelete:
		s.log.Println("delete request")
	case requestTypeAbort:
		s.log.Println("abort request")
		var req AbortRequest
		if err := json.Unmarshal(bodyRdr.Bytes(), &req); err != nil {
			errResp := errorResponse(err, "failed to decode ABORT request")
			if err = je.Encode(errResp); err != nil {
				s.log.Printf("failed to encode error response: %v", err)
			}
			return
		}

		req.clientID = clientID
		s.abort(ctx, w, &req)
	default:
		errMsg := "unknown request type"
		errResp := Response{
			Status: statusError,
			Error:  &errMsg,
		}
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}
}

func (s *Server) put(ctx context.Context, w jcp.JCPResponseWriter, r *PutRequest) {
	job, err := s.store.AddJob(ctx, 0, r.Queue, r.Pri, nil, r.Job)
	je := json.NewEncoder(w)
	if err != nil {
		errResp := errorResponse(err)
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}

	resp := Response{
		Status: statusOK,
		ID:     &job.ID,
	}
	if err = je.Encode(resp); err != nil {
		s.log.Printf("failed to encode response: %v", err)
	}
}

func (s *Server) get(ctx context.Context, w jcp.JCPResponseWriter, r *GetRequest) {
	je := json.NewEncoder(w)
	job, queueName, err := s.store.NextJob(ctx, r.clientID, r.Queues)
	if err != nil {
		errResp := errorResponse(err)
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}
	resp := Response{
		Status: statusOK,
		ID:     &job.ID,
		Job:    &job.Payload,
		Queue:  &queueName,
		Pri:    &job.Pri,
	}
	if err = je.Encode(resp); err != nil {
		s.log.Printf("failed to encode response: %v", err)
	}
}

func (s *Server) abort(ctx context.Context, w jcp.JCPResponseWriter, r *AbortRequest) {
	je := json.NewEncoder(w)
	if err := s.store.AbortJob(ctx, r.clientID, r.ID); err != nil {
		errResp := errorResponse(err)
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}
	if err := je.Encode(Response{Status: statusOK}); err != nil {
		s.log.Printf("failed to encode response: %v", err)
	}
}

// errorResponse creates an ErrorResponse from an error. If msgs is omitted, the error's message is used.
func errorResponse(err error, msgs ...string) Response {
	if err == inmem.ErrNoJob {
		return Response{Status: statusNoJob}
	}

	errMsg := err.Error()
	if len(msgs) > 0 {
		errMsg = strings.Join(msgs, "\n")
	}

	return Response{
		Status: statusError,
		Error:  &errMsg,
	}
}

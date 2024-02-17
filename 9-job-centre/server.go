package jobcentre

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"

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
	ErrorResponse struct {
		Status responseStatus `json:"status"` // Status message. Always "error" for an ErrorResponse.
		Error  string         `json:"error"`  // Error message.
	}

	GenRequest struct {
		Request requestType `json:"request"` // Type of request.
	}

	PutRequest struct {
		Queue string          `json:"queue"` // Queue name.
		Job   json.RawMessage // Job payload.
		Pri   uint64          // Job priority. Higher integer has higher priority.
	}

	PutResponse struct {
		Status responseStatus `json:"status"`       // Status message.
		ID     *uint64        `json:"id,omitempty"` // ID of the job.
	}

	GetRequest struct {
		Queues []string `json:"queues"` // Names of queues from which to retrieve the highest priority job.
		Wait   bool     `json:"wait"`   // If true, the server will wait until there is an available job to respond. If false, the server will respond with "no-job" status if there are no available jobs.
	}

	GetResponse struct {
		Status responseStatus   // Status message.
		ID     *uint64          `json:"id,omitempty"`    // ID of the job.
		Job    *json.RawMessage `json:"job,omitempty"`   // Job payload.
		Queue  *string          `json:"queue,omitempty"` // Name of the queue from which the job was retrieved.
		Pri    *uint64          `json:"pri,omitempty"`   // Job priority.
	}

	DeleteRequest struct {
		ID uint64 `json:"id"` // ID of the job to delete.
	}

	DeleteResponse struct {
		Status responseStatus // Status message.
	}

	AbortRequest struct {
		ID uint64 `json:"id"` // ID of the job to abort.
	}
)

type (
	store interface {
		// AddJob adds a job to the queue.
		AddJob(ctx context.Context, clientID uint64, queueName string, pri uint64, payload json.RawMessage) (inmem.Job, error)
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
		errResp := ErrorResponse{
			Status: statusError,
			Error:  "failed to decode request body",
		}
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}

	switch requestType(body.Request) {
	case requestTypePut:
		s.log.Println("put request")
		var req PutRequest
		json.Unmarshal(bodyRdr.Bytes(), &req)
		s.put(ctx, w, &req)

	case requestTypeGet:
		s.log.Println("get request")

	case requestTypeDelete:
		s.log.Println("delete request")
	case requestTypeAbort:
		s.log.Println("abort request")
	default:
		errResp := ErrorResponse{
			Status: statusError,
			Error:  "unknown request type",
		}
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}
}

func (s *Server) put(ctx context.Context, w jcp.JCPResponseWriter, r *PutRequest) {
	job, err := s.store.AddJob(ctx, 0, r.Queue, r.Pri, r.Job)
	je := json.NewEncoder(w)
	if err != nil {
		errResp := newErrorResponse(err)
		if err = je.Encode(errResp); err != nil {
			s.log.Printf("failed to encode error response: %v", err)
		}
		return
	}

	resp := PutResponse{
		Status: statusOK,
		ID:     &job.ID,
	}
	if err = je.Encode(resp); err != nil {
		s.log.Printf("failed to encode response: %v", err)
	}
}

// newErrorResponse creates an ErrorResponse from an error.
func newErrorResponse(err error) ErrorResponse {
	status := statusError
	message := err.Error()
	if err == inmem.ErrNoJob {
		status = statusNoJob
		message = "No job found"
	}

	errResp := ErrorResponse{
		Status: status,
		Error:  message,
	}
	return errResp
}

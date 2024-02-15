package queue

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sync"
)

var (
	ErrNoJob = errors.New("no-job")
)

type jobState int

const (
	JobStateUnAssigned jobState = iota // Job not yet assigned to a worker. Default status.
	JobStateAssigned                   // Job assigned to a worker.
	JobStateDeleted                    // Job deleted. Can not be reassigned or retrieved by clients.
	JobStateAborted                    // Job aborted. Once a job is aborted, it can be reassigned to any client. Jobs are automatically aborted when that client disconnects.
)

// job represents a job in a queue.
type job struct {
	ID      uint64          // Unique identifier.
	Pri     int64           // Priority. Higher value has higher priority.
	state   jobState        // Current status of the job.
	Payload json.RawMessage // JSON serialized data associated with job
}

type Store struct {
	qMu    *sync.Mutex      // Protects the queues map.
	queues map[string][]job // Job queues mapped to queue names. In each queue. jobs are sorted in ascending priority order.
	idMu   *sync.Mutex      // Protect ID incrementor.
	curID  uint64           // Next available ID.
}

func NewQueue() *Store {
	return &Store{
		idMu:   &sync.Mutex{},
		qMu:    &sync.Mutex{},
		queues: make(map[string][]job),
		curID:  10000,
	}
}

// nextID returns the next available ID.
func (q *Store) nextID() uint64 {
	q.idMu.Lock()
	q.curID += 1
	q.idMu.Unlock()
	return q.curID
}

// AddJob adds a job to the named queue.
func (q *Store) AddJob(ctx context.Context, queueName string, pri int64, payload json.RawMessage) error {
	id := q.nextID()
	q.qMu.Lock()
	defer q.qMu.Unlock()

	newJob := job{
		ID:      id,
		Pri:     pri,
		Payload: payload,
		state:   JobStateUnAssigned,
	}

	curQ, ok := q.queues[queueName]
	if !ok {
		q.queues[queueName] = []job{newJob}
		return nil
	}

	index := slices.IndexFunc(curQ, func(j job) bool {
		return newJob.Pri >= j.Pri
	})

	// newJob has lowest priority
	if index == -1 {
		curQ = append(curQ, newJob)
	} else {
		curQ = slices.Insert(curQ, index, newJob)
	}

	q.queues[queueName] = curQ
	return nil
}

// NextJob retrieves the highest priority job of all the named queues.
func (q *Store) NextJob(ctx context.Context, queueNames []string) (job, error) {
	highestPriJob := job{Pri: math.MinInt64}
	for _, name := range queueNames {
		j, err := q.nextJob(ctx, name)
		if err != nil {
			return job{}, err
		}
		if j.Pri > highestPriJob.Pri {
			highestPriJob = j
		}
	}

	if highestPriJob.Pri == math.MaxInt64 {
		return job{}, ErrNoJob
	}
	return highestPriJob, nil
}

// nextJob retrieves the highest priority job of the named queue.
func (q *Store) nextJob(ctx context.Context, queueName string) (job, error) {
	q.qMu.Lock()
	defer q.qMu.Unlock()
	curQ, ok := q.queues[queueName]
	if !ok {
		return job{}, ErrNoJob
	}
	if len(curQ) == 0 {
		return job{}, ErrNoJob
	}
	return curQ[0], nil
}

// Package inmem provides an in-memory implementation of the job queues store.
package inmem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Payload json.RawMessage // JSON serialized data associated with job

	state    jobState // Current status of the job.
	workerID *int     // ID of the worker that has been assigned the job, nil if not assigned.
}

type Store struct {
	qMu      *sync.Mutex      // Protects the queues map.
	queues   map[string][]job // Queues of available jobs mapped to queue names. In each queue. jobs are sorted in ascending priority order.
	assigned map[uint64]job   // Jobs assigned to workers. Key is worker ID, value is job.
	deleted  map[uint64]job   // Deleted job. These jobs can not be reassigned. Key is worker ID, value is job.
	idMu     *sync.Mutex      // Protect ID incrementor.
	curID    uint64           // Next available ID.
}

func NewQueue() *Store {
	return &Store{
		idMu:     &sync.Mutex{},
		qMu:      &sync.Mutex{},
		queues:   make(map[string][]job),
		assigned: make(map[uint64]job),
		deleted:  map[uint64]job{},
		curID:    10000,
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
func (q *Store) AddJob(ctx context.Context, userID uint64, queueName string, pri int64, payload json.RawMessage) error {
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
func (s *Store) NextJob(ctx context.Context, userID uint64, queueNames []string) (job, string, error) {
	highestPriJob := job{Pri: math.MinInt64}
	queueName := ""
	for _, name := range queueNames {
		j, err := s.peek(ctx, name)
		if err != nil {
			return job{}, "", err
		}
		if j.Pri > highestPriJob.Pri {
			highestPriJob = j
			queueName = name
		}
	}

	if highestPriJob.Pri == math.MaxInt64 {
		return job{}, "", ErrNoJob
	}

	_, err := s.dequeue(ctx, userID, queueName)
	if err != nil {
		return job{}, "", fmt.Errorf("s.dequeue: %w", err)
	}
	return highestPriJob, queueName, nil
}

// peek retrieves the highest priority job of the named queue. The job is left in the queue.
func (s *Store) peek(ctx context.Context, queueName string) (job, error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()
	curQ, ok := s.queues[queueName]
	if !ok {
		return job{}, ErrNoJob
	}
	if len(curQ) == 0 {
		return job{}, ErrNoJob
	}
	return curQ[0], nil
}

// dequeue remove the job from the queue and adds it to the store's assigned map.
func (s *Store) dequeue(ctx context.Context, userID uint64, queueName string) (job, error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()
	q, ok := s.queues[queueName]
	if !ok {
		return job{}, ErrNoJob
	}

	if len(q) == 0 {
		return job{}, ErrNoJob
	}

	// High pri job is first element
	job := q[0]

	// Move the job to the assigned map
	s.assigned[job.ID] = job
	q = slices.Delete(q, 0, 1)
	s.queues[queueName] = q
	return job, nil
}

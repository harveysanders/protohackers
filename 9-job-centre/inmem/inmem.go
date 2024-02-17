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

// Job represents a Job in a queue.
type Job struct {
	ID        uint64          // Unique identifier.
	Pri       uint64          // A job priority is any non-negative integer. Higher value has higher priority.
	Payload   json.RawMessage // JSON serialized data associated with job
	queueName string          // Name of queue where the job is located.
}

type Store struct {
	qMu      *sync.Mutex      // Protects the queues map.
	queues   map[string][]Job // Queues of available jobs mapped to queue names. In each queue. jobs are sorted in ascending priority order.
	assigned map[uint64]Job   // Jobs assigned to workers. Key is worker ID, value is job.
	deleted  map[uint64]Job   // Deleted job. These jobs can not be reassigned. Key is worker ID, value is job.
	idMu     *sync.Mutex      // Protect ID incrementor.
	curID    uint64           // Next available ID.
}

func NewQueue() *Store {
	return &Store{
		idMu:     &sync.Mutex{},
		qMu:      &sync.Mutex{},
		queues:   make(map[string][]Job),
		assigned: make(map[uint64]Job),
		deleted:  map[uint64]Job{},
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
func (q *Store) AddJob(ctx context.Context, clientID uint64, queueName string, pri uint64, payload json.RawMessage) (Job, error) {
	id := q.nextID()
	q.qMu.Lock()
	defer q.qMu.Unlock()

	newJob := Job{
		ID:        id,
		Pri:       pri,
		Payload:   payload,
		queueName: queueName,
	}

	curQ, ok := q.queues[queueName]
	if !ok {
		q.queues[queueName] = []Job{newJob}
		return newJob, nil
	}

	index := slices.IndexFunc(curQ, func(j Job) bool {
		return newJob.Pri >= j.Pri
	})

	// newJob has lowest priority
	if index == -1 {
		curQ = append(curQ, newJob)
	} else {
		curQ = slices.Insert(curQ, index, newJob)
	}

	q.queues[queueName] = curQ
	return newJob, nil
}

// NextJob retrieves the highest priority job of all the named queues.
func (s *Store) NextJob(ctx context.Context, clientID uint64, queueNames []string) (Job, string, error) {
	highestPriJob := Job{Pri: 0}
	queueName := ""
	for _, name := range queueNames {
		j, err := s.peek(ctx, name)
		if err != nil {
			if err == ErrNoJob {
				continue
			}
			return Job{}, "", err
		}

		if j.Pri > highestPriJob.Pri {
			highestPriJob = j
			queueName = name
		}
	}

	if highestPriJob.Pri == math.MaxInt64 {
		return Job{}, "", ErrNoJob
	}

	_, err := s.dequeue(ctx, clientID, queueName)
	if err != nil {
		return Job{}, "", fmt.Errorf("s.dequeue: %w", err)
	}
	return highestPriJob, queueName, nil
}

// AbortJob aborts a job an assigned job. An abort is only valid from the client that is currently working on that job.
func (s *Store) AbortJob(ctx context.Context, clientID uint64, jobID uint64) error {
	s.qMu.Lock()
	job, ok := s.assigned[clientID]
	if !ok {
		return ErrNoJob
	}
	if job.ID != jobID {
		return ErrNoJob
	}

	s.qMu.Unlock()

	// return the job to the queue
	_, err := s.AddJob(ctx, clientID, job.queueName, job.Pri, job.Payload)
	if err != nil {
		return fmt.Errorf("s.AddJob: %w", err)
	}

	s.qMu.Lock()
	delete(s.assigned, clientID)
	s.qMu.Unlock()
	return nil
}

// peek retrieves the highest priority job of the named queue. The job is left in the queue.
func (s *Store) peek(ctx context.Context, queueName string) (Job, error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()
	curQ, ok := s.queues[queueName]
	if !ok {
		return Job{}, ErrNoJob
	}
	if len(curQ) == 0 {
		return Job{}, ErrNoJob
	}
	return curQ[0], nil
}

// dequeue remove the job from the queue and adds it to the store's assigned map.
func (s *Store) dequeue(ctx context.Context, clientID uint64, queueName string) (Job, error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()
	q, ok := s.queues[queueName]
	if !ok {
		return Job{}, ErrNoJob
	}

	if len(q) == 0 {
		return Job{}, ErrNoJob
	}

	// High pri job is first element
	job := q[0]

	// Move the job to the assigned map
	s.assigned[clientID] = job
	q = slices.Delete(q, 0, 1)
	s.queues[queueName] = q
	return job, nil
}

// DeleteJob deletes a job from the store. Any client can delete a job,
// even if it was worked on by another client.
func (s *Store) DeleteJob(ctx context.Context, clientID uint64, jobID uint64) error {
	_, _, err := s.deleteJobByID(ctx, jobID)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) deleteJobByID(ctx context.Context, jobID uint64) (Job, string,
	error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()

	// TODO: Optimize this first if it becomes an issue.
	for queueName, q := range s.queues {
		idx := slices.IndexFunc(q, func(j Job) bool {
			return j.ID == jobID
		})

		if idx > -1 {
			// Job found
			job := q[idx]
			// Move the job to the deleted map
			s.deleted[job.ID] = job
			// remove from queue
			q = slices.Delete(q, idx, 1)
			s.queues[queueName] = q
			return job, queueName, nil
		}
	}

	// Check if assigned
	for _, j := range s.assigned {
		if j.ID == jobID {
			return j, "assigned", nil
		}
	}
	return Job{}, "", ErrNoJob
}

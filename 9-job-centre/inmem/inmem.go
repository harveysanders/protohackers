// Package inmem provides an in-memory implementation of the job queues store.
package inmem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// waiter is a client waiting for a job on any of the listed queues.
type waiter struct {
	ready      chan string // Channel to notify the client when a job is available.
	queueNames []string    // Names of queues the client is waiting on.
}
type Store struct {
	qMu      *sync.Mutex       // Protects the queues map.
	queues   map[string]queue  // Queues of available jobs mapped to queue names. In each queue. jobs are sorted in ascending priority order.
	assigned map[uint64]Job    // Jobs assigned to workers. Key is worker ID, value is job.
	deleted  map[uint64]Job    // Deleted job. These jobs can not be reassigned. Key is worker ID, value is job.
	idMu     *sync.Mutex       // Protect ID incrementor.
	curID    uint64            // Next available ID.
	waiting  map[uint64]waiter // Clients waiting for a job on a queue.
}

type queue struct {
	// jobs are sorted in ascending priority order.
	jobs []Job
}

func NewQueue() *queue {
	return &queue{
		jobs: make([]Job, 0, 100),
	}
}

func NewStore() *Store {
	return &Store{
		idMu:     &sync.Mutex{},
		qMu:      &sync.Mutex{},
		queues:   make(map[string]queue),
		assigned: make(map[uint64]Job),
		deleted:  map[uint64]Job{},
		curID:    10000,
		waiting:  make(map[uint64]waiter),
	}
}

// nextID returns the next available ID.
func (s *Store) nextID() uint64 {
	s.idMu.Lock()
	s.curID += 1
	s.idMu.Unlock()
	return s.curID
}

type AddJobParams struct {
	ID        *uint64
	QueueName string
	Priority  uint64
	Payload   json.RawMessage
}

// AddJob adds a job to the named queue.
func (s *Store) AddJob(ctx context.Context, clientID uint64, args AddJobParams) (Job, error) {
	id := args.ID
	if id == nil {
		nextID := s.nextID()
		id = &nextID
	}
	s.qMu.Lock()
	defer s.qMu.Unlock()

	newJob := Job{
		ID:        *id,
		Pri:       args.Priority,
		Payload:   args.Payload,
		queueName: args.QueueName,
	}

	curQ, ok := s.queues[newJob.queueName]
	if !ok {
		s.queues[newJob.queueName] = queue{
			jobs: []Job{newJob},
		}
		s.notify(newJob.queueName)
		return newJob, nil
	}

	index := slices.IndexFunc(curQ.jobs, func(j Job) bool {
		return newJob.Pri >= j.Pri
	})

	// newJob has lowest priority
	if index == -1 {
		curQ.jobs = append(curQ.jobs, newJob)
	} else {
		curQ.jobs = slices.Insert(curQ.jobs, index, newJob)
	}

	s.queues[newJob.queueName] = curQ

	// Notify any waiting clients
	s.notify(newJob.queueName)
	return newJob, nil
}

func (s *Store) notify(queueName string) {
	for _, v := range s.waiting {
		if slices.Contains(v.queueNames, queueName) {
			v.ready <- queueName
			log.Printf("notified: %s", queueName)
			return
		}
	}
	log.Println("no one listening")
}

// NextJob retrieves the highest priority job of all the named queues.
func (s *Store) NextJob(ctx context.Context, clientID uint64, queueNames []string, wait bool) (Job, string, error) {
	highestPriJob := Job{Pri: 0}
	queueName := ""
	// Need to hold the lock until the job is dequeued,
	// or the next client could dequeue the same job between the peek and dequeue.
	s.qMu.Lock()
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
		s.qMu.Unlock()
		return Job{}, "", ErrNoJob
	}

	_, err := s.dequeue(ctx, clientID, queueName)
	s.qMu.Unlock()
	if err != nil && err != ErrNoJob {
		return Job{}, "", fmt.Errorf("s.dequeue: %w", err)
	}
	if err == ErrNoJob {
		if !wait {
			return Job{}, "", fmt.Errorf("s.dequeue: %w", err)
		}

		// Wait for a job to be added to the queue
		ready := make(chan string)

		s.qMu.Lock()
		s.waiting[clientID] = waiter{queueNames: queueNames, ready: ready}
		s.qMu.Unlock()

		log.Printf("[%d] waiting for next job...\n", clientID)
		queueName = <-ready
		log.Printf("[%d] received job on queue %q\n", clientID, queueName)

		s.qMu.Lock()
		delete(s.waiting, clientID)

		nextJob, err := s.dequeue(ctx, clientID, queueName)
		s.qMu.Unlock()
		if err != nil {
			return Job{}, "", fmt.Errorf("s.dequeue: %w", err)
		}
		return nextJob, queueName, nil
	}
	return highestPriJob, queueName, nil
}

// AbortJob aborts a job an assigned job. An abort is only valid from the client that is currently working on that job.
func (s *Store) AbortJob(ctx context.Context, clientID uint64, jobID uint64) error {
	s.qMu.Lock()
	job, ok := s.assigned[clientID]
	if !ok {
		s.qMu.Unlock()
		return ErrNoJob
	}
	if job.ID != jobID {
		s.qMu.Unlock()
		return ErrNoJob
	}

	s.qMu.Unlock()

	// return the job to the queue
	_, err := s.AddJob(ctx, clientID, AddJobParams{
		ID:        &jobID,
		QueueName: job.queueName,
		Priority:  job.Pri,
		Payload:   job.Payload,
	})
	if err != nil {
		return fmt.Errorf("s.AddJob: %w", err)
	}

	s.qMu.Lock()
	delete(s.assigned, clientID)
	s.qMu.Unlock()
	return nil
}

func (s *Store) GetAssignedJob(ctx context.Context, clientID uint64) (Job, error) {
	s.qMu.Lock()
	defer s.qMu.Unlock()
	job, ok := s.assigned[clientID]
	if !ok {
		return Job{}, ErrNoJob
	}
	return job, nil
}

// peek retrieves the highest priority job of the named queue. The job is left in the queue.
func (s *Store) peek(ctx context.Context, queueName string) (Job, error) {
	curQ, ok := s.queues[queueName]
	if !ok {
		return Job{}, ErrNoJob
	}
	if len(curQ.jobs) == 0 {
		return Job{}, ErrNoJob
	}
	return curQ.jobs[0], nil
}

// dequeue remove the job from the queue and adds it to the store's assigned map.
func (s *Store) dequeue(ctx context.Context, clientID uint64, queueName string) (Job, error) {
	q, ok := s.queues[queueName]
	if !ok {
		return Job{}, ErrNoJob
	}

	if len(q.jobs) == 0 {
		return Job{}, ErrNoJob
	}

	// High pri job is first element
	job := q.jobs[0]

	// Move the job to the assigned map
	s.assigned[clientID] = job
	q.jobs = slices.Delete(q.jobs, 0, 1)
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
		idx := slices.IndexFunc(q.jobs, func(j Job) bool {
			return j.ID == jobID
		})

		if idx > -1 {
			// Job found
			job := q.jobs[idx]
			// Move the job to the deleted map
			s.deleted[job.ID] = job
			// remove from queue
			q.jobs = slices.Delete(q.jobs, idx, idx+1)
			s.queues[queueName] = q
			return job, queueName, nil
		}
	}

	// Check if assigned
	for clientID, j := range s.assigned {
		if j.ID == jobID {
			delete(s.assigned, clientID)
			return j, "assigned", nil
		}
	}
	return Job{}, "", ErrNoJob
}

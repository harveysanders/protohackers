package queue

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
)

type jobState int

const (
	JobStateUnAssigned jobState = iota // Job not yet assigned to a worker. Default status.
	JobStateAssigned                   // Job assigned to a worker.
	JobStateDeleted                    // Job deleted. Can not be reassigned or retrieved by clients.
	JobStateAborted                    // Job aborted. Once a job is aborted, it can be reassigned to any client. Jobs are automatically aborted when that client disconnects.
)

type job struct {
	ID      uint64          // Unique identifier.
	Pri     int64           // Priority. Higher value has higher priority.
	state   jobState        // Current status of the job.
	Payload json.RawMessage // JSON serialized data associated with job
}

type Queue struct {
	jobsMu *sync.Mutex      // Protects the jobs map.
	jobs   map[string][]job // Internal jobs store. Jobs are sorted in descending priority order.
	idMu   *sync.Mutex      // Protect ID incrementor
	curID  uint64           // Next available ID.
}

func NewQueue() *Queue {
	return &Queue{
		idMu:   &sync.Mutex{},
		jobsMu: &sync.Mutex{},
		jobs:   make(map[string][]job),
		curID:  10000,
	}
}

func (q *Queue) nextID() uint64 {
	q.idMu.Lock()
	q.curID += 1
	q.idMu.Unlock()
	return q.curID
}

func (q *Queue) Insert(ctx context.Context, queueName string, pri int64, payload json.RawMessage) error {
	id := q.nextID()
	q.jobsMu.Lock()
	defer q.jobsMu.Unlock()

	newJob := job{
		ID:      id,
		Pri:     pri,
		Payload: payload,
		state:   JobStateUnAssigned,
	}

	curQ, ok := q.jobs[queueName]
	if !ok {
		q.jobs[queueName] = []job{newJob}
		return nil
	}

	index := slices.IndexFunc(curQ, func(j job) bool {
		return newJob.Pri >= j.Pri
	})
	// Job has lowest priority
	if index == -1 {
		curQ = append(curQ, newJob)
	} else {
		slices.Insert(curQ, index, newJob)
	}

	q.jobs[queueName] = curQ
	return nil
}

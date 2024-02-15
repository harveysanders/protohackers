package inmem_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/stretchr/testify/require"
)

type testJob struct {
	queueName string
	pri       int64
	payload   json.RawMessage
}

func TestInsert(t *testing.T) {
	toInsert := []testJob{
		{
			queueName: "test",
			pri:       3,
			payload:   json.RawMessage(`{"test": "test"}`),
		},
		{
			queueName: "test",
			pri:       1,
			payload:   json.RawMessage(`{"test": "test"}`),
		},
		{
			queueName: "test",
			pri:       2,
			payload:   json.RawMessage(`{"test": "test"}`),
		},
	}

	ctx := context.Background()
	q := inmem.NewQueue()
	clientID := uint64(1)

	for _, ti := range toInsert {
		err := q.AddJob(ctx, clientID, ti.queueName, ti.pri, ti.payload)
		require.NoError(t, err)
	}
}

func TestGet(t *testing.T) {
	t.Run("one job", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		q := inmem.NewQueue()

		err := q.AddJob(ctx, clientID, "q1", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		j, queueName, err := q.NextJob(ctx, clientID, []string{"q1"})
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, int64(1), j.Pri)
	})

	t.Run("multiple jobs", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		q := inmem.NewQueue()

		err := q.AddJob(ctx, clientID, "queue1", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		err = q.AddJob(ctx, clientID, "queue1", 2, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		j, queueName, err := q.NextJob(ctx, clientID, []string{"queue1"})
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, int64(2), j.Pri)
	})

	t.Run("retrieve highest priority from all queues", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		ctx = context.WithValue(ctx, "clientID", 1)
		q := inmem.NewQueue()

		jobs := []testJob{
			{
				queueName: "queue1",
				pri:       1,
				payload:   json.RawMessage(`{"test": 1}`),
			},
			{
				queueName: "queue2",
				pri:       2,
				payload:   json.RawMessage(`{"test": 2}`),
			},
		}

		for _, job := range jobs {
			err := q.AddJob(ctx, clientID, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID, []string{"queue1", "queue2"})
		require.NoError(t, err)
		require.Equal(t, "queue2", queueName)
		require.Equal(t, int64(2), j.Pri)

		j, queueName, err = q.NextJob(ctx, clientID, []string{"queue2", "queue1"})
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, int64(2), j.Pri)
	})

	t.Run("job unavailable after assigned", func(t *testing.T) {
		jobs := []testJob{
			{
				queueName: "test",
				pri:       1,
			},
		}

		clientID1 := uint64(1)
		clientID2 := uint64(2)
		ctx := context.Background()
		q := inmem.NewQueue()

		for _, job := range jobs {
			err := q.AddJob(ctx, clientID1, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID1, []string{"test"})
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, int64(1), j.Pri)

		j, queueName, err = q.NextJob(ctx, clientID2, []string{"test"})
		require.Equal(t, inmem.ErrNoJob, err)
		require.Equal(t, "", queueName)
		require.Empty(t, j)
	})
}

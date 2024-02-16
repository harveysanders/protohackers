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

func TestAddJob(t *testing.T) {
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
		job, err := q.AddJob(ctx, clientID, ti.queueName, ti.pri, ti.payload)
		require.NoError(t, err)

		require.NotEmpty(t, job.ID)
		require.Equal(t, ti.payload, job.Payload)
		require.Equal(t, ti.pri, job.Pri)
	}
}

func TestNextJob(t *testing.T) {
	t.Run("one job", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		q := inmem.NewQueue()

		_, err := q.AddJob(ctx, clientID, "q1", 1, json.RawMessage(`{"test": "test"}`))
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

		job, err := q.AddJob(ctx, clientID, "queue1", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)

		job, err = q.AddJob(ctx, clientID, "queue1", 2, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)

		j, queueName, err := q.NextJob(ctx, clientID, []string{"queue1"})
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, int64(2), j.Pri)
	})

	t.Run("retrieve highest priority from all queues", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
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
			_, err := q.AddJob(ctx, clientID, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID, []string{"queue1", "queue2"})
		require.NoError(t, err)
		require.Equal(t, "queue2", queueName)
		require.Equal(t, int64(2), j.Pri)

		j, queueName, err = q.NextJob(ctx, clientID, []string{"queue2", "queue1"})
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, int64(1), j.Pri)
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
			_, err := q.AddJob(ctx, clientID1, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID1, []string{"test"})
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, int64(1), j.Pri)

		j, queueName, err = q.NextJob(ctx, clientID2, []string{"test"})
		require.ErrorIs(t, err, inmem.ErrNoJob)
		require.Equal(t, "", queueName)
		require.Empty(t, j)
	})
}

func TestDelete(t *testing.T) {
	t.Run("Delete assigned job", func(t *testing.T) {
		jobs := []testJob{
			{
				queueName: "test",
				pri:       40,
			},

			{
				queueName: "test",
				pri:       300,
			},
		}

		clientID := uint64(1)
		ctx := context.Background()
		q := inmem.NewQueue()

		for _, job := range jobs {
			_, err := q.AddJob(ctx, clientID, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID, []string{"test"})
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, int64(300), j.Pri)
		require.NotEmpty(t, j.ID)

		err = q.DeleteJob(ctx, clientID, j.ID)
		require.NoError(t, err)

		j, queueName, err = q.NextJob(ctx, clientID, []string{"test"})
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, int64(40), j.Pri)

		j, queueName, err = q.NextJob(ctx, clientID, []string{"test"})
		require.ErrorIs(t, err, inmem.ErrNoJob)
		require.Empty(t, j)
		require.Empty(t, queueName)
	})

	t.Run("delete available job", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		job := testJob{
			queueName: "test",
			pri:       300,
		}
		q := inmem.NewQueue()
		queued, err := q.AddJob(ctx, clientID, job.queueName, job.pri, job.payload)
		require.NoError(t, err)
		require.NotEmpty(t, queued.ID)

		err = q.DeleteJob(ctx, clientID, queued.ID)
		require.NoError(t, err)

		_, _, err = q.NextJob(ctx, clientID, []string{"test"})
		require.ErrorIs(t, err, inmem.ErrNoJob)

		err = q.DeleteJob(ctx, clientID, queued.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job should already be deleted")
	})

}

func TestAbortJob(t *testing.T) {
	t.Run("Aborted jobs should be returned to the queue", func(t *testing.T) {
		jobs := []testJob{
			{
				queueName: "q1",
				pri:       256,
			},
			{
				queueName: "q1",
				pri:       512,
			},
		}

		ctx := context.Background()
		clientID := uint64(123)
		q := inmem.NewQueue()

		for _, job := range jobs {
			_, err := q.AddJob(ctx, clientID, job.queueName, job.pri, job.payload)
			require.NoError(t, err)
		}

		j, queueName, err := q.NextJob(ctx, clientID, []string{"q1"})
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, int64(512), j.Pri)

		err = q.AbortJob(ctx, clientID, j.ID)
		require.NoError(t, err)

		j, queueName, err = q.NextJob(ctx, clientID, []string{"q1"})
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, int64(512), j.Pri)
	})

	t.Run("Abort assigned job", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		q := inmem.NewQueue()
		_, err := q.AddJob(ctx, clientID, "test", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		job, _, err := q.NextJob(ctx, clientID, []string{"test"})
		require.NoError(t, err)

		err = q.AbortJob(ctx, clientID, job.ID)
		require.NoError(t, err)

		err = q.AbortJob(ctx, clientID, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job should already be aborted")
	})

	t.Run("cannot abort job that is unassigned", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		q := inmem.NewQueue()
		job, err := q.AddJob(ctx, clientID, "test", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		err = q.AbortJob(ctx, clientID, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job is not be assigned yet")
	})

	t.Run("cannot abort another client's job", func(t *testing.T) {
		ctx := context.Background()
		clientID1 := uint64(123)
		clientID2 := uint64(456)
		q := inmem.NewQueue()
		_, err := q.AddJob(ctx, clientID1, "test", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		job, _, err := q.NextJob(ctx, clientID1, []string{"test"})
		require.NoError(t, err)

		err = q.AbortJob(ctx, clientID2, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job is assigned to another client")
	})
}

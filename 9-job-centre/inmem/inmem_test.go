package inmem_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/stretchr/testify/require"
)

func TestAddJob(t *testing.T) {
	toInsert := []inmem.AddJobParams{
		{
			QueueName: "test",
			Priority:  3,
			Payload:   json.RawMessage(`{"test": "test"}`),
		},
		{
			QueueName: "test",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		},
		{
			QueueName: "test",
			Priority:  2,
			Payload:   json.RawMessage(`{"test": "test"}`),
		},
	}

	ctx := context.Background()
	s := inmem.NewStore()
	clientID := uint64(1)

	for _, args := range toInsert {
		job, err := s.AddJob(ctx, clientID, args)
		require.NoError(t, err)

		require.NotEmpty(t, job.ID)
		require.Equal(t, args.Payload, job.Payload)
		require.Equal(t, args.Priority, job.Pri)
	}
}

func TestNextJob(t *testing.T) {
	t.Run("one job", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		s := inmem.NewStore()
		args := inmem.AddJobParams{
			QueueName: "q1",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		}
		_, err := s.AddJob(ctx, clientID, args)
		require.NoError(t, err)

		j, queueName, err := s.NextJob(ctx, clientID, []string{"q1"}, false)
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, uint64(1), j.Pri)
	})

	t.Run("multiple jobs", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		s := inmem.NewStore()

		job, err := s.AddJob(ctx, clientID, inmem.AddJobParams{
			QueueName: "queue1",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		})
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)

		job, err = s.AddJob(ctx, clientID, inmem.AddJobParams{
			QueueName: "queue1",
			Priority:  2,
			Payload:   json.RawMessage(`{"test": "test"}`),
		})
		require.NoError(t, err)
		require.NotEmpty(t, job.ID)

		j, queueName, err := s.NextJob(ctx, clientID, []string{"queue1"}, false)
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, uint64(2), j.Pri)
	})

	t.Run("retrieve highest priority from all queues", func(t *testing.T) {
		clientID := uint64(1)
		ctx := context.Background()
		s := inmem.NewStore()

		jobs := []inmem.AddJobParams{
			{
				QueueName: "queue1",
				Priority:  1,
				Payload:   json.RawMessage(`{"test": 1}`),
			},
			{
				QueueName: "queue2",
				Priority:  2,
				Payload:   json.RawMessage(`{"test": 2}`),
			},
		}

		for _, job := range jobs {
			_, err := s.AddJob(ctx, clientID, job)
			require.NoError(t, err)
		}

		j, queueName, err := s.NextJob(ctx, clientID, []string{"queue1", "queue2"}, false)
		require.NoError(t, err)
		require.Equal(t, "queue2", queueName)
		require.Equal(t, uint64(2), j.Pri)

		j, queueName, err = s.NextJob(ctx, clientID, []string{"queue2", "queue1"}, false)
		require.NoError(t, err)
		require.Equal(t, "queue1", queueName)
		require.Equal(t, uint64(1), j.Pri)
	})

	t.Run("job unavailable after assigned", func(t *testing.T) {
		jobs := []inmem.AddJobParams{
			{
				QueueName: "test",
				Priority:  1,
			},
		}

		clientID1 := uint64(1)
		clientID2 := uint64(2)
		ctx := context.Background()
		s := inmem.NewStore()

		for _, job := range jobs {
			_, err := s.AddJob(ctx, clientID1, job)
			require.NoError(t, err)
		}

		j, queueName, err := s.NextJob(ctx, clientID1, []string{"test"}, false)
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, uint64(1), j.Pri)

		j, queueName, err = s.NextJob(ctx, clientID2, []string{"test"}, false)
		require.ErrorIs(t, err, inmem.ErrNoJob)
		require.Equal(t, "", queueName)
		require.Empty(t, j)
	})
}

func TestDelete(t *testing.T) {
	t.Run("Delete assigned job", func(t *testing.T) {
		jobs := []inmem.AddJobParams{
			{
				QueueName: "test",
				Priority:  40,
			},

			{
				QueueName: "test",
				Priority:  300,
			},
		}

		clientID := uint64(1)
		ctx := context.Background()
		s := inmem.NewStore()

		for _, job := range jobs {
			_, err := s.AddJob(ctx, clientID, job)
			require.NoError(t, err)
		}

		j, queueName, err := s.NextJob(ctx, clientID, []string{"test"}, false)
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, uint64(300), j.Pri)
		require.NotEmpty(t, j.ID)

		err = s.DeleteJob(ctx, clientID, j.ID)
		require.NoError(t, err)

		j, queueName, err = s.NextJob(ctx, clientID, []string{"test"}, false)
		require.NoError(t, err)
		require.Equal(t, "test", queueName)
		require.Equal(t, uint64(40), j.Pri)

		j, queueName, err = s.NextJob(ctx, clientID, []string{"test"}, false)
		require.ErrorIs(t, err, inmem.ErrNoJob)
		require.Empty(t, j)
		require.Empty(t, queueName)
	})

	t.Run("delete available job", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		job := inmem.AddJobParams{
			QueueName: "test",
			Priority:  300,
		}
		s := inmem.NewStore()
		queued, err := s.AddJob(ctx, clientID, job)
		require.NoError(t, err)
		require.NotEmpty(t, queued.ID)

		err = s.DeleteJob(ctx, clientID, queued.ID)
		require.NoError(t, err)

		_, _, err = s.NextJob(ctx, clientID, []string{"test"}, false)
		require.ErrorIs(t, err, inmem.ErrNoJob)

		err = s.DeleteJob(ctx, clientID, queued.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job should already be deleted")
	})

}

func TestAbortJob(t *testing.T) {
	t.Run("Aborted jobs should be returned to the queue", func(t *testing.T) {
		jobs := []inmem.AddJobParams{
			{
				QueueName: "q1",
				Priority:  256,
			},
			{
				QueueName: "q1",
				Priority:  512,
			},
		}

		ctx := context.Background()
		clientID := uint64(123)
		s := inmem.NewStore()

		for _, job := range jobs {
			_, err := s.AddJob(ctx, clientID, job)
			require.NoError(t, err)
		}

		j, queueName, err := s.NextJob(ctx, clientID, []string{"q1"}, false)
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, uint64(512), j.Pri)

		err = s.AbortJob(ctx, clientID, j.ID)
		require.NoError(t, err)

		j, queueName, err = s.NextJob(ctx, clientID, []string{"q1"}, false)
		require.NoError(t, err)
		require.Equal(t, "q1", queueName)
		require.Equal(t, uint64(512), j.Pri)
	})

	t.Run("Abort assigned job", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		s := inmem.NewStore()
		args := inmem.AddJobParams{
			QueueName: "test",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		}
		_, err := s.AddJob(ctx, clientID, args)
		require.NoError(t, err)

		job, _, err := s.NextJob(ctx, clientID, []string{"test"}, false)
		require.NoError(t, err)

		err = s.AbortJob(ctx, clientID, job.ID)
		require.NoError(t, err)

		err = s.AbortJob(ctx, clientID, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job should already be aborted")
	})

	t.Run("cannot abort job that is unassigned", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		s := inmem.NewStore()
		args := inmem.AddJobParams{
			QueueName: "test",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		}
		job, err := s.AddJob(ctx, clientID, args)
		require.NoError(t, err)

		err = s.AbortJob(ctx, clientID, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job is not be assigned yet")
	})

	t.Run("cannot abort another client's job", func(t *testing.T) {
		ctx := context.Background()
		clientID1 := uint64(123)
		clientID2 := uint64(456)
		s := inmem.NewStore()
		_, err := s.AddJob(ctx, clientID1, inmem.AddJobParams{
			QueueName: "test",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`)})
		require.NoError(t, err)

		job, _, err := s.NextJob(ctx, clientID1, []string{"test"}, false)
		require.NoError(t, err)

		err = s.AbortJob(ctx, clientID2, job.ID)
		require.ErrorIs(t, err, inmem.ErrNoJob, "job is assigned to another client")
	})
}

func TestWaitForNextJob(t *testing.T) {
	t.Run("wait for job", func(t *testing.T) {
		ctx := context.Background()
		clientID := uint64(123)
		s := inmem.NewStore()
		job := inmem.AddJobParams{
			QueueName: "qwerty",
			Priority:  1,
			Payload:   json.RawMessage(`{"test": "test"}`),
		}

		go func(j inmem.AddJobParams) {
			// SImulate a delay in adding the job
			time.Sleep(100 * time.Millisecond)
			_, err := s.AddJob(ctx, clientID, j)
			require.NoError(t, err)
		}(job)

		// Not waiting, so should return ErrNoJob
		_, _, err := s.NextJob(ctx, clientID, []string{"test"}, false)
		require.ErrorIs(t, err, inmem.ErrNoJob)

		// Should wait indefinitely for a job
		j, queueName, err := s.NextJob(ctx, clientID, []string{"test", "qwerty", "abc"}, true)
		require.NoError(t, err)
		require.Equal(t, "qwerty", queueName)
		require.Equal(t, uint64(1), j.Pri)
	})
}

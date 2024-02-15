package queue_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/harveysanders/protohackers/9-job-centre/queue"
	"github.com/stretchr/testify/require"
)

func TestInsert(t *testing.T) {
	toInsert := []struct {
		queueName string
		pri       int64
		payload   json.RawMessage
	}{
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
	q := queue.NewQueue()
	for _, ti := range toInsert {
		err := q.Insert(ctx, ti.queueName, ti.pri, ti.payload)
		require.NoError(t, err)
	}
}

func TestGet(t *testing.T) {
	t.Run("one job", func(t *testing.T) {
		ctx := context.Background()
		q := queue.NewQueue()
		err := q.Insert(ctx, "test", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		j, err := q.Get(ctx, []string{"test"})
		require.NoError(t, err)
		require.Equal(t, int64(1), j.Pri)
	})

	t.Run("multiple jobs", func(t *testing.T) {
		ctx := context.Background()
		q := queue.NewQueue()

		err := q.Insert(ctx, "queue1", 1, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		err = q.Insert(ctx, "queue1", 2, json.RawMessage(`{"test": "test"}`))
		require.NoError(t, err)

		j, err := q.Get(ctx, []string{"queue1"})
		require.NoError(t, err)
		require.Equal(t, int64(2), j.Pri)
	})
}

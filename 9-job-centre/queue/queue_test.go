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

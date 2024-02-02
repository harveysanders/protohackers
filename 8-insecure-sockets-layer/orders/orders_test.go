package orders_test

import (
	"testing"

	"github.com/harveysanders/protohackers/8-insecure-sockets-layer/orders"
	"github.com/stretchr/testify/require"
)

func TestMustCopies(t *testing.T) {
	t.Run("returns the toy with the most requested copies", func(t *testing.T) {
		input := "10x toy car,15x dog on a string,4x inflatable motorcycle"

		want := "15x dog on a string"

		got, err := orders.MostCopies(input)
		require.NoError(t, err)
		require.Equal(t, want, got)

	})
}

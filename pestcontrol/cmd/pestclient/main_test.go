package main

import (
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	client := NewClient("pestcontrol.protohackers.com:20547")
	err := client.Connect()
	require.NoError(t, err)

	defer func() {
		err := client.Close()
		require.NoError(t, err, "error closing connection")
	}()

	err = client.SendHello()
	require.NoError(t, err)

	resp := make([]byte, 2048)
	nRead, err := client.bufR.Read(resp)
	require.NoError(t, err)

	rawMsg := resp[:nRead]
	msg := proto.Message{}
	err = msg.UnmarshalBinary(rawMsg)
	require.NoError(t, err)

	gotHello, err := msg.ToMsgHello()
	require.NoError(t, err)
	require.Equal(t, "pestcontrol", gotHello.Protocol)
}

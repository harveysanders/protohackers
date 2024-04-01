package pestcontrol

import (
	"bytes"
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	client := Client{}
	err := client.Connect("pestcontrol.protohackers.com:20547")
	require.NoError(t, err)

	defer func() {
		err := client.Close()
		require.NoError(t, err, "error closing connection")
	}()

	err = client.sendHello()
	require.NoError(t, err)

	resp := make([]byte, 2048)
	nRead, err := client.conn.Read(resp)
	require.NoError(t, err)

	msg := proto.Message{}
	_, err = msg.ReadFrom(bytes.NewReader(resp[:nRead]))
	require.NoError(t, err)

	gotHello, err := msg.ToMsgHello()
	require.NoError(t, err)
	require.Equal(t, "pestcontrol", gotHello.Protocol)
}

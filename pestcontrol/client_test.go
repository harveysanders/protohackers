package pestcontrol

import (
	"bytes"
	"encoding"
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

type msgDirection int

const (
	msgDirectionInbound  msgDirection = 1
	msgDirectionOutbound msgDirection = 2
)

func TestClient_AuthServerSession(t *testing.T) {
	reqResp := []struct {
		dir msgDirection
		msg encoding.BinaryMarshaler
	}{
		{dir: msgDirectionOutbound, msg: proto.MsgHello{}},
		{dir: msgDirectionInbound, msg: proto.MsgHello{}},
	}

	config := ServerConfig{
		// TODO: Mock the Authority server
		AuthServerAddr: "pestcontrol.protohackers.com:20547",
	}
	client := Client{}
	client.Connect(config.AuthServerAddr)

	for _, rr := range reqResp {
		switch rr.dir {
		case msgDirectionOutbound:
			data, err := rr.msg.MarshalBinary()
			require.NoError(t, err)
			_, err = client.conn.Write(data)
			require.NoError(t, err)

		case msgDirectionInbound:
			resp := make([]byte, 2048)
			nRead, err := client.conn.Read(resp)
			require.NoError(t, err)
			data, err := rr.msg.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, data, resp[:nRead], "unexpected response")
		}
	}

}

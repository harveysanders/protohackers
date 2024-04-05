package pestcontrol_test

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/inmem"
	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.SkipNow()
	testStore := inmem.NewStore()
	srv := pestcontrol.NewServer(
		nil,
		pestcontrol.ServerConfig{AuthServerAddr: "pestcontrol.protohackers.com:20547"},
		testStore,
	)

	go func() {
		err := srv.ListenAndServe(":12345")
		if err != nil {
			t.Log(err)
			return
		}
	}()

	time.Sleep(500 * time.Millisecond)

	client, err := net.Dial("tcp", "localhost:12345")
	require.NoError(t, err)

	defer func() {
		_ = client.Close()
		_ = srv.Close()
	}()

	helloMsg := proto.MsgHello{}
	msg, err := helloMsg.MarshalBinary()
	require.NoError(t, err)

	_, err = client.Write(msg)
	require.NoError(t, err)

	respData := make([]byte, 2048)
	nRead, err := client.Read(respData)
	require.NoError(t, err)

	var resp proto.Message
	_, err = resp.ReadFrom(bytes.NewReader(respData[:nRead]))
	require.NoError(t, err)
	_, err = resp.ToMsgHello()
	require.NoError(t, err)

	observation := proto.MsgSiteVisit{Site: 900085189, Populations: []proto.PopulationCount{{Species: "Aedes aegypti", Count: 10}, {Species: "Anopheles gambiae", Count: 5}}}

	msg, err = observation.MarshalBinary()
	require.NoError(t, err)

	_, err = client.Write(msg)
	require.NoError(t, err)

	time.Sleep(740 * time.Millisecond)

	policy, err := testStore.GetPolicy(900085189, "Aedes aegypti")
	require.NoError(t, err)
	require.Equal(t, pestcontrol.Cull, policy.Action)
}

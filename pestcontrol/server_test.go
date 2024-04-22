package pestcontrol_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/pestcontrol"
	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/harveysanders/protohackers/pestcontrol/sqlite"
	"github.com/stretchr/testify/require"
)

var dsn = os.Getenv("PESTCONTROL_TEST_DSN")

func TestServer(t *testing.T) {

	t.Run("sets the correct policy", func(t *testing.T) {
		db := sqlite.NewDB(dsn)
		err := db.Open(true)
		require.NoError(t, err)
		defer func() {
			_ = db.Close()
		}()
		require.NoError(t, err)

		pestcontrolAddr := "localhost:12345"
		authorityAddr := "localhost:20547"
		siteID := uint32(900085189)
		species := "Ethiopian Buna Wusha"
		targetPop := map[string]pestcontrol.TargetPopulation{}
		targetPop[species] = pestcontrol.TargetPopulation{
			Species: species,
			Min:     10,
			Max:     20,
		}

		// Set up mock Authority server
		authServer := NewMockAuthorityServer()
		authServer.sites[siteID] = pestcontrol.Site{
			ID:                siteID,
			TargetPopulations: targetPop,
			Policies:          map[string]pestcontrol.Policy{},
		}

		go func() {
			err := authServer.ListenAndServe(authorityAddr)
			if err != nil {
				t.Log(err)
				return
			}
		}()

		siteService := sqlite.NewSiteService(db.DB)
		srv := pestcontrol.NewServer(
			nil,
			pestcontrol.ServerConfig{AuthServerAddr: authorityAddr},
			siteService,
		)

		go func() {
			err := srv.ListenAndServe(pestcontrolAddr)
			if err != nil {
				t.Log(err)
				return
			}
		}()

		// wait for server to start
		time.Sleep(500 * time.Millisecond)

		fieldClient, err := net.Dial("tcp", pestcontrolAddr)
		require.NoError(t, err)

		defer func() {
			_ = fieldClient.Close()
			_ = srv.Close()
			_ = authServer.Close()
		}()

		// Clients sends initial "Hello" message
		helloMsg := proto.MsgHello{}
		msg, err := helloMsg.MarshalBinary()
		require.NoError(t, err)

		_, err = fieldClient.Write(msg)
		require.NoError(t, err)

		respData := make([]byte, 2048)
		nRead, err := fieldClient.Read(respData)
		require.NoError(t, err)

		var resp proto.Message
		_, err = resp.ReadFrom(bytes.NewReader(respData[:nRead]))
		require.NoError(t, err)
		_, err = resp.ToMsgHello()
		require.NoError(t, err)

		// After "Hello" response,
		// Send the Site Visit observation (species counts)
		observation := proto.MsgSiteVisit{
			Site:        siteID,
			Populations: []proto.PopulationCount{{Species: species, Count: 9}},
		}

		msg, err = observation.MarshalBinary()
		require.NoError(t, err)

		_, err = fieldClient.Write(msg)
		require.NoError(t, err)

		<-authServer.policyChange
		// The Pestcontrol service should have
		// - created a new "Conserve" policy on the Authority server for the specified site
		authSrvPolicy, ok := authServer.sites[siteID].Policies[species]
		require.True(t, ok, "Authority server should have a policy for the site")
		require.Equal(t, pestcontrol.Conserve, authSrvPolicy.Action)

		// - and eventually recorded the policy in its own records
		retryTick := time.NewTicker(100 * time.Millisecond)
		ctx, cancel := context.WithDeadline(
			context.Background(),
			time.Now().Add(3*time.Second),
		)

		defer retryTick.Stop()
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				t.Fatal("timed out waiting for policy to be recorded")
			case <-retryTick.C:
				policy, err := siteService.GetPolicy(ctx, siteID, species)
				if err != nil {
					// Retry if policy not yet recorded in the Pestcontrol DB
					require.ErrorIs(t, err, pestcontrol.ErrPolicyNotFound)
					continue
				}
				require.Equal(t, pestcontrol.Conserve, policy.Action)
				return
			}
		}
	})
}

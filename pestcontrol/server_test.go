package pestcontrol_test

import (
	"bytes"
	"context"
	"log"
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
		authServer.sites[siteID] = site{
			id:                siteID,
			targetPopulations: targetPop,
			policies:          map[string][]pestcontrol.Policy{},
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
			SiteID:      siteID,
			Populations: []proto.PopulationCount{{Species: species, Count: 9}},
		}

		msg, err = observation.MarshalBinary()
		require.NoError(t, err)

		_, err = fieldClient.Write(msg)
		require.NoError(t, err)

		<-authServer.policyChange
		// The Pestcontrol service should have
		// - created a new "Conserve" policy on the Authority server for the specified site
		authSrvPolicies, ok := authServer.sites[siteID].policies[species]
		require.True(t, ok, "Authority server should have a policy for the site")
		require.Len(t, authSrvPolicies, 1)
		require.Equal(t, pestcontrol.Conserve, authSrvPolicies[0].Action)

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

	t.Run("ultimately settles on one correct policy with multiple observations", func(t *testing.T) {
		db := sqlite.NewDB(dsn)
		err := db.Open(true)
		require.NoError(t, err)
		defer func() {
			_ = db.Close()
		}()
		require.NoError(t, err)

		pestcontrolAddr := "localhost:12345"
		authorityAddr := "localhost:20547"
		siteID := uint32(3813688379)
		species := "three-legged swallow"
		targetPop := map[string]pestcontrol.TargetPopulation{}
		targetPop[species] = pestcontrol.TargetPopulation{
			Species: species,
			Min:     27,
			Max:     58,
		}

		// Set up mock Authority server
		authServer := NewMockAuthorityServer()
		authServer.sites[siteID] = site{
			id:                siteID,
			targetPopulations: targetPop,
			policies:          map[string][]pestcontrol.Policy{},
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
		observations := []struct {
			sendAfter *time.Timer
			msg       proto.MsgSiteVisit
		}{
			{
				sendAfter: time.NewTimer(0),
				msg: proto.MsgSiteVisit{
					SiteID:      siteID,
					Populations: []proto.PopulationCount{{Species: species, Count: 63}}},
			},
			{
				sendAfter: time.NewTimer(3 * time.Second),
				msg: proto.MsgSiteVisit{
					SiteID:      siteID,
					Populations: []proto.PopulationCount{{Species: species, Count: 18}}},
			},
			{
				sendAfter: time.NewTimer(6 * time.Second),
				msg: proto.MsgSiteVisit{
					SiteID:      siteID,
					Populations: []proto.PopulationCount{{Species: species, Count: 55}}},
			},
			{
				sendAfter: time.NewTimer(9 * time.Second),
				msg: proto.MsgSiteVisit{
					SiteID:      siteID,
					Populations: []proto.PopulationCount{{Species: species, Count: 63}}},
			},
		}

		for _, o := range observations {
			<-o.sendAfter.C
			log.Println("Sending observation")
			msg, err = o.msg.MarshalBinary()
			require.NoError(t, err)

			_, err = fieldClient.Write(msg)
			require.NoError(t, err)
		}

		for i := 0; i < len(observations)-1; i++ {
			<-authServer.policyChange
		}

		// The Pestcontrol service should have
		// - created a new "Conserve" policy on the Authority server for the specified site
		authSrvPolicies, ok := authServer.sites[siteID].policies[species]
		require.True(t, ok, "Authority server should have a policy for the site")
		require.Len(t, authSrvPolicies, 1)

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
				require.Equal(t, pestcontrol.Cull, policy.Action)
				return
			}
		}
	})

}

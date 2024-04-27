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
		mustHelloHandshake(t, fieldClient)

		// After "Hello" response,
		// Send the Site Visit observation (species counts)
		observation := proto.MsgSiteVisit{
			SiteID:      siteID,
			Populations: []proto.PopulationCount{{Species: species, Count: 9}},
		}

		msg, err := observation.MarshalBinary()
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
		mustHelloHandshake(t, fieldClient)

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
			msg, err := o.msg.MarshalBinary()
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

	t.Run("handles observations from multiple sites", func(t *testing.T) {
		db := sqlite.NewDB(dsn)
		err := db.Open(true)
		require.NoError(t, err)
		defer func() {
			_ = db.Close()
		}()
		require.NoError(t, err)

		pestcontrolAddr := "localhost:12345"
		authorityAddr := "localhost:20547"

		siteAlpha := site{
			id: 9876543,
			targetPopulations: map[string]pestcontrol.TargetPopulation{
				"blue hoofed pegacorn": {Min: 10, Max: 20},
			},
			policies: map[string][]pestcontrol.Policy{},
		}
		siteBravo := site{
			id: 1234567,
			targetPopulations: map[string]pestcontrol.TargetPopulation{
				"giant tardigrade": {Min: 1, Max: 5},
			},
			policies: map[string][]pestcontrol.Policy{},
		}

		// Set up mock Authority server
		authServer := NewMockAuthorityServer()
		for _, s := range []site{siteAlpha, siteBravo} {
			authServer.sites[s.id] = s
		}

		go func() {
			err := authServer.ListenAndServe(authorityAddr)
			if err != nil {
				t.Log(err)
				return
			}
		}()
		defer func() { _ = authServer.Close() }()

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
		defer func() { _ = srv.Close() }()

		// wait for server to start
		time.Sleep(500 * time.Millisecond)

		fieldClient, err := net.Dial("tcp", pestcontrolAddr)
		require.NoError(t, err)
		defer func() { _ = fieldClient.Close() }()

		// Clients sends initial "Hello" message
		mustHelloHandshake(t, fieldClient)

		// Have Pestcontrol service set a policy for the first site
		type clientLabel string
		var (
			alpha clientLabel = "alpha"
			bravo clientLabel = "bravo"
		)

		observations := []struct {
			client    clientLabel
			sendAfter *time.Timer
			msg       proto.MsgSiteVisit
		}{
			{
				client:    alpha,
				sendAfter: time.NewTimer(0),
				// Should set a "Conserve" policy at site Alpha
				// with policy ID 1
				msg: proto.MsgSiteVisit{
					SiteID:      siteAlpha.id,
					Populations: []proto.PopulationCount{{Species: "blue hoofed pegacorn", Count: 1}},
				},
			},
			{
				client:    bravo,
				sendAfter: time.NewTimer(1 * time.Second),
				// Should set a "Cull" policy at site Bravo
				// with policy ID 1 (different site)
				msg: proto.MsgSiteVisit{
					SiteID:      siteBravo.id,
					Populations: []proto.PopulationCount{{Species: "giant tardigrade", Count: 16}},
				},
			},
			{
				client:    bravo,
				sendAfter: time.NewTimer(2 * time.Second),
				// Should remove the "Cull" policy at site Bravo
				msg: proto.MsgSiteVisit{
					SiteID:      siteBravo.id,
					Populations: []proto.PopulationCount{{Species: "giant tardigrade", Count: 3}},
				},
			},
		}

		for _, o := range observations {
			<-o.sendAfter.C
			log.Println("Sending observation")
			msg, err := o.msg.MarshalBinary()
			require.NoError(t, err)

			_, err = fieldClient.Write(msg)
			require.NoError(t, err)
		}

		// TODO: Figure out a more reliable way to wait for the policy changes
		time.Sleep(4 * time.Second)

		// The Pestcontrol service should have
		// - created a new "Conserve" policy on the Authority server for site Alpha
		authSrvPolicies, ok := authServer.sites[siteAlpha.id].policies["blue hoofed pegacorn"]
		require.True(t, ok, "Authority server should have a policy for the site")
		require.Len(t, authSrvPolicies, 1)

		// - and should have no policy for site Bravo
		_, ok = authServer.sites[siteBravo.id].policies["giant tardigrade"]
		require.True(t, ok, "Authority server should have previously created a 'Cull' policy for the site")
		require.Len(t, authSrvPolicies, 0, "Authority server should have deleted the policy after population settled in the target range")

	})
}

func mustHelloHandshake(t *testing.T, c net.Conn) {
	t.Helper()

	helloMsg := proto.MsgHello{}
	msg, err := helloMsg.MarshalBinary()
	require.NoError(t, err)

	_, err = c.Write(msg)
	require.NoError(t, err)

	respData := make([]byte, 25)
	nRead, err := c.Read(respData)
	require.NoError(t, err)

	var resp proto.Message
	_, err = resp.ReadFrom(bytes.NewReader(respData[:nRead]))
	require.NoError(t, err)
	_, err = resp.ToMsgHello()
	require.NoError(t, err)
}

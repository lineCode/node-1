/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package connection

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mysteriumnetwork/node/communication"
	"github.com/mysteriumnetwork/node/identity"
	"github.com/mysteriumnetwork/node/server"
	"github.com/mysteriumnetwork/node/service_discovery/dto"
	"github.com/mysteriumnetwork/node/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type testContext struct {
	suite.Suite
	fakeConnectionFactory *connectionFactoryFake
	connManager           *connectionManager
	fakeDiscoveryClient   *server.ClientFake
	fakeStatsKeeper       *fakeSessionStatsKeeper
	fakeDialog            *fakeDialog
	fakePromiseIssuer     *fakePromiseIssuer
	sync.RWMutex
}

var (
	myID                  = identity.FromAddress("identity-1")
	activeProviderID      = identity.FromAddress("fake-node-1")
	activeProviderContact = dto.Contact{}
	activeServiceType     = "fake-service"
	activeProposal        = dto.ServiceProposal{
		ProviderID:       activeProviderID.Address,
		ProviderContacts: []dto.Contact{activeProviderContact},
		ServiceType:      activeServiceType,
	}
	establishedSessionID = session.ID("session-100")
)

func (tc *testContext) SetupTest() {
	tc.Lock()
	defer tc.Unlock()

	tc.fakeDiscoveryClient = server.NewClientFake()
	tc.fakeDiscoveryClient.RegisterProposal(activeProposal, nil)

	tc.fakeDialog = &fakeDialog{sessionID: establishedSessionID}
	dialogCreator := func(consumer, provider identity.Identity, contact dto.Contact) (communication.Dialog, error) {
		tc.RLock()
		defer tc.RUnlock()
		return tc.fakeDialog, nil
	}

	tc.fakePromiseIssuer = &fakePromiseIssuer{}
	promiseIssuerFactory := func(_ identity.Identity, _ communication.Dialog) PromiseIssuer {
		return tc.fakePromiseIssuer
	}

	tc.fakeConnectionFactory = &connectionFactoryFake{
		mockError: nil,
		mockConnection: &connectionFake{
			nil,
			[]fakeState{
				processStarted,
				connectingState,
				waitState,
				authenticatingState,
				getConfigState,
				assignIPState,
				connectedState,
			},
			[]fakeState{
				exitingState,
				processExited,
			},
			nil,
			sync.WaitGroup{},
			sync.RWMutex{},
		},
	}

	tc.fakeStatsKeeper = &fakeSessionStatsKeeper{}

	tc.connManager = NewManager(tc.fakeDiscoveryClient, dialogCreator, promiseIssuerFactory, tc.fakeConnectionFactory.CreateConnection, tc.fakeStatsKeeper)
}

func (tc *testContext) TestWhenNoConnectionIsMadeStatusIsNotConnected() {
	assert.Exactly(tc.T(), statusNotConnected(), tc.connManager.Status())
}

func (tc *testContext) TestWithUnknownProviderConnectionIsNotMade() {
	noProposalsError := errors.New("provider has no service proposals")

	assert.Equal(tc.T(), noProposalsError, tc.connManager.Connect(myID, identity.FromAddress("unknown-node"), ConnectParams{}))
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())
}

func (tc *testContext) TestOnConnectErrorStatusIsNotConnected() {
	tc.fakeConnectionFactory.mockError = errors.New("fatal connection error")

	assert.Error(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())
	assert.True(tc.T(), tc.fakeDialog.closed)
}

func (tc *testContext) TestWhenManagerMadeConnectionStatusReturnsConnectedStateAndSessionId() {
	err := tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	assert.NoError(tc.T(), err)
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())
	assert.True(tc.T(), tc.fakeStatsKeeper.sessionStartMarked)
}

func (tc *testContext) TestStatusReportsConnectingWhenConnectionIsInProgress() {
	tc.fakeConnectionFactory.mockConnection.onStartReportStates = []fakeState{}

	go func() {
		tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	}()

	waitABit()

	assert.Equal(tc.T(), statusConnecting(), tc.connManager.Status())
	tc.connManager.Disconnect()
}

func (tc *testContext) TestStatusReportsDisconnectingThenNotConnected() {
	tc.fakeConnectionFactory.mockConnection.onStopReportStates = []fakeState{}
	err := tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	assert.NoError(tc.T(), err)
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())

	assert.NoError(tc.T(), tc.connManager.Disconnect())
	assert.Equal(tc.T(), statusDisconnecting(), tc.connManager.Status())
	tc.fakeConnectionFactory.mockConnection.reportState(exitingState)
	tc.fakeConnectionFactory.mockConnection.reportState(processExited)
	waitABit()
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())
	assert.True(tc.T(), tc.fakeStatsKeeper.sessionEndMarked)
}

func (tc *testContext) TestConnectResultsInAlreadyConnectedErrorWhenConnectionExists() {
	assert.NoError(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	assert.Equal(tc.T(), ErrAlreadyExists, tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
}

func (tc *testContext) TestDisconnectReturnsErrorWhenNoConnectionExists() {
	assert.Equal(tc.T(), ErrNoConnection, tc.connManager.Disconnect())
}

func (tc *testContext) TestReconnectingStatusIsReportedWhenOpenVpnGoesIntoReconnectingState() {
	assert.NoError(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	tc.fakeConnectionFactory.mockConnection.reportState(reconnectingState)
	waitABit()
	assert.Equal(tc.T(), statusReconnecting(), tc.connManager.Status())
}

func (tc *testContext) TestDoubleDisconnectResultsInError() {
	assert.NoError(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())
	assert.NoError(tc.T(), tc.connManager.Disconnect())
	waitABit()
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())
	assert.Equal(tc.T(), ErrNoConnection, tc.connManager.Disconnect())
}

func (tc *testContext) TestTwoConnectDisconnectCyclesReturnNoError() {
	assert.NoError(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())
	assert.NoError(tc.T(), tc.connManager.Disconnect())
	waitABit()
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())

	assert.NoError(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())
	assert.NoError(tc.T(), tc.connManager.Disconnect())
	waitABit()
	assert.Equal(tc.T(), statusNotConnected(), tc.connManager.Status())

}

func (tc *testContext) TestConnectFailsIfConnectionFactoryReturnsError() {
	tc.fakeConnectionFactory.mockError = errors.New("failed to create vpn instance")
	assert.Error(tc.T(), tc.connManager.Connect(myID, activeProviderID, ConnectParams{}))
}

func (tc *testContext) TestStatusIsConnectedWhenConnectCommandReturnsWithoutError() {
	tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	assert.Equal(tc.T(), statusConnected(establishedSessionID), tc.connManager.Status())
}

func (tc *testContext) TestConnectingInProgressCanBeCanceled() {
	tc.fakeConnectionFactory.mockConnection.onStartReportStates = []fakeState{}
	connectWaiter := &sync.WaitGroup{}
	connectWaiter.Add(1)
	var err error
	go func() {
		defer connectWaiter.Done()
		err = tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	}()

	waitABit()
	assert.Equal(tc.T(), statusConnecting(), tc.connManager.Status())
	assert.NoError(tc.T(), tc.connManager.Disconnect())

	connectWaiter.Wait()

	assert.Equal(tc.T(), ErrConnectionCancelled, err)
}

func (tc *testContext) TestConnectMethodReturnsErrorIfConnectionExitsDuringConnect() {
	tc.fakeConnectionFactory.mockConnection.onStartReportStates = []fakeState{}
	tc.fakeConnectionFactory.mockConnection.onStopReportStates = []fakeState{}
	connectWaiter := sync.WaitGroup{}
	connectWaiter.Add(1)

	var err error
	go func() {
		defer connectWaiter.Done()
		err = tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	}()
	waitABit()
	tc.fakeConnectionFactory.mockConnection.reportState(processExited)
	connectWaiter.Wait()
	assert.Equal(tc.T(), ErrConnectionFailed, err)
}

func (tc *testContext) Test_PromiseIssuer_WhenManagerMadeConnectionIsStarted() {
	err := tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	assert.NoError(tc.T(), err)
	assert.True(tc.T(), tc.fakePromiseIssuer.startCalled)
}

func (tc *testContext) Test_PromiseIssuer_OnConnectErrorIsStopped() {
	tc.fakeConnectionFactory.mockError = errors.New("fatal connection error")

	err := tc.connManager.Connect(myID, activeProviderID, ConnectParams{})
	assert.Error(tc.T(), err)
	assert.True(tc.T(), tc.fakePromiseIssuer.stopCalled)
}

func TestConnectionManagerSuite(t *testing.T) {
	suite.Run(t, new(testContext))
}

func waitABit() {
	//usually time.Sleep call gives a chance for other goroutines to kick in
	//important when testing async code
	time.Sleep(10 * time.Millisecond)
}

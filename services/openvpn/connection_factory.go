/*
 * Copyright (C) 2018 The "MysteriumNetwork/node" Authors.
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

package openvpn

import (
	"time"

	"github.com/mysteriumnetwork/go-openvpn/openvpn/management"
	"github.com/mysteriumnetwork/go-openvpn/openvpn/middlewares/client/auth"
	openvpn_bytescount "github.com/mysteriumnetwork/go-openvpn/openvpn/middlewares/client/bytescount"
	"github.com/mysteriumnetwork/go-openvpn/openvpn/middlewares/state"
	"github.com/mysteriumnetwork/node/client/stats"
	"github.com/mysteriumnetwork/node/core/connection"
	"github.com/mysteriumnetwork/node/core/location"
	"github.com/mysteriumnetwork/node/identity"
	"github.com/mysteriumnetwork/node/server"
	"github.com/mysteriumnetwork/node/services/openvpn/middlewares/client/bytescount"
	openvpn_session "github.com/mysteriumnetwork/node/services/openvpn/session"
	"github.com/mysteriumnetwork/node/session"
)

// ProcessBasedConnectionFactory represents a factory for creating process-based openvpn connections
type ProcessBasedConnectionFactory struct {
	mysteriumAPIClient    server.Client
	openvpnBinary         string
	configDirectory       string
	runtimeDirectory      string
	originalLocationCache location.Cache
	signerFactory         identity.SignerFactory
	statsKeeper           stats.SessionStatsKeeper
}

// NewProcessBasedConnectionFactory creates a new ProcessBasedConnectionFactory
func NewProcessBasedConnectionFactory(
	mysteriumAPIClient server.Client,
	openvpnBinary, configDirectory, runtimeDirectory string,
	statsKeeper stats.SessionStatsKeeper,
	originalLocationCache location.Cache,
	signerFactory identity.SignerFactory,
) *ProcessBasedConnectionFactory {
	return &ProcessBasedConnectionFactory{
		mysteriumAPIClient:    mysteriumAPIClient,
		openvpnBinary:         openvpnBinary,
		configDirectory:       configDirectory,
		runtimeDirectory:      runtimeDirectory,
		statsKeeper:           statsKeeper,
		originalLocationCache: originalLocationCache,
		signerFactory:         signerFactory,
	}
}

func (op *ProcessBasedConnectionFactory) newAuthMiddleware(sessionID session.ID, signer identity.Signer) management.Middleware {
	credentialsProvider := openvpn_session.SignatureCredentialsProvider(sessionID, signer)
	return auth.NewMiddleware(credentialsProvider)
}

func (op *ProcessBasedConnectionFactory) newBytecountMiddleware() management.Middleware {
	statsSaver := bytescount.NewSessionStatsSaver(op.statsKeeper)
	return openvpn_bytescount.NewMiddleware(statsSaver, 1*time.Second)
}

func (op *ProcessBasedConnectionFactory) newStateMiddleware(session session.ID, signer identity.Signer, connectionOptions connection.ConnectOptions, stateChannel connection.StateChannel) management.Middleware {
	// TODO Move stats.Sender from 'openvpn' package
	statsSender := stats.NewRemoteStatsSender(
		op.statsKeeper,
		op.mysteriumAPIClient,
		session,
		connectionOptions.ProviderID,
		signer,
		op.originalLocationCache.Get().Country,
		time.Minute,
	)
	stateCallback := GetStateCallback(stateChannel)
	return state.NewMiddleware(stateCallback, statsSender.StateHandler)
}

// CreateConnection implements the connection.ConnectionCreator interface
func (op *ProcessBasedConnectionFactory) CreateConnection(options connection.ConnectOptions, stateChannel connection.StateChannel) (connection.Connection, error) {
	vpnClientConfig, err := NewClientConfigFromSession(options.SessionConfig, op.configDirectory, op.runtimeDirectory)
	if err != nil {
		return nil, err
	}

	signer := op.signerFactory(options.ConsumerID)

	stateMiddleware := op.newStateMiddleware(options.SessionID, signer, options, stateChannel)
	authMiddleware := op.newAuthMiddleware(options.SessionID, signer)
	byteCountMiddleware := op.newBytecountMiddleware()

	return NewClient(
		op.openvpnBinary,
		vpnClientConfig,
		stateMiddleware,
		byteCountMiddleware,
		authMiddleware,
	), nil
}

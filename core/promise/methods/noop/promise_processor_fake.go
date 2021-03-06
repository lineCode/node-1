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

package noop

import discovery_dto "github.com/mysteriumnetwork/node/service_discovery/dto"

// FakePromiseEngine do nothing. It required for the temporary --experiment-promise-check flag.
// TODO it should be removed once --experiment-promise-check will be deleted.
type FakePromiseEngine struct{}

// Start fakes promise engine start
func (*FakePromiseEngine) Start(_ discovery_dto.ServiceProposal) error {
	return nil
}

// Stop fakes promise engine stop
func (*FakePromiseEngine) Stop() error {
	return nil
}

// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2015-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/wire"
)

// activeNetParams is a pointer to the parameters specific to the
// currently active decred network.
var activeNetParams = &testNet3Params

// params is used to group parameters for various networks such as the main
// network and test networks.
type params struct {
	*chaincfg.Params
	WalletRPCServerPort string
}

// testNet3Params contains parameters specific to the test network (version 0)
// (wire.TestNet).  NOTE: The RPC port is intentionally different than the
// reference implementation - see the mainNetParams comment for details.

var testNet3Params = params{
	Params:              chaincfg.TestNet3Params(),
	WalletRPCServerPort: "19110",
}

// netName returns the name used when referring to a decred network.  At the
// time of writing, dcrd currently places blocks for testnet version 0 in the
// data and log directory "testnet", which does not match the Name field of the
// chaincfg parameters.  This function can be used to override this directory name
// as "testnet" when the passed active network matches wire.TestNet.
//
// A proper upgrade to move the data and log directories for this network to
// "testnet" is planned for the future, at which point this function can be
// removed and the network parameter's name used instead.
func netName(chainParams *params) string {
	switch chainParams.Net {
	case wire.TestNet3:
		return "testnet3"
	default:
		return chainParams.Name
	}
}

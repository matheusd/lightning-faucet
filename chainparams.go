package main

import (
	"github.com/decred/dcrd/chaincfg"
)

// params is used to group parameters for various networks such as the main
// network and test networks.
type params struct {
	*chaincfg.Params
	rpcPort string
}

var (
	// decredMainNetParams contains parameters specific to the main network
	// (wire.MainNet).
	decredMainNetParams = params{
		Params:  &chaincfg.MainNetParams,
		rpcPort: "10009",
	}

	// decredTestNet3Params contains parameters specific to the test network
	// (wire.TestNet3).
	decredTestNet3Params = params{
		Params:  &chaincfg.TestNet3Params,
		rpcPort: "10009",
	}

	// decredSimNetParams contains parameters specific to the simulation test network
	// (wire.SimNet).
	decredSimNetParams = params{
		Params:  &chaincfg.SimNetParams,
		rpcPort: "10009",
	}
)

// activeNetParams is a pointer to the parameters specific to the
// currently active Decred network. (default: --testnet)
var activeNetParams = &decredTestNet3Params

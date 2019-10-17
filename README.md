lightning-faucet
================

[![Build Status](https://github.com/decred/lightning-faucet/workflows/Build%20and%20Test/badge.svg)](https://github.com/decred/lightning-faucet/actions)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](http://copyfree.org)

## Lightning Network Faucet Overview
The Lightning Network Faucet is a faucet that is currently deployed on the
Decred testnet. The following faucets are currently available:

- https://testnet-dcrln-01.matheusd.com
- https://testnet-dcrln-01.davec.name

The Testnet Lightning Faucet (TLF) is similar to other existing Decred
faucets.  However, rather then sending dcr directly on-chain to a user of
the faucet, the TLF will instead open a payment channel with the target user.
The user can then either use their new link to the Lightning Network to
facilitate payments, or immediately close the channel (which immediately
credits them on-chain like regular faucets).

Currently the TLF is only compatible with `dcrlnd`.

## Installation

In order to build from source, the following build dependencies are
required:

* **Go:** Installation instructions can be found [here](https://golang.org/doc/install).

Minimum Go version supported is 1.11. This project uses go modules, so either
compile it with GO111MODULES=on or outside of the $GOPATH.

With the preliminary steps completed, to install the Testnet Lightning Faucet

```no-highlight
$ git clone https://github.com/decred/lightning-faucet src/github.com/decred/lightning-faucet
$ cd src/github.com/decred/lightning-faucet
$ go install -v
```

## Deploying The Faucet

Once you have the faucet installed, you'll need to ensure you have a local
[`dcrlnd`](https://github.com/decred/dcrlnd) active and fully synced.

Once the node is synced, execute the following command (from this directory) to
deploy the faucet:

```no-highlight
lightning-faucet --lnd_node=X.X.X.X:10009
```

Where `X.X.X.X:10009` is the IP address and port for your active `dcrlnd` node.

To enable HTTPS support via [Let's Encrypt](https://letsencrypt.org), specify
a few additional options:

```no-highlight
lightning-faucet -lnd_node=X.X.X.X:10009 -use_le_https -domain my-faucet-domain.example.com
```

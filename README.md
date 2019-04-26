## Lightning Network Faucet

[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/lightninglabs/lightning-faucet/blob/master/LICENSE) 
&nbsp;&nbsp;&nbsp;&nbsp;

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
  
  * **Go:** Installation instructions can be found [here](http://golang.org/doc/install). 

  Minimum Go version supported is 1.11. This project uses go modules, so either
  compile it with GO111MODULES=on or outside of the $GOPATH.

With the preliminary steps completed, to install the Testnet Lightning Faucet
```
$ git clone https://github.com/decred/lightning-faucet src/github.com/decred/lightning-faucet
$ cd src/github.com/decred/lightning-faucet
$ go install -v
```

## Deploying The Faucet

Once you have the faucet installed, you'll need to ensure you have a local
[`dcrlnd`](https://github.com/decred/dcrlnd) active and fully synced.

Once the node is synced, execute the following command (from this directory) to
deploy the faucet:
```
lightning-faucet --lnd_ip=X.X.X.X
```

Where `X.X.X.X` is the public, reachable IP address for your active `dcrlnd` node.

To enable HTTPS support via [Let's Encrypt](https://letsencrypt.org), specify 
a few additional options:

```
lightning-faucet -lnd_ip=X.X.X.X -use_le_https -domain my-faucet-domain.example.com
```


module github.com/decred/lightning-faucet/main

go 1.12

require (
	github.com/btcsuite/btcd v0.0.0-20180903232927-cff30e1d23fc
	github.com/btcsuite/btcutil v0.0.0-20180706230648-ab6388e0c60a
	github.com/btcsuite/btcwallet v0.0.0-20180904010540-8ae4afc70174
	github.com/btcsuite/golangcrypto v0.0.0-20150304025918-53f62d9b43e8
	github.com/coreos/bbolt v0.0.0-20180223184059-4f5275f4ebbf
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/dcrutil v1.2.0
	github.com/decred/dcrd/wire v1.2.0
	github.com/decred/dcrlnd v0.2.1-alpha
	github.com/golang/crypto v0.0.0-20180904163835-0709b304e793
	github.com/golang/protobuf v1.2.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.6.2
	github.com/grpc-ecosystem/grpc-gateway v1.6.4
	github.com/juju/loggo v0.0.0-20180524022052-584905176618
	github.com/lightningnetwork/lnd v0.0.0-20180906040142-fb95858afce6
	github.com/rogpeppe/fastuuid v0.0.0-20150106093220-6724a57986af
	golang.org/x/crypto v0.0.0-20190320223903-b7391e95e576
	golang.org/x/net v0.0.0-20190125091013-d26f9f9a57f3
	golang.org/x/sys v0.0.0-20190318195719-6c81ef8f67ca
	golang.org/x/text v0.3.0
	google.golang.org/genproto v0.0.0-20190111180523-db91494dd46c
	google.golang.org/grpc v1.18.0
	gopkg.in/errgo.v1 v1.0.0
	gopkg.in/macaroon-bakery.v2 v2.1.0
	gopkg.in/macaroon.v2 v2.0.0
)

replace (
	github.com/decred/dcrd => github.com/decred/dcrd v0.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrd/bech32 => github.com/decred/dcrd/bech32 v0.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrd/blockchain => github.com/decred/dcrd/blockchain v0.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrd/connmgr => github.com/matheusd/dcrd/connmgr v0.0.0-20190410055418-133f994b52da
	github.com/decred/dcrd/dcrjson/v2 => github.com/decred/dcrd/dcrjson/v2 v2.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrd/peer => github.com/decred/dcrd/peer v0.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrd/rpcclient/v2 => github.com/decred/dcrd/rpcclient/v2 v2.0.0-20190306151227-8cbb5ae69df7
	github.com/decred/dcrlnd => github.com/matheusd/dcrlnd v0.0.0-20190423164340-0f1927992ec2
	github.com/decred/dcrlnd/macaroons => github.com/matheusd/dcrlnd/macaroons v0.0.0-20190423164340-0f1927992ec2

	github.com/decred/dcrwallet => github.com/decred/dcrwallet v0.0.0-20190322135901-7e0e5a4227d7
	github.com/decred/dcrwallet/wallet/v2 => github.com/decred/dcrwallet/wallet/v2 v2.0.0-20190322135901-7e0e5a4227d7

	github.com/decred/lightning-onion => github.com/decred/lightning-onion v0.0.0-20190321210301-95556fb4cc37

)

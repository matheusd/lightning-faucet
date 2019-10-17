module github.com/decred/lightning-faucet

go 1.12

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/chaincfg v1.3.0
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/dcrutil v1.2.0
	github.com/decred/dcrd/wire v1.2.0
	github.com/decred/dcrlnd v0.1.1-0.20190528130025-71d9ffc3f0bf
	github.com/decred/slog v1.0.0
	github.com/golang/crypto v0.0.0-20180904163835-0709b304e793
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/mux v1.6.2
	github.com/jessevdk/go-flags v1.4.0
	github.com/jrick/logrotate v1.0.0
	google.golang.org/grpc v1.18.0
	gopkg.in/macaroon.v2 v2.0.0
)

replace (
	github.com/decred/dcrd => github.com/decred/dcrd v0.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrd/bech32 => github.com/decred/dcrd/bech32 v0.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrd/blockchain => github.com/decred/dcrd/blockchain v0.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrd/dcrjson/v2 => github.com/decred/dcrd/dcrjson/v2 v2.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrd/peer => github.com/decred/dcrd/peer v0.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrd/rpcclient/v2 => github.com/decred/dcrd/rpcclient/v2 v2.0.0-20190322200752-8cbb5ae69df7
	github.com/decred/dcrlnd/macaroons => github.com/matheusd/dcrlnd/macaroons v0.0.0-20190423164340-0f1927992ec2

	github.com/decred/dcrwallet => github.com/decred/dcrwallet v0.0.0-20190322135901-7e0e5a4227d7
	github.com/decred/dcrwallet/wallet/v2 => github.com/decred/dcrwallet/wallet/v2 v2.0.0-20190322135901-7e0e5a4227d7

	github.com/decred/lightning-onion => github.com/decred/lightning-onion v0.0.0-20190321210301-95556fb4cc37

)

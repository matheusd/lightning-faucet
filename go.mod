module github.com/decred/lightning-faucet

go 1.12

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/chaincfg v1.5.2
	github.com/decred/dcrd/chaincfg/chainhash v1.0.2
	github.com/decred/dcrd/dcrutil/v2 v2.0.1
	github.com/decred/dcrd/wire v1.3.0
	github.com/decred/dcrlnd v0.2.1
	github.com/decred/slog v1.0.0
	github.com/gorilla/mux v1.7.4
	github.com/jessevdk/go-flags v1.4.0
	github.com/jrick/logrotate v1.0.0
	golang.org/x/crypto v0.0.0-20190829043050-9756ffdc2472
	google.golang.org/grpc v1.28.0
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

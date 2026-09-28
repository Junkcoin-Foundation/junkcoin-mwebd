module github.com/JunkoinFoundation/junkcoin-mwebd

go 1.26.0

require (
	github.com/JunkoinFoundation/junkcoin-mwebd/sign v0.1.0
	github.com/btcsuite/btclog v1.0.0
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0
	github.com/ethereum/go-ethereum v1.14.8
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/ltcmweb/coinswapd v0.1.0
	github.com/ltcmweb/ltcd v0.25.14
	github.com/ltcmweb/ltcd/chaincfg/chainhash v1.0.3
	github.com/ltcmweb/neutrino v0.17.4
	github.com/ltcsuite/ltcwallet/walletdb v1.3.5
	golang.org/x/net v0.59.0
	google.golang.org/grpc v1.75.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

replace github.com/JunkoinFoundation/junkcoin-mwebd/sign v0.1.0 => ./sign

replace github.com/ltcmweb/ltcd => ./patched/ltcd

replace github.com/ltcmweb/neutrino => ./patched/neutrino

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/aead/siphash v1.0.1 // indirect
	github.com/alitto/pond/v2 v2.2.0 // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.1.0 // indirect
	github.com/decred/dcrd/lru v1.1.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/holiman/uint256 v1.3.1 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/ltcmweb/ltcd/btcec/v2 v2.3.3 // indirect
	github.com/ltcmweb/neutrino/cache v1.1.0 // indirect
	github.com/ltcmweb/secp256k1 v0.1.6 // indirect
	github.com/ltcsuite/lnd/queue v1.1.0 // indirect
	github.com/ltcsuite/lnd/ticker v1.1.0 // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	go.etcd.io/bbolt v1.3.10 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/exp v0.0.0-20231110203233-9a3e6036ecaa // indirect
	golang.org/x/mobile v0.0.0-20260908204917-8b95e45f8d3e // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/tools v0.50.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
)

tool golang.org/x/mobile/cmd/gobind

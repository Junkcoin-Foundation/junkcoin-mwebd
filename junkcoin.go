package mwebd

import (
	"errors"
	"math/big"
	"time"

	"github.com/ltcmweb/ltcd/chaincfg"
	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
	"github.com/ltcmweb/ltcd/wire"
)

var (
	junkMainNet  chaincfg.Params
	junkTestNet4 chaincfg.Params
)

func init() {
	genesisHash, _ := chainhash.NewHashFromStr("a2effa738145e377e08a61d76179c21703e13e48910b30a2a87f0dfe794b64c6")
	merkleRoot, _ := chainhash.NewHashFromStr("3de124b0274307911fe12550e96bf76cb92c12835db6cb19f82658b8aca1dbc8")

	mainPowLimit := new(big.Int)
	mainPowLimit.SetString("00000fffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)

	genesisBlock := wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:    1,
			PrevBlock:  chainhash.Hash{},
			MerkleRoot: *merkleRoot,
			Timestamp:  time.Unix(1367394064, 0),
			Bits:       0x1e0ffff0,
			Nonce:      112158625,
		},
		Transactions: []*wire.MsgTx{{
			Version: 1,
			TxIn: []*wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				// Real junkcoin genesis coinbase (junkcoind getblock a2effa73...):
				// scriptSig = 04ffff001d 0104 4c5d <93-byte msg "Wed May 1, 2013: ...">.
				// The old bytes were Litecoin testnet's "Spot gold fell..." coinbase with a
				// bogus P2PK script, so BuildBasicFilter(genesis) produced fh@0=d0b86409...
				// while junkcoind's filter index has 7e26bd8c... -- neutrino then banned
				// every peer with "peer served invalid filter header checkpoint".
				SignatureScript: []byte{
					0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, 0x4c, 0x5d, 0x57, 0x65, 0x64,
					0x20, 0x4d, 0x61, 0x79, 0x20, 0x31, 0x2c, 0x20, 0x32, 0x30, 0x31, 0x33,
					0x3a, 0x20, 0x53, 0x70, 0x6f, 0x74, 0x20, 0x67, 0x6f, 0x6c, 0x64, 0x20,
					0x66, 0x65, 0x6c, 0x6c, 0x20, 0x31, 0x2e, 0x33, 0x20, 0x70, 0x65, 0x72,
					0x63, 0x65, 0x6e, 0x74, 0x20, 0x74, 0x6f, 0x20, 0x24, 0x31, 0x2c, 0x34,
					0x35, 0x37, 0x2e, 0x39, 0x30, 0x20, 0x61, 0x6e, 0x20, 0x6f, 0x75, 0x6e,
					0x63, 0x65, 0x20, 0x62, 0x79, 0x20, 0x33, 0x3a, 0x31, 0x31, 0x20, 0x70,
					0x2e, 0x6d, 0x2e, 0x20, 0x45, 0x44, 0x54, 0x20, 0x28, 0x31, 0x39, 0x31,
					0x31, 0x20, 0x47, 0x4d, 0x54, 0x29,
				},
				Sequence: 0xffffffff,
			}},
			TxOut: []*wire.TxOut{{
				Value: 50 * 100000000,
				// Real junkcoin genesis coinbase output: P2PK 65-byte pubkey.
				PkScript: []byte{
					0x41, 0x04, 0x01, 0x84, 0x71, 0x0f, 0xa6, 0x89, 0xad, 0x50, 0x23, 0x69,
					0x0c, 0x80, 0xf3, 0xa4, 0x9c, 0x8f, 0x13, 0xf8, 0xd4, 0x5b, 0x8c, 0x85,
					0x7f, 0xbc, 0xbc, 0x8b, 0xc4, 0xa8, 0xe4, 0xd3, 0xeb, 0x4b, 0x10, 0xf4,
					0xd4, 0x60, 0x4f, 0xa0, 0x8d, 0xce, 0x60, 0x1a, 0xaf, 0x0f, 0x47, 0x02,
					0x16, 0xfe, 0x1b, 0x51, 0x85, 0x0b, 0x4a, 0xcf, 0x21, 0xb1, 0x79, 0xc4,
					0x50, 0x70, 0xac, 0x7b, 0x03, 0xa9, 0xac,
				},
			}},
			LockTime: 0,
		}},
	}

	junkMainNet = chaincfg.Params{
		Name: "mainnet",
		// pchMessageStart {fb,c0,b6,db} in junkcoin/src/chainparams.cpp, but
		// wire serializes BitcoinNet little-endian (see ltcd MainNet =
		// 0xdbb6c0fb), so the constant is the byte-reversed array order.
		// The old value 0xfbc0b6db sent db b6 c0 fb and peers answered with
		// "HEADER ERROR - MESSAGESTART".
		Net:         wire.BitcoinNet(0xdbb6c0fb),
		DefaultPort: "9771",
		DNSSeeds: []chaincfg.DNSSeed{
			{Host: "mainnet.junk-coin.com", HasFiltering: true},
			{Host: "junk-seed.s3na.xyz", HasFiltering: true},
			{Host: "jkc-seed.junkiewally.xyz", HasFiltering: true},
		},
		GenesisBlock:             &genesisBlock,
		GenesisHash:              genesisHash,
		PowLimit:                 mainPowLimit,
		PowLimitBits:             0x1e0ffff0,
		BIP0034Height:            8460,
		BIP0065Height:            0,
		BIP0066Height:            0,
		CoinbaseMaturity:         70,
		MwebPegoutMaturity:       6,
		SubsidyReductionInterval: 100000,
		TargetTimespan:           time.Hour * 24,
		TargetTimePerBlock:       time.Minute,
		RetargetAdjustmentFactor: 4,
		ReduceMinDifficulty:      false,
		MinDiffReductionTime:     0,
		GenerateSupported:        false,

		Bech32HRPSegwit: "jc",
		Bech32HRPMweb:   "jcmweb",

		PubKeyHashAddrID: 0x10, // 16 -> addresses start with '7'
		ScriptHashAddrID: 0x05, // 5 -> addresses start with '3'
		PrivateKeyID:     0x90, // 144 -> WIF starts with 'N'
	}

	junkTestNet4 = chaincfg.Params{
		Name: "testnet4",
		// pchMessageStart {fc,c1,b7,dc}: wire writes BitcoinNet little-endian,
		// so the constant must be byte-reversed (0xdcb7c1fc). The old value
		// 0xfcc1b7dc sent dc b7 c1 fc and junkcoind answered with
		// "HEADER ERROR - MESSAGESTART (version ...)".
		Net:         wire.BitcoinNet(0xdcb7c1fc),
		DefaultPort: "19771",
		DNSSeeds: []chaincfg.DNSSeed{
			{Host: "testnet.junk-coin.com", HasFiltering: true},
			{Host: "junk-testnet.s3na.xyz", HasFiltering: true},
		},
		GenesisBlock: &genesisBlock,
		GenesisHash:  genesisHash,
		PowLimit:     mainPowLimit,
		// junkcoind computes nProofOfWorkLimit as GetCompact(powLimit) =
		// 0x1e0fffff; testnet min-difficulty blocks carry this value (block 1
		// and 2 observed), so the sanity check must expect it, not the
		// mainnet genesis bits 0x1e0ffff0.
		PowLimitBits:             0x1e0fffff,
		BIP0034Height:            0,
		BIP0065Height:            0,
		BIP0066Height:            0,
		CoinbaseMaturity:         30,
		MwebPegoutMaturity:       6,
		SubsidyReductionInterval: 100000,
		TargetTimespan:           time.Hour * 4,
		TargetTimePerBlock:       time.Minute,
		RetargetAdjustmentFactor: 4,
		ReduceMinDifficulty:      true,
		MinDiffReductionTime:     time.Minute * 2,
		GenerateSupported:        false,

		Bech32HRPSegwit: "tjc",
		Bech32HRPMweb:   "tjcmweb",

		PubKeyHashAddrID: 0x6F, // 111 -> addresses start with 't'
		ScriptHashAddrID: 0x05, // 5 -> script addresses start with '2'
		PrivateKeyID:     0xEF, // 239 -> WIF starts with 'c'
	}

	// Register both networks so ltcutil.DecodeAddress recognizes junkcoin
	// bech32 prefixes ("jc1"/"jcmweb1", "tjc1"/"tjcmweb1").  Without this
	// every bech32/mweb recipient fails with "decoded address is of unknown
	// format".
	//
	// Junkcoin's mainnet reuses Litecoin's message magic (pchMessageStart
	// fb c0 b6 db == wire.MainNet), so chaincfg.Register(&junkMainNet)
	// returns ErrDuplicateNet and — important here — returns BEFORE the
	// address prefixes/IDs are added to the lookup sets.  Register a clone
	// under an unused magic instead: only the address-related sets matter
	// for DecodeAddress, while the params used at runtime (junkMainNet)
	// keeps the real magic for the wire.
	if err := chaincfg.Register(&junkMainNet); err != nil {
		if !errors.Is(err, chaincfg.ErrDuplicateNet) {
			panic("failed to register junkcoin mainnet: " + err.Error())
		}
		alias := junkMainNet
		for net := wire.BitcoinNet(0x4a554e4b); ; net-- {
			alias.Net = net
			err := chaincfg.Register(&alias)
			if err == nil {
				break
			}
			if !errors.Is(err, chaincfg.ErrDuplicateNet) {
				panic("failed to register junkcoin mainnet address sets: " + err.Error())
			}
		}
	}
	if err := chaincfg.Register(&junkTestNet4); err != nil {
		panic("failed to register junkcoin testnet4: " + err.Error())
	}
}

func GetJunkMainNetParams() chaincfg.Params {
	return junkMainNet
}

func GetJunkTestNet4Params() chaincfg.Params {
	return junkTestNet4
}

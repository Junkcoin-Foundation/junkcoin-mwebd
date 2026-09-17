package mwebd

import (
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
			Version:   1,
			PrevBlock: chainhash.Hash{},
			MerkleRoot: *merkleRoot,
			Timestamp: time.Unix(1367394064, 0),
			Bits:      0x1e0ffff0,
			Nonce:     112158625,
		},
		Transactions: []*wire.MsgTx{{
			Version: 1,
			TxIn: []*wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, 0x35, 0x53, 0x70, 0x6f, 0x74, 0x20, 0x67, 0x6f, 0x6c, 0x64, 0x20, 0x66, 0x65, 0x6c, 0x6c, 0x20, 0x31, 0x2e, 0x33, 0x20, 0x70, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x20, 0x74, 0x6f, 0x20, 0x24, 0x31, 0x2c, 0x34, 0x35, 0x37, 0x2e, 0x39, 0x30, 0x20, 0x61, 0x6e, 0x20, 0x6f, 0x75, 0x6e, 0x63, 0x65, 0x20, 0x62, 0x79, 0x20, 0x33, 0x3a, 0x31, 0x31, 0x20, 0x70, 0x2e, 0x6d, 0x2e, 0x45, 0x44, 0x54, 0x20, 0x28, 0x31, 0x39, 0x31, 0x31, 0x20, 0x47, 0x4d, 0x54, 0x29},
				Sequence: 0xffffffff,
			}},
			TxOut: []*wire.TxOut{{
				Value: 50 * 100000000,
				PkScript: []byte{0x04, 0x01, 0x8c, 0x00, 0x1e, 0x01, 0x04, 0x28, 0x30, 0x34, 0x30, 0x38, 0x30, 0x34, 0x30, 0x36, 0x30, 0x39, 0x30, 0x39, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30},
			}},
			LockTime: 0,
		}},
	}

	junkMainNet = chaincfg.Params{
		Name:        "mainnet",
		Net:         wire.BitcoinNet(0xfbc0b6db),
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
		Name:        "testnet4",
		Net:         wire.BitcoinNet(0xfcc1b7dc),
		DefaultPort: "19771",
		DNSSeeds: []chaincfg.DNSSeed{
			{Host: "testnet.junk-coin.com", HasFiltering: true},
			{Host: "junk-testnet.s3na.xyz", HasFiltering: true},
		},
		GenesisBlock:             &genesisBlock,
		GenesisHash:              genesisHash,
		PowLimit:                 mainPowLimit,
		PowLimitBits:             0x1e0ffff0,
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
		MinDiffReductionTime:     time.Minute * 20,
		GenerateSupported:        false,

		Bech32HRPSegwit: "tjc",
		Bech32HRPMweb:   "tjcmweb",

		PubKeyHashAddrID: 0x6F, // 111 -> addresses start with 't'
		ScriptHashAddrID: 0x05, // 5 -> script addresses start with '2'
		PrivateKeyID:     0xEF, // 239 -> WIF starts with 'c'
	}
}

func GetJunkMainNetParams() chaincfg.Params {
	return junkMainNet
}

func GetJunkTestNet4Params() chaincfg.Params {
	return junkTestNet4
}

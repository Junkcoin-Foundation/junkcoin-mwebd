package chainsync

import (
	"fmt"

	"github.com/ltcmweb/ltcd/chaincfg"
	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
	"github.com/ltcmweb/ltcd/wire"
)

// ErrCheckpointMismatch is returned if given filter headers don't pass our
// control check.
var ErrCheckpointMismatch = fmt.Errorf("checkpoint doesn't match")

// filterHeaderCheckpoints holds a mapping from heights to filter headers for
// various heights. We use them to check whether peers are serving us the
// expected filter headers.
var filterHeaderCheckpoints = map[wire.BitcoinNet]map[uint32]*chainhash.Hash{
	// Junkcoin: upstream entries (Litecoin mainnet + testnet4 filter-header
	// checkpoints) were removed.  Junkcoin reuses Litecoin's message magics
	// (0xdbb6c0fb / 0xdcb7c1fc), so chaincfg.TestNet4Params.Net matched
	// Junkcoin's params.Net and every checkpointed height that disagreed was
	// rejected with ErrCheckpointMismatch plus a 24h ban
	// (banman.InvalidFilterHeaderCheckpoint) at blockmanager.go:1412.
	// Peer-provided checkpoints are still cross-checked between independent
	// peers by blockmanager (getCheckpointedCFHeaders), and each cfheader
	// links cryptographically to the next, so nothing here is weakened.
}

// ControlCFHeader controls the given filter header against our list of
// checkpoints. It returns ErrCheckpointMismatch if we have a checkpoint at the
// given height, and it doesn't match.
func ControlCFHeader(params chaincfg.Params, fType wire.FilterType,
	height uint32, filterHeader *chainhash.Hash) error {

	if fType != wire.GCSFilterRegular {
		return fmt.Errorf("unsupported filter type %v", fType)
	}

	control, ok := filterHeaderCheckpoints[params.Net]
	if !ok {
		return nil
	}

	hash, ok := control[height]
	if !ok {
		return nil
	}

	if *filterHeader != *hash {
		return ErrCheckpointMismatch
	}

	return nil
}

// hashFromStr makes a chainhash.Hash from a valid hex string. If the string is
// invalid, a nil pointer will be returned.
func hashFromStr(hexStr string) *chainhash.Hash {
	hash, _ := chainhash.NewHashFromStr(hexStr)
	return hash
}

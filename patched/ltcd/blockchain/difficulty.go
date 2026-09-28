// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"time"

	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
)

var (
	// bigOne is 1 represented as a big.Int.  It is defined here to avoid
	// the overhead of creating it multiple times.
	bigOne = big.NewInt(1)

	// oneLsh256 is 1 shifted left 256 bits.  It is defined here to avoid
	// the overhead of creating it multiple times.
	oneLsh256 = new(big.Int).Lsh(bigOne, 256)
)

// HashToBig converts a chainhash.Hash into a big.Int that can be used to
// perform math comparisons.
func HashToBig(hash *chainhash.Hash) *big.Int {
	// A Hash is in little-endian, but the big package wants the bytes in
	// big-endian, so reverse them.
	buf := *hash
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}

	return new(big.Int).SetBytes(buf[:])
}

// CompactToBig converts a compact representation of a whole number N to an
// unsigned 32-bit number.  The representation is similar to IEEE754 floating
// point numbers.
//
// Like IEEE754 floating point, there are three basic components: the sign,
// the exponent, and the mantissa.  They are broken out as follows:
//
// - the most significant 8 bits represent the unsigned base 256 exponent
// - bit 23 (the 24th bit) represents the sign bit
// - the least significant 23 bits represent the mantissa
//
//	-------------------------------------------------
//	|   Exponent     |    Sign    |    Mantissa     |
//	-------------------------------------------------
//	| 8 bits [31-24] | 1 bit [23] | 23 bits [22-00] |
//	-------------------------------------------------
//
// The formula to calculate N is:
//
//	N = (-1^sign) * mantissa * 256^(exponent-3)
//
// This compact form is only used in litecoin to encode unsigned 256-bit numbers
// which represent difficulty targets, thus there really is not a need for a
// sign bit, but it is implemented here to stay consistent with litecoind.
func CompactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

// BigToCompact converts a whole number N to a compact representation using
// an unsigned 32-bit number.  The compact representation only provides 23 bits
// of precision, so values larger than (2^23 - 1) only encode the most
// significant digits of the number.  See CompactToBig for details.
func BigToCompact(n *big.Int) uint32 {
	// No need to do any work if it's zero.
	if n.Sign() == 0 {
		return 0
	}

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes.  So, shift the number right or left
	// accordingly.  This is equivalent to:
	// mantissa = mantissa / 256^(exponent-3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		// Use a copy to avoid modifying the caller's original number.
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

// CalcWork calculates a work value from difficulty bits. Litecoin increases
// the difficulty for generating a block by decreasing the value which the
// generated hash must be less than.  This difficulty target is stored in each
// block header using a compact representation as described in the documentation
// for CompactToBig.  The main chain is selected by choosing the chain that has
// the most proof of work (highest difficulty).  Since a lower target difficulty
// value equates to higher actual difficulty, the work value which will be
// accumulated must be the inverse of the difficulty.  Also, in order to avoid
// potential division by zero and really small floating point numbers, the
// result adds 1 to the denominator and multiplies the numerator by 2^256.
func CalcWork(bits uint32) *big.Int {
	// Return a work value of zero if the passed difficulty bits represent
	// a negative number. Note this should not happen in practice with valid
	// blocks, but an invalid block could trigger it.
	difficultyNum := CompactToBig(bits)
	if difficultyNum.Sign() <= 0 {
		return big.NewInt(0)
	}

	// (1 << 256) / (difficultyNum + 1)
	denominator := new(big.Int).Add(difficultyNum, bigOne)
	return new(big.Int).Div(oneLsh256, denominator)
}

// calcEasiestDifficulty calculates the easiest possible difficulty that a block
// can have given starting difficulty bits and a duration.  It is mainly used to
// verify that claimed proof of work by a block is sane as compared to a
// known good checkpoint.
func (b *BlockChain) calcEasiestDifficulty(bits uint32, duration time.Duration) uint32 {
	// Convert types used in the calculations below.
	durationVal := int64(duration / time.Second)
	adjustmentFactor := big.NewInt(b.chainParams.RetargetAdjustmentFactor)

	// The test network rules allow minimum difficulty blocks after more
	// than twice the desired amount of time needed to generate a block has
	// elapsed.
	if b.chainParams.ReduceMinDifficulty {
		reductionTime := int64(b.chainParams.MinDiffReductionTime /
			time.Second)
		if durationVal > reductionTime {
			return b.chainParams.PowLimitBits
		}
	}

	// Since easier difficulty equates to higher numbers, the easiest
	// difficulty for a given duration is the largest value possible given
	// the number of retargets for the duration and starting difficulty
	// multiplied by the max adjustment factor.
	newTarget := CompactToBig(bits)
	for durationVal > 0 && newTarget.Cmp(b.chainParams.PowLimit) < 0 {
		newTarget.Mul(newTarget, adjustmentFactor)
		durationVal -= b.maxRetargetTimespan
	}

	// Limit new value to the proof of work limit.
	if newTarget.Cmp(b.chainParams.PowLimit) > 0 {
		newTarget.Set(b.chainParams.PowLimit)
	}

	return BigToCompact(newTarget)
}

// findPrevTestNetDifficulty returns the difficulty of the previous block which
// did not have the special testnet minimum difficulty rule applied.
func findPrevTestNetDifficulty(startNode HeaderCtx, c ChainCtx) uint32 {
	// Search backwards through the chain for the last block without
	// the special rule applied.
	iterNode := startNode
	for iterNode != nil && iterNode.Height()%c.BlocksPerRetarget() != 0 &&
		iterNode.Bits() == c.ChainParams().PowLimitBits {

		iterNode = iterNode.Parent()
	}

	// Return the found difficulty or the minimum difficulty if no
	// appropriate block was found.
	lastBits := c.ChainParams().PowLimitBits
	if iterNode != nil {
		lastBits = iterNode.Bits()
	}
	return lastBits
}

// calcNextRequiredDifficulty calculates the required difficulty for the block
// after the passed previous HeaderCtx based on the difficulty retarget rules.
// This function differs from the exported CalcNextRequiredDifficulty in that
// the exported version uses the current best chain as the previous HeaderCtx
// while this function accepts any block node. This function accepts a ChainCtx
// parameter that gives the necessary difficulty context variables.
//
// Ported from junkcoin's GetNextWorkRequired (src/pow.cpp) and
// CalculateJunkcoinNextWorkRequired (src/junkcoin.cpp): the retarget interval,
// timespan bounds and min-difficulty rules are height dependent, so they cannot
// be expressed with static chaincfg values.
func calcNextRequiredDifficulty(lastNode HeaderCtx, newBlockTime time.Time,
	c ChainCtx) (uint32, error) {

	params := c.ChainParams()

	// Regtest never retargets.
	if params.PoWNoRetargeting {
		return params.PowLimitBits, nil
	}

	// Genesis block: nProofOfWorkLimit.
	if lastNode == nil {
		return BigToCompact(params.PowLimit), nil
	}

	powLimitBits := BigToCompact(params.PowLimit)
	targetSpacing := int64(params.TargetTimePerBlock / time.Second)

	// AllowDigishieldMinDifficultyForBlock: min-difficulty blocks are allowed
	// when the new block is more than 2 * nPowTargetSpacing seconds after its
	// parent (nHeightEffective is 0 on all networks, so no height gate).
	if params.ReduceMinDifficulty &&
		newBlockTime.Unix() > lastNode.Timestamp()+2*targetSpacing {

		return powLimitBits, nil
	}

	nextHeight := int64(lastNode.Height()) + 1
	fNewProtocol := nextHeight >= 69360

	targetTimespan := int64(params.TargetTimespan / time.Second)
	if !fNewProtocol {
		targetTimespan *= 12
	}
	interval := targetTimespan / targetSpacing

	// Only change once per difficulty adjustment interval.
	if nextHeight%interval != 0 {
		if params.ReduceMinDifficulty {
			// Return the last block that did not have the special
			// min-difficulty rule applied. The walk boundary is the static
			// params.DifficultyAdjustmentInterval(), not the (possibly 12x)
			// retarget interval above.
			walkInterval := int64(params.TargetTimespan /
				params.TargetTimePerBlock)
			iterNode := lastNode
			for iterNode.Parent() != nil &&
				int64(iterNode.Height())%walkInterval != 0 &&
				iterNode.Bits() == powLimitBits {

				iterNode = iterNode.Parent()
			}
			return iterNode.Bits(), nil
		}
		return lastNode.Bits(), nil
	}

	// Litecoin/Art Forz: go back the full period unless this is the first
	// retarget after genesis.
	blocksToGoBack := int32(interval - 1)
	if nextHeight != interval {
		blocksToGoBack = int32(interval)
	}

	firstNode := lastNode.RelativeAncestorCtx(blocksToGoBack)
	if firstNode == nil {
		return 0, AssertError("unable to obtain previous retarget block")
	}

	minTimespan := targetTimespan / 4
	maxTimespan := targetTimespan * 4
	if fNewProtocol {
		minTimespan = targetTimespan - targetTimespan/4
		maxTimespan = targetTimespan + targetTimespan/4
	}

	actualTimespan := lastNode.Timestamp() - firstNode.Timestamp()
	switch {
	case nextHeight > 10000:
		if actualTimespan < minTimespan {
			actualTimespan = minTimespan
		} else if actualTimespan > maxTimespan {
			actualTimespan = maxTimespan
		}
	case nextHeight > 5000:
		if actualTimespan < minTimespan/2 {
			actualTimespan = minTimespan / 2
		} else if actualTimespan > maxTimespan {
			actualTimespan = maxTimespan
		}
	default:
		if actualTimespan < minTimespan/4 {
			actualTimespan = minTimespan / 4
		} else if actualTimespan > maxTimespan {
			actualTimespan = maxTimespan
		}
	}

	oldTarget := CompactToBig(lastNode.Bits())
	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(actualTimespan))
	newTarget.Div(newTarget, big.NewInt(targetTimespan))
	if newTarget.Cmp(params.PowLimit) > 0 {
		newTarget.Set(params.PowLimit)
	}

	newTargetBits := BigToCompact(newTarget)
	log.Debugf("Difficulty retarget at block height %d", nextHeight)
	log.Debugf("Old target %08x (%064x)", lastNode.Bits(), oldTarget)
	log.Debugf("New target %08x (%064x)", newTargetBits, CompactToBig(newTargetBits))
	log.Debugf("Actual timespan %d, adjusted timespan %d, target timespan %d",
		lastNode.Timestamp()-firstNode.Timestamp(), actualTimespan,
		targetTimespan)

	return newTargetBits, nil
}

// CalcNextRequiredDifficulty calculates the required difficulty for the block
// after the end of the current best chain based on the difficulty retarget
// rules.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextRequiredDifficulty(timestamp time.Time) (uint32, error) {
	b.chainLock.Lock()
	difficulty, err := calcNextRequiredDifficulty(b.bestChain.Tip(), timestamp, b)
	b.chainLock.Unlock()
	return difficulty, err
}

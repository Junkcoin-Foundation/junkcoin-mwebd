// Copyright (C) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC license that can be found in
// the LICENSE file.

package blockchain

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
	"github.com/ltcmweb/ltcd/wire"
)

// auxPowMergedMiningHeader is the tag merged-mining miners must place
// directly before the chain merkle root in the parent coinbase scriptSig.
// It matches pchMergedMiningHeader in Junkcoin Core (auxpow.h).
var auxPowMergedMiningHeader = []byte{0xfa, 0xbe, 'm', 'm'}

// checkAuxPowProofOfWork verifies a header carrying the AuxPoW flag.  It
// mirrors CheckAuxPowProofOfWork plus CAuxPow::check in Junkcoin Core
// (junkcoin.cpp and auxpow.cpp): the merge-mining proof must commit to
// this header's hash, the parent coinbase must be part of the parent
// block, and the parent chain's proof of work must satisfy this header's
// target.  The child's own hash is deliberately not checked against the
// target; the parent's work stands in for it.
func checkAuxPowProofOfWork(header *wire.BlockHeader,
	target *big.Int) error {

	proof := header.AuxPow
	if proof == nil {
		str := "header has auxpow flag but no auxpow proof attached"
		return ruleError(ErrInvalidAuxPow, str)
	}

	if proof.Index != 0 {
		str := "auxpow is not a generate"
		return ruleError(ErrInvalidAuxPow, str)
	}

	ourChainID := header.Version >> 16
	parentChainID := proof.ParentBlock.Version >> 16
	if parentChainID == ourChainID {
		str := fmt.Sprintf("auxpow parent has our chain ID (%d)",
			parentChainID)
		return ruleError(ErrInvalidAuxPow, str)
	}

	if len(proof.ChainMerkle) > 30 {
		str := fmt.Sprintf("auxpow chain merkle branch too long [len %v]",
			len(proof.ChainMerkle))
		return ruleError(ErrInvalidAuxPow, str)
	}

	if len(proof.CoinbaseTx.TxIn) == 0 {
		str := "auxpow parent coinbase has no inputs"
		return ruleError(ErrInvalidAuxPow, str)
	}

	// The chain merkle root commits to this block's hash and must be
	// embedded in the parent coinbase scriptSig.  The script stores it
	// in reversed (display) byte order.
	rootHash := checkMerkleBranch(header.BlockHash(), proof.ChainMerkle,
		proof.ChainIndex)
	var rootReversed chainhash.Hash
	for i := 0; i < chainhash.HashSize; i++ {
		rootReversed[i] = rootHash[chainhash.HashSize-1-i]
	}

	// The parent coinbase must link into the parent block's merkle tree.
	coinbaseHash := proof.CoinbaseTx.TxHash()
	merkleRoot := checkMerkleBranch(coinbaseHash, proof.MerkleBranch,
		proof.Index)
	if merkleRoot != proof.ParentBlock.MerkleRoot {
		str := fmt.Sprintf("auxpow merkle root incorrect: got %v, want %v",
			merkleRoot, proof.ParentBlock.MerkleRoot)
		return ruleError(ErrInvalidAuxPow, str)
	}

	script := proof.CoinbaseTx.TxIn[0].SignatureScript

	// The chain merkle root must appear in the scriptSig, and when the
	// merged mining header tag is present it must sit directly in front
	// of the root, exactly once.
	pcHead := bytes.Index(script, auxPowMergedMiningHeader)
	pc := bytes.Index(script, rootReversed[:])
	if pc < 0 {
		str := "auxpow missing chain merkle root in parent coinbase"
		return ruleError(ErrInvalidAuxPow, str)
	}
	if pcHead >= 0 {
		if bytes.Index(script[pcHead+1:],
			auxPowMergedMiningHeader) >= 0 {

			str := "multiple merged mining headers in coinbase"
			return ruleError(ErrInvalidAuxPow, str)
		}
		if pcHead+len(auxPowMergedMiningHeader) != pc {
			str := "merged mining header is not just before chain " +
				"merkle root"
			return ruleError(ErrInvalidAuxPow, str)
		}
	} else if pc > 20 {
		str := "auxpow chain merkle root must start in the first 20 " +
			"bytes of the parent coinbase"
		return ruleError(ErrInvalidAuxPow, str)
	}

	// The root is followed by the chain merkle tree size and a nonce
	// fixing this chain's leaf index.
	if len(script) < pc+chainhash.HashSize+8 {
		str := "auxpow missing chain merkle tree size and nonce in " +
			"parent coinbase"
		return ruleError(ErrInvalidAuxPow, str)
	}
	treeSize := binary.LittleEndian.Uint32(script[pc+chainhash.HashSize:])
	nonce := binary.LittleEndian.Uint32(
		script[pc+chainhash.HashSize+4:])
	merkleHeight := uint(len(proof.ChainMerkle))
	if treeSize != uint32(1)<<merkleHeight {
		str := fmt.Sprintf("auxpow merkle branch size %d does not "+
			"match parent coinbase size %d",
			uint32(1)<<merkleHeight, treeSize)
		return ruleError(ErrInvalidAuxPow, str)
	}
	if uint32(proof.ChainIndex) !=
		getAuxPowExpectedIndex(nonce, uint32(ourChainID), merkleHeight) {

		str := fmt.Sprintf("auxpow wrong index: got %d, want %d",
			proof.ChainIndex,
			getAuxPowExpectedIndex(nonce, uint32(ourChainID),
				merkleHeight))
		return ruleError(ErrInvalidAuxPow, str)
	}

	// The parent chain's proof of work must satisfy this header's
	// claimed target.
	parentPow := proof.ParentBlock.PowHash()
	if HashToBig(&parentPow).Cmp(target) > 0 {
		str := fmt.Sprintf("auxpow parent hash of %064x is higher "+
			"than expected max of %064x",
			HashToBig(&parentPow), target)
		return ruleError(ErrHighHash, str)
	}

	return nil
}

// checkMerkleBranch calculates the merkle root for the given hash by
// walking the branch, mirroring CAuxPow::CheckMerkleBranch in Junkcoin
// Core.  An index of -1 yields the zero hash.
func checkMerkleBranch(hash chainhash.Hash, branch []chainhash.Hash,
	index int32) chainhash.Hash {

	if index == -1 {
		return chainhash.Hash{}
	}
	for _, sibling := range branch {
		var buf [2 * chainhash.HashSize]byte
		if index&1 == 1 {
			copy(buf[:chainhash.HashSize], sibling[:])
			copy(buf[chainhash.HashSize:], hash[:])
		} else {
			copy(buf[:chainhash.HashSize], hash[:])
			copy(buf[chainhash.HashSize:], sibling[:])
		}
		hash = chainhash.DoubleHashH(buf[:])
		index >>= 1
	}
	return hash
}

// getAuxPowExpectedIndex picks the deterministic leaf slot for a chain in
// the chain merkle tree, mirroring CAuxPow::getExpectedIndex in Junkcoin
// Core.  All arithmetic is 32-bit like the C++ version.
func getAuxPowExpectedIndex(nonce, chainID uint32, height uint) uint32 {
	rand := nonce
	rand = rand*1103515245 + 12345
	rand += chainID
	rand = rand*1103515245 + 12345
	return rand % (1 << height)
}

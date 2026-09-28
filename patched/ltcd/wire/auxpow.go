// Copyright (C) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC license that can be found in
// the LICENSE file.

package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
)

// VersionAuxPowFlag is the bit in the block header version field that
// signals the header is followed by merge-mining (AuxPoW) data.  It
// matches CBlockHeader::VERSION_AUXPOW (1 << 8) in Junkcoin Core.
const VersionAuxPowFlag = 1 << 8

// maxAuxPowHashes bounds a single AuxPoW merkle branch so a corrupt
// varint cannot make the reader seek an absurd distance before the
// enclosing message size limit is enforced.
const maxAuxPowHashes = 65536

// AuxPowProof holds the merge-mining proof that Junkcoin Core appends to
// a block header whose version carries VersionAuxPowFlag.  The layout
// (see primitives/block.h and auxpow.h in Junkcoin Core) is:
//
//	parent chain coinbase transaction (full transaction encoding)
//	hashBlock          (32 bytes)
//	vMerkleBranch      (varint count followed by that many hashes)
//	nIndex             (int32)
//	vChainMerkleBranch (varint count followed by that many hashes)
//	nChainIndex        (int32)
//	parent block header (80 bytes)
//
// The proof lets a verifier check that the parent chain's proof of work
// satisfies this header's target without grinding this header itself.
type AuxPowProof struct {
	// CoinbaseTx is the parent chain's coinbase transaction committing
	// to this block.
	CoinbaseTx *MsgTx

	// HashBlock is the CMerkleTx hashBlock field, the hash of the block
	// that contains CoinbaseTx.
	HashBlock chainhash.Hash

	// MerkleBranch links CoinbaseTx into its parent block's merkle tree.
	MerkleBranch []chainhash.Hash

	// Index is the position of CoinbaseTx in its parent block.
	Index int32

	// ChainMerkle links the parent block into the chain merkle tree
	// whose leaves commit to this block's hash.
	ChainMerkle []chainhash.Hash

	// ChainIndex is the leaf index of this block in the chain merkle
	// tree.
	ChainIndex int32

	// ParentBlock is the parent chain's block header.
	ParentBlock BlockHeader
}

// readAuxPow reads the merge-mining data appended to a block header whose
// version carries VersionAuxPowFlag and returns it as an AuxPowProof.
func readAuxPow(r io.Reader, pver uint32) (*AuxPowProof, error) {
	// The parent chain's coinbase transaction.
	tx := &MsgTx{}
	if err := tx.BtcDecode(r, pver, WitnessEncoding); err != nil {
		return nil, err
	}

	// hashBlock of the CMerkleTx base.
	var hashBlock chainhash.Hash
	if _, err := io.ReadFull(r, hashBlock[:]); err != nil {
		return nil, err
	}

	// vMerkleBranch of the CMerkleTx base.
	merkleBranch, err := readAuxPowHashes(r, pver)
	if err != nil {
		return nil, err
	}

	// nIndex of the CMerkleTx base.
	index, err := readAuxPowInt32(r)
	if err != nil {
		return nil, err
	}

	// vChainMerkleBranch of the AuxPoW proof.
	chainMerkle, err := readAuxPowHashes(r, pver)
	if err != nil {
		return nil, err
	}

	// nChainIndex of the AuxPoW proof.
	chainIndex, err := readAuxPowInt32(r)
	if err != nil {
		return nil, err
	}

	// The parent chain's block header.
	var parent BlockHeader
	if err := readBlockHeader(r, pver, &parent); err != nil {
		return nil, err
	}

	return &AuxPowProof{
		CoinbaseTx:   tx,
		HashBlock:    hashBlock,
		MerkleBranch: merkleBranch,
		Index:        index,
		ChainMerkle:  chainMerkle,
		ChainIndex:   chainIndex,
		ParentBlock:  parent,
	}, nil
}

// readAuxPowHashes reads a varint hash count followed by that many
// 32-byte hashes.
func readAuxPowHashes(r io.Reader, pver uint32) ([]chainhash.Hash, error) {
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}
	if count > maxAuxPowHashes {
		str := fmt.Sprintf("auxpow merkle branch too large [count %v, max %v]",
			count, maxAuxPowHashes)
		return nil, messageError("readAuxPowHashes", str)
	}
	hashes := make([]chainhash.Hash, count)
	for i := range hashes {
		if _, err := io.ReadFull(r, hashes[i][:]); err != nil {
			return nil, err
		}
	}
	return hashes, nil
}

// readAuxPowInt32 reads a little-endian int32 field of the AuxPoW proof.
func readAuxPowInt32(r io.Reader) (int32, error) {
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b[:])), nil
}

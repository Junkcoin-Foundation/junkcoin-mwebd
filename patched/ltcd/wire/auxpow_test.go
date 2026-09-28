package wire

import (
	"bytes"
	"os"
	"testing"

	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
)

// buildAuxPowBytes assembles the AuxPoW blob appended to a header carrying
// VersionAuxPowFlag, using the same layout as Junkcoin Core.
func buildAuxPowBytes(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	// Parent chain coinbase transaction: version 1, one input, no outputs.
	tx := &MsgTx{
		Version: 1,
		TxIn: []*TxIn{
			{
				PreviousOutPoint: OutPoint{
					Index: 0xffffffff,
				},
				SignatureScript: bytes.Repeat([]byte{0x51}, 41),
				Sequence:        0xffffffff,
			},
		},
		TxOut:    []*TxOut{},
		LockTime: 0,
	}
	if err := tx.BtcEncode(&buf, 70015, WitnessEncoding); err != nil {
		t.Fatalf("failed to encode auxpow tx: %v", err)
	}

	// hashBlock.
	if _, err := buf.Write(make([]byte, chainhash.HashSize)); err != nil {
		t.Fatal(err)
	}

	// vMerkleBranch: empty.
	buf.WriteByte(0)

	// nIndex: -1.
	buf.Write([]byte{0xff, 0xff, 0xff, 0xff})

	// vChainMerkleBranch: empty.
	buf.WriteByte(0)

	// nChainIndex: 0.
	buf.Write([]byte{0, 0, 0, 0})

	// Parent block header.
	var parent BlockHeader
	parent.Version = 0x20000004
	if err := writeBlockHeader(&buf, 70015, &parent); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// TestMsgHeadersDecodeAuxPow verifies that a headers message mixing plain
// and AuxPoW headers decodes with the stream fully consumed and headers
// linking intact.
func TestMsgHeadersDecodeAuxPow(t *testing.T) {
	plain := &BlockHeader{
		Version:    0x20210004,
		MerkleRoot: chainhash.Hash{0x11},
		Bits:       0x1e0fffff,
		Nonce:      7,
	}
	aux := &BlockHeader{
		Version:    0x20210004 | VersionAuxPowFlag,
		MerkleRoot: chainhash.Hash{0x22},
		Bits:       0x1e0fffff,
		Nonce:      9,
	}
	aux.PrevBlock = plain.BlockHash()

	var payload bytes.Buffer
	if err := WriteVarInt(&payload, 70015, 2); err != nil {
		t.Fatal(err)
	}
	for _, bh := range []*BlockHeader{plain, aux} {
		if err := writeBlockHeader(&payload, 70015, bh); err != nil {
			t.Fatal(err)
		}
		if bh.Version&VersionAuxPowFlag != 0 {
			payload.Write(buildAuxPowBytes(t))
		}
		if err := WriteVarInt(&payload, 70015, 0); err != nil {
			t.Fatal(err)
		}
	}

	var msg MsgHeaders
	r := bytes.NewReader(payload.Bytes())
	if err := msg.BtcDecode(r, 70015, LatestEncoding); err != nil {
		t.Fatalf("BtcDecode failed: %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("stream not fully consumed, %d bytes left", r.Len())
	}
	if len(msg.Headers) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(msg.Headers))
	}
	if msg.Headers[1].Version != aux.Version {
		t.Fatalf("auxpow header version = %#x, want %#x",
			msg.Headers[1].Version, aux.Version)
	}
	if msg.Headers[1].PrevBlock != plain.BlockHash() {
		t.Fatal("headers do not link after auxpow decode")
	}
}

// TestMsgHeadersDecodeAuxPowFixture decodes a real 2000-header payload
// captured from junkcoind containing three AuxPoW blocks and verifies the
// stream is consumed exactly, the chain links hold, and the flagged headers
// are recognized.
func TestMsgHeadersDecodeAuxPowFixture(t *testing.T) {
	data, err := os.ReadFile("/tmp/headers_payload.bin")
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}

	var msg MsgHeaders
	r := bytes.NewReader(data)
	if err := msg.BtcDecode(r, 70015, LatestEncoding); err != nil {
		t.Fatalf("BtcDecode failed: %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("stream not fully consumed, %d bytes left", r.Len())
	}
	if len(msg.Headers) != MaxBlockHeadersPerMsg {
		t.Fatalf("expected %d headers, got %d", MaxBlockHeadersPerMsg,
			len(msg.Headers))
	}

	wantPrev := chainhash.Hash{
		0xbd, 0x9a, 0x71, 0x76, 0x2f, 0x9f, 0x9a, 0x41,
		0x5c, 0x83, 0x9e, 0xb8, 0x42, 0xf5, 0x2b, 0x63,
		0xa5, 0xb2, 0xab, 0xa1, 0x28, 0x24, 0xac, 0x99,
		0x01, 0x39, 0xc7, 0x47, 0x7f, 0x82, 0xb1, 0x1d,
	}
	if msg.Headers[0].PrevBlock != wantPrev {
		t.Fatalf("first prev = %v, want %v", msg.Headers[0].PrevBlock,
			wantPrev)
	}

	for i := 1; i < len(msg.Headers); i++ {
		if msg.Headers[i].PrevBlock != msg.Headers[i-1].BlockHash() {
			t.Fatalf("header %d does not link to %d", i, i-1)
		}
	}

	auxIdx := []int{}
	for i, bh := range msg.Headers {
		if bh.Version&VersionAuxPowFlag != 0 {
			if bh.AuxPow == nil {
				t.Fatalf("header %d: auxpow flag set but no proof",
					i)
			}
			if bh.AuxPow.ParentBlock.Version == 0 {
				t.Fatalf("header %d: empty parent header", i)
			}
			auxIdx = append(auxIdx, i)
		} else if bh.AuxPow != nil {
			t.Fatalf("header %d: auxpow proof on unflagged header", i)
		}
	}
	if len(auxIdx) != 3 {
		t.Fatalf("expected 3 auxpow headers, got %v", auxIdx)
	}
	want := []int{740, 741, 798}
	for i := range want {
		if auxIdx[i] != want[i] {
			t.Fatalf("auxpow indices = %v, want %v", auxIdx, want)
		}
	}
}

package blockchain

import (
	"bytes"
	"os"
	"testing"

	"github.com/ltcmweb/ltcd/wire"
)

// TestCheckProofOfWorkHeadersFixture replays a real 2000-header response
// from junkcoind (containing three merge-mined blocks) through the full
// context-less header check.  Every plain header must satisfy its own
// scrypt target and every AuxPoW header must satisfy the merge-mining
// proof verification.
func TestCheckProofOfWorkHeadersFixture(t *testing.T) {
	data, err := os.ReadFile("/tmp/headers_payload.bin")
	if err != nil {
		t.Skipf("fixture not available: %v", err)
	}

	var msg wire.MsgHeaders
	r := bytes.NewReader(data)
	if err := msg.BtcDecode(r, 70015, wire.LatestEncoding); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if r.Len() != 0 {
		t.Fatalf("stream not fully consumed, %d bytes left", r.Len())
	}

	// powLimit only bounds the compact target's range here; any value
	// at or above the headers' own targets keeps that check focused on
	// what this test is about.
	powLimit := CompactToBig(0x1e0fffff)

	auxCount := 0
	for i, header := range msg.Headers {
		isAux := header.Version&wire.VersionAuxPowFlag != 0
		if isAux {
			auxCount++
			if header.AuxPow == nil {
				t.Fatalf("header %d: auxpow flag set but no proof",
					i)
			}
		} else if header.AuxPow != nil {
			t.Fatalf("header %d: auxpow proof on unflagged header", i)
		}
		if err := checkProofOfWork(header, powLimit, BFNone); err != nil {
			t.Fatalf("header %d (auxpow=%v): %v", i, isAux, err)
		}
	}
	if auxCount != 3 {
		t.Fatalf("expected 3 auxpow headers in fixture, got %d", auxCount)
	}
}

// TestCheckProofOfWorkMissingAuxPow ensures a header flagged as merge
// mined but lacking a proof is rejected instead of silently accepted.
func TestCheckProofOfWorkMissingAuxPow(t *testing.T) {
	header := &wire.BlockHeader{
		Version: 0x20210004 | wire.VersionAuxPowFlag,
		Bits:    0x1e0fffff,
	}
	powLimit := CompactToBig(0x1e0fffff)
	err := checkProofOfWork(header, powLimit, BFNone)
	if err == nil {
		t.Fatal("expected error for flagged header without proof")
	}
	ruleErr, ok := err.(RuleError)
	if !ok || ruleErr.ErrorCode != ErrInvalidAuxPow {
		t.Fatalf("expected ErrInvalidAuxPow, got %v", err)
	}
}

package mwebd

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/JunkoinFoundation/junkcoin-mwebd/proto"
	"github.com/JunkoinFoundation/junkcoin-mwebd/sign"
	"github.com/ltcmweb/ltcd/btcec/v2"
	"github.com/ltcmweb/ltcd/chaincfg"
	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
	"github.com/ltcmweb/ltcd/ltcutil"
	"github.com/ltcmweb/ltcd/ltcutil/mweb"
	"github.com/ltcmweb/ltcd/ltcutil/mweb/mw"
	"github.com/ltcmweb/ltcd/txscript"
	"github.com/ltcmweb/ltcd/wire"
)

// Mobile (gomobile) friendly wrappers.
//
// gomobile cannot bind methods that take or return gRPC/context/protobuf types,
// so every RPC used by the Android wallet is re-exposed here using only
// primitives, byte slices and JSON strings.

type mobileUtxo struct {
	Height    int32  `json:"height"`
	Value     uint64 `json:"value"`
	Address   string `json:"address"`
	OutputId  string `json:"outputId"`
	BlockTime uint32 `json:"blockTime"`
}

type mobileStatus struct {
	BlockHeaderHeight int32  `json:"blockHeaderHeight"`
	MwebHeaderHeight  int32  `json:"mwebHeaderHeight"`
	MwebUtxosHeight   int32  `json:"mwebUtxosHeight"`
	BlockTime         uint32 `json:"blockTime"`
}

type mobileCreateResult struct {
	RawTx     string   `json:"rawTx"` // hex
	OutputIds []string `json:"outputIds"`
}

func chainParamsFor(chain string) chaincfg.Params {
	if chain == "testnet" || chain == "test" {
		return junkTestNet4
	}
	return junkMainNet
}

// AddressesFor generates MWEB addresses without a running daemon.
// chain: "mainnet" or "testnet". Returns a JSON array of addresses.
func AddressesFor(chain string, scanSecret, spendPub []byte, from, to int32) (string, error) {
	cp := chainParamsFor(chain)
	resp := sign.Addresses(&sign.AddressesRequest{
		Scan:     scanSecret,
		SpendPub: spendPub,
		From:     uint32(from),
		To:       uint32(to),
	}, &cp)
	b, err := json.Marshal(resp.Address)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// StatusJSON returns the sync status as JSON.
func (s *Server) StatusJSON() (string, error) {
	resp, err := s.Status(context.Background(), &proto.StatusRequest{})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(&mobileStatus{
		BlockHeaderHeight: resp.BlockHeaderHeight,
		MwebHeaderHeight:  resp.MwebHeaderHeight,
		MwebUtxosHeight:   resp.MwebUtxosHeight,
		BlockTime:         resp.BlockTime,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// AddressesJSON generates MWEB addresses from a running daemon as a JSON array.
func (s *Server) AddressesJSON(scanSecret, spendPub []byte, from, to int32) (string, error) {
	resp, err := s.Addresses(context.Background(), &proto.AddressRequest{
		ScanSecret:  scanSecret,
		SpendPubkey: spendPub,
		FromIndex:   uint32(from),
		ToIndex:     uint32(to),
	})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(resp.Address)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// UtxosOnceJSON performs a one-shot (non streaming) scan of the MWEB UTXO set
// for an account and returns the matching UTXOs as a JSON array.
func (s *Server) UtxosOnceJSON(scanSecret []byte, fromHeight int64) (string, error) {
	scan := (*mw.SecretKey)(scanSecret)

	heightMap, err := s.cs.MwebCoinDB.GetLeavesAtHeight()
	if err != nil {
		return "", err
	}
	var heights []uint32
	for height := range heightMap {
		heights = append(heights, height)
	}
	slices.Sort(heights)
	index, _ := slices.BinarySearch(heights, uint32(fromHeight))
	leaf := uint64(0)
	if index > 0 {
		leaf = heightMap[heights[index-1]]
	}

	lfs, err := s.cs.MwebCoinDB.GetLeafset()
	if err != nil {
		return "", err
	}

	var found []*proto.Utxo
	var leaves []uint64
	for ; leaf < lfs.Size; leaf++ {
		if lfs.Contains(leaf) {
			leaves = append(leaves, leaf)
		}
		if len(leaves) == 1000 || leaf == lfs.Size-1 {
			if len(leaves) == 0 {
				continue
			}
			utxos, err := s.cs.MwebCoinDB.FetchLeaves(leaves)
			if err != nil {
				return "", err
			}
			found = append(found, s.filterUtxos(scan, utxos)...)
			leaves = leaves[:0]
		}
	}

	out := make([]mobileUtxo, 0, len(found))
	for _, u := range found {
		out = append(out, mobileUtxo{
			Height:    u.Height,
			Value:     u.Value,
			Address:   u.Address,
			OutputId:  u.OutputId,
			BlockTime: u.BlockTime,
		})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateRaw runs the MWEB transaction builder over a pre-built transaction.
// Returns JSON {"rawTx":"<hex>","outputIds":["<hex>", ...]}.
//
// feeRatePerKb must be int64: gomobile silently drops uint64 parameters.
func (s *Server) CreateRaw(rawTx, scanSecret, spendSecret []byte,
	feeRatePerKb int64, dryRun bool) (string, error) {
	resp, err := s.Create(context.Background(), &proto.CreateRequest{
		RawTx:        rawTx,
		ScanSecret:   scanSecret,
		SpendSecret:  spendSecret,
		FeeRatePerKb: uint64(feeRatePerKb),
		DryRun:       dryRun,
	})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(&mobileCreateResult{
		RawTx:     hex.EncodeToString(resp.RawTx),
		OutputIds: resp.OutputId,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// BroadcastRaw broadcasts a raw transaction, returning its txid.
func (s *Server) BroadcastRaw(rawTx []byte) (string, error) {
	resp, err := s.Broadcast(context.Background(), &proto.BroadcastRequest{
		RawTx: rawTx,
	})
	if err != nil {
		return "", err
	}
	return resp.Txid, nil
}

// SpentOnce takes comma separated output ids and returns (as JSON) those that
// are no longer in the UTXO set, i.e. which have been spent.
func (s *Server) SpentOnce(outputIdsCSV string) (string, error) {
	var ids []string
	for _, id := range strings.Split(outputIdsCSV, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	resp, err := s.Spent(context.Background(), &proto.SpentRequest{OutputId: ids})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(resp.OutputId)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// OutputIdValid reports whether an output id is a 32 byte hex string.
// Output ids are used in raw byte order (see Spent/Create), never reversed.
func OutputIdValid(outputId string) bool {
	b, err := hex.DecodeString(outputId)
	return err == nil && len(b) == chainhash.HashSize
}

// mobilePeginInput describes one transparent input funding a peg-in.
type mobilePeginInput struct {
	OutputId string `json:"outputId"` // outpoint hash, raw byte order (same as BuildRawTxJSON)
	Index    uint32 `json:"index"`
	Value    int64  `json:"value"`
	Address  string `json:"address"` // address the prevout was received on
	PrivKey  string `json:"privKey"` // hex encoded 32 byte private key
}

// mobilePeginRequest is the JSON request for CreatePeginJSON:
//
//	{"chain":"mainnet",
//	 "inputs":[{"outputId":"<hex>","index":0,"value":123,"address":"7..","privKey":"<hex>"}],
//	 "recipient":"jcmweb1...","amount":123,
//	 "changeAddress":"7..","changeValue":45,
//	 "feeRatePerKb":1000,
//	 "scanSecret":"<hex>","spendSecret":"<hex>"}
type mobilePeginRequest struct {
	Chain         string             `json:"chain"`
	Inputs        []mobilePeginInput `json:"inputs"`
	Recipient     string             `json:"recipient"`
	Amount        uint64             `json:"amount"`
	ChangeAddress string             `json:"changeAddress"`
	ChangeValue   uint64             `json:"changeValue"`
	FeeRatePerKb  int64              `json:"feeRatePerKb"`
	ScanSecret    string             `json:"scanSecret"`  // hex, 32 bytes
	SpendSecret   string             `json:"spendSecret"` // hex, 32 bytes
}

type mobilePeginResult struct {
	RawTx string `json:"rawTx"` // hex, fully signed
	Txid  string `json:"txid"`
	Fee   int64  `json:"fee"` // sum(in) - sum(out), litoshis
}

// CreatePeginJSON builds a peg-in transaction (transparent inputs funding an
// MWEB output) in one call:
//
//  1. serialize a skeleton holding the transparent inputs and the MWEB
//     recipient output,
//  2. run the daemon's Create, which builds the MWEB kernel and replaces the
//     outputs with the peg-in output,
//  3. append the transparent change output (Create classifies any transparent
//     skeleton output as a peg-out, so change must be added afterwards), and
//  4. sign the transparent inputs.
//
// reqJSON is a mobilePeginRequest. Returns a mobilePeginResult as JSON.
func (s *Server) CreatePeginJSON(reqJSON string) (string, error) {
	var req mobilePeginRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return "", err
	}
	if len(req.Inputs) == 0 {
		return "", fmt.Errorf("no inputs")
	}
	if req.Amount == 0 {
		return "", fmt.Errorf("amount is zero")
	}
	scanSecret, err := hex.DecodeString(req.ScanSecret)
	if err != nil || len(scanSecret) != 32 {
		return "", fmt.Errorf("bad scan secret")
	}
	spendSecret, err := hex.DecodeString(req.SpendSecret)
	if err != nil || len(spendSecret) != 32 {
		return "", fmt.Errorf("bad spend secret")
	}
	// A zero spend secret would send Create down the (unsupported here)
	// hardware wallet ledger path.
	if bytes.Equal(spendSecret, make([]byte, 32)) {
		return "", fmt.Errorf("spend secret is zero")
	}
	cp := chainParamsFor(req.Chain)

	// Skeleton: transparent inputs + the single MWEB recipient output.
	tx := wire.NewMsgTx(2)
	var sumIn int64
	for _, in := range req.Inputs {
		raw, err := hex.DecodeString(in.OutputId)
		if err != nil || len(raw) != chainhash.HashSize {
			return "", fmt.Errorf("bad output id %q", in.OutputId)
		}
		if in.Value <= 0 {
			return "", fmt.Errorf("bad input value %d", in.Value)
		}
		var h chainhash.Hash
		copy(h[:], raw)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: h, Index: in.Index},
			Sequence:         wire.MaxTxInSequenceNum,
		})
		sumIn += in.Value
	}

	recipientAddr, err := ltcutil.DecodeAddress(req.Recipient, &cp)
	if err != nil {
		return "", fmt.Errorf("bad recipient %q: %w", req.Recipient, err)
	}
	recipientScript, err := txscript.PayToAddrScript(recipientAddr)
	if err != nil {
		return "", err
	}
	if !txscript.IsMweb(recipientScript) {
		return "", fmt.Errorf("recipient %q is not an MWEB address", req.Recipient)
	}
	// The skeleton output doubles as the recipient declaration: Create reads
	// its value as the MWEB recipient amount and its script as the stealth
	// address.
	skeletonOut := &wire.TxOut{Value: int64(req.Amount), PkScript: recipientScript}
	tx.AddTxOut(skeletonOut)

	var skeleton bytes.Buffer
	if err := tx.Serialize(&skeleton); err != nil {
		return "", err
	}

	resp, err := s.Create(context.Background(), &proto.CreateRequest{
		RawTx:        skeleton.Bytes(),
		ScanSecret:   scanSecret,
		SpendSecret:  spendSecret,
		FeeRatePerKb: uint64(req.FeeRatePerKb),
	})
	if err != nil {
		return "", err
	}
	if len(resp.RawTx) == 0 {
		return "", fmt.Errorf("daemon returned an empty transaction")
	}

	var ptx wire.MsgTx
	if err := ptx.Deserialize(bytes.NewReader(resp.RawTx)); err != nil {
		return "", err
	}

	// Create replaced the skeleton output with the canonical pegin output:
	// value = amount + kernel fee, script = OP_9 || kernel ID (the same
	// fixup litecoin's Transact::AddMWEBTx performs, "Update pegin
	// output").  The block level HogEx accounting requires the output value
	// to equal the kernel's pegin field, so assert both here.
	if len(ptx.TxOut) != 1 {
		return "", fmt.Errorf("unexpected output count after Create: %d", len(ptx.TxOut))
	}
	if ptx.Mweb == nil || len(ptx.Mweb.TxBody.Kernels) != 1 {
		return "", fmt.Errorf("missing MWEB kernel after Create")
	}
	kernel := ptx.Mweb.TxBody.Kernels[0]
	mwebFee := uint64(mweb.EstimateFee(
		[]*wire.TxOut{skeletonOut}, ltcutil.Amount(req.FeeRatePerKb), false))
	if kernel.Pegin != req.Amount+mwebFee || kernel.Fee != mwebFee {
		return "", fmt.Errorf("kernel mismatch: pegin=%d fee=%d, want pegin=%d fee=%d",
			kernel.Pegin, kernel.Fee, req.Amount+mwebFee, mwebFee)
	}
	if ptx.TxOut[0].Value != int64(kernel.Pegin) {
		return "", fmt.Errorf("pegin output value %d != kernel pegin %d",
			ptx.TxOut[0].Value, kernel.Pegin)
	}
	if want := mweb.NewPegin(kernel.Pegin, kernel.Hash()).PkScript;
		!bytes.Equal(ptx.TxOut[0].PkScript, want) {
		return "", fmt.Errorf("pegin output does not commit to the kernel")
	}

	// Create strips every transparent skeleton output (pegin is re-added
	// above, peg-outs live in the kernel), so transparent change must be
	// appended here, after Create.
	if req.ChangeValue > 0 {
		if req.ChangeAddress == "" {
			return "", fmt.Errorf("change value set without a change address")
		}
		changeAddr, err := ltcutil.DecodeAddress(req.ChangeAddress, &cp)
		if err != nil {
			return "", fmt.Errorf("bad change address %q: %w", req.ChangeAddress, err)
		}
		changeScript, err := txscript.PayToAddrScript(changeAddr)
		if err != nil {
			return "", err
		}
		if txscript.IsMweb(changeScript) {
			return "", fmt.Errorf("change address must be transparent")
		}
		ptx.AddTxOut(&wire.TxOut{Value: int64(req.ChangeValue), PkScript: changeScript})
	}

	var sumOut int64
	for _, out := range ptx.TxOut {
		sumOut += out.Value
	}
	ltcFee := sumIn - sumOut
	if ltcFee < 0 {
		return "", fmt.Errorf("inputs (%d) do not cover outputs (%d)", sumIn, sumOut)
	}
	// Total fee paid by the sender: the transparent leftover plus the MWEB
	// kernel fee (which entered through the pegin output value).
	fee := ltcFee + int64(mwebFee)

	if err := signPeginInputs(&ptx, req.Inputs, &cp); err != nil {
		return "", err
	}

	var raw bytes.Buffer
	if err := ptx.Serialize(&raw); err != nil {
		return "", err
	}
	b, err := json.Marshal(&mobilePeginResult{
		RawTx: hex.EncodeToString(raw.Bytes()),
		Txid:  ptx.TxHash().String(),
		Fee:   fee,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// signPeginInputs signs every transparent input of tx with the matching
// private key, using the script type of the previous output:
//
//	P2PKH          -> scriptSig (legacy sighash, mweb excluded)
//	P2WPKH         -> witness (BIP143)
//	P2SH-P2WPKH    -> scriptSig pushing the witness program + witness
//	P2TR (BIP 86)  -> witness (BIP342)
//
// The previous outpoints must match req.Inputs order. The mweb body is not
// committed to by any of these sighashes (C++ SignatureHash serializes with
// SERIALIZE_TRANSACTION_NO_WITNESS | SERIALIZE_NO_MWEB).
func signPeginInputs(tx *wire.MsgTx, ins []mobilePeginInput, cp *chaincfg.Params) error {
	if len(ins) != len(tx.TxIn) {
		return fmt.Errorf("input count mismatch: %d vs %d", len(ins), len(tx.TxIn))
	}

	fetcher := txscript.NewMultiPrevOutFetcher(nil)
	scripts := make([][]byte, len(ins))
	for i, in := range ins {
		addr, err := ltcutil.DecodeAddress(in.Address, cp)
		if err != nil {
			return fmt.Errorf("input %d: bad address %q: %w", i, in.Address, err)
		}
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return fmt.Errorf("input %d: %w", i, err)
		}
		if txscript.IsMweb(pkScript) {
			return fmt.Errorf("input %d: MWEB addresses cannot fund a peg-in", i)
		}
		scripts[i] = pkScript
		fetcher.AddPrevOut(tx.TxIn[i].PreviousOutPoint,
			&wire.TxOut{Value: in.Value, PkScript: pkScript})
	}
	sigHashes := txscript.NewTxSigHashes(tx, fetcher)

	for i, in := range ins {
		privBytes, err := hex.DecodeString(in.PrivKey)
		if err != nil || len(privBytes) != 32 {
			return fmt.Errorf("input %d: bad private key", i)
		}
		key, pub := btcec.PrivKeyFromBytes(privBytes)

		switch txscript.GetScriptClass(scripts[i]) {
		case txscript.PubKeyHashTy:
			scriptSig, err := txscript.SignatureScript(tx, i, scripts[i],
				txscript.SigHashAll, key, true)
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			tx.TxIn[i].SignatureScript = scriptSig

		case txscript.WitnessV0PubKeyHashTy:
			sig, err := txscript.RawTxInWitnessSignature(tx, sigHashes, i,
				in.Value, scripts[i], txscript.SigHashAll, key)
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			tx.TxIn[i].Witness = wire.TxWitness{sig, pub.SerializeCompressed()}

		case txscript.ScriptHashTy:
			// P2SH-P2WPKH: redeem script is the witness program
			// 0x0014||keyhash, pushed by the scriptSig; the witness
			// signs over the program itself.  The P2SH script commits
			// to hash160(witnessScript), not to the key hash.
			pubKeyHash := ltcutil.Hash160(pub.SerializeCompressed())
			witnessScript := make([]byte, 0, 22)
			witnessScript = append(witnessScript, txscript.OP_0, byte(len(pubKeyHash)))
			witnessScript = append(witnessScript, pubKeyHash...)

			// OP_HASH160 <20-byte hash> OP_EQUAL = 23 bytes.
			if len(scripts[i]) != 23 ||
				scripts[i][0] != txscript.OP_HASH160 ||
				scripts[i][1] != 20 ||
				scripts[i][22] != txscript.OP_EQUAL ||
				!bytes.Equal(scripts[i][2:22], ltcutil.Hash160(witnessScript)) {
				return fmt.Errorf("input %d: unsupported P2SH script: script=%x hash160(ws)=%x",
					i, scripts[i], ltcutil.Hash160(witnessScript))
			}

			sig, err := txscript.RawTxInWitnessSignature(tx, sigHashes, i,
				in.Value, witnessScript, txscript.SigHashAll, key)
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			scriptSig, err := txscript.NewScriptBuilder().
				AddData(witnessScript).Script()
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			tx.TxIn[i].SignatureScript = scriptSig
			tx.TxIn[i].Witness = wire.TxWitness{sig, pub.SerializeCompressed()}

		case txscript.WitnessV1TaprootTy:
			// BIP 86 key-path spend: empty taproot tree, as generated
			// by the wallet's taprootTweakPrivateKey().
			witness, err := txscript.TaprootWitnessSignature(tx, sigHashes, i,
				in.Value, scripts[i], txscript.SigHashDefault, key)
			if err != nil {
				return fmt.Errorf("input %d: %w", i, err)
			}
			tx.TxIn[i].Witness = witness

		default:
			return fmt.Errorf("input %d: unsupported script type %s",
				i, txscript.GetScriptClass(scripts[i]).String())
		}
	}
	return nil
}

type mobileTxIn struct {
	OutputId string `json:"outputId"`
	Index    uint32 `json:"index"`
}

type mobileTxOut struct {
	Address string `json:"address"`
	Value   uint64 `json:"value"`
}

// mobileTxRequest is the JSON request shared by BuildRawTxJSON and
// EstimateFeeJSON:
//
//	{"chain":"mainnet",
//	 "inputs":[{"outputId":"<hex>","index":0}, ...],
//	 "recipients":[{"address":"<mweb|transparent addr>","value":123}, ...]}
type mobileTxRequest struct {
	Chain      string        `json:"chain"`
	Inputs     []mobileTxIn  `json:"inputs"`
	Recipients []mobileTxOut `json:"recipients"`
}

func (r *mobileTxRequest) txOuts() ([]*wire.TxOut, error) {
	cp := chainParamsFor(r.Chain)
	txOuts := make([]*wire.TxOut, 0, len(r.Recipients))
	for _, out := range r.Recipients {
		addr, err := ltcutil.DecodeAddress(out.Address, &cp)
		if err != nil {
			return nil, fmt.Errorf("bad address %q: %w", out.Address, err)
		}
		pkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, err
		}
		txOuts = append(txOuts, &wire.TxOut{
			Value:    int64(out.Value),
			PkScript: pkScript,
		})
	}
	return txOuts, nil
}

// BuildRawTxJSON builds the serialized unsigned transaction skeleton that
// CreateRaw expects: inputs reference MWEB coins by output id plus the index
// of the address they were received on; recipients may be MWEB or transparent
// (pegout) addresses. Returns the raw tx bytes.
func BuildRawTxJSON(reqJSON string) ([]byte, error) {
	var req mobileTxRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return nil, err
	}

	tx := wire.NewMsgTx(2)
	for _, in := range req.Inputs {
		raw, err := hex.DecodeString(in.OutputId)
		if err != nil || len(raw) != chainhash.HashSize {
			return nil, fmt.Errorf("bad output id %q", in.OutputId)
		}
		var h chainhash.Hash
		copy(h[:], raw)
		tx.AddTxIn(&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{Hash: h, Index: in.Index},
			Sequence:         wire.MaxTxInSequenceNum,
		})
	}

	txOuts, err := req.txOuts()
	if err != nil {
		return nil, err
	}
	for _, txOut := range txOuts {
		tx.AddTxOut(txOut)
	}

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EstimateFeeJSON estimates the fee (litoshis) the daemon will charge for the
// given outputs. Pass the same recipients JSON you pass to BuildRawTxJSON,
// including the change output. Returns {"fee":<n>}.
func EstimateFeeJSON(reqJSON string, feeRatePerKb int64) (string, error) {
	var req mobileTxRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return "", err
	}
	txOuts, err := req.txOuts()
	if err != nil {
		return "", err
	}
	fee := mweb.EstimateFee(txOuts, ltcutil.Amount(feeRatePerKb), false)
	b, err := json.Marshal(map[string]uint64{"fee": fee})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

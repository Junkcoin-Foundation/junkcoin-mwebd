package mwebd

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/JunkoinFoundation/junkcoin-mwebd/ledger"
	"github.com/JunkoinFoundation/junkcoin-mwebd/proto"
	"github.com/JunkoinFoundation/junkcoin-mwebd/sign"
	"github.com/btcsuite/btclog"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/ltcmweb/ltcd/chaincfg"
	"github.com/ltcmweb/ltcd/chaincfg/chainhash"
	"github.com/ltcmweb/ltcd/ltcutil"
	"github.com/ltcmweb/ltcd/ltcutil/mweb"
	"github.com/ltcmweb/ltcd/ltcutil/mweb/mw"
	"github.com/ltcmweb/ltcd/txscript"
	"github.com/ltcmweb/ltcd/wire"
	"github.com/ltcmweb/neutrino"
	"github.com/ltcmweb/neutrino/mwebdb"
	"github.com/ltcsuite/ltcwallet/walletdb"
	_ "github.com/ltcsuite/ltcwallet/walletdb/bdb"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Server struct {
	proto.UnimplementedRpcServer
	db        walletdb.DB
	cs        *neutrino.ChainService
	cp        chaincfg.Params
	mtx       sync.Mutex
	server    *grpc.Server
	utxoChan  map[mw.SecretKey]map[*utxoStreamer]struct{}
	coinCache *lru.Cache[mw.SecretKey, *lru.Cache[chainhash.Hash, *mweb.Coin]]
	ledgerTx  *ledger.TxContext
}

type ServerArgs struct {
	Chain, DataDir, PeerAddr, ProxyAddr string
}

func NewBareServer(chainParams chaincfg.Params) *Server {
	return &Server{cp: chainParams}
}

func NewServer(chain, dataDir, peer string) (*Server, error) {
	return NewServer2(&ServerArgs{
		Chain: chain, DataDir: dataDir, PeerAddr: peer,
	})
}

func NewServer2(args *ServerArgs) (s *Server, err error) {
	s = &Server{server: grpc.NewServer()}
	proto.RegisterRpcServer(s.server, s)

	s.utxoChan = map[mw.SecretKey]map[*utxoStreamer]struct{}{}
	s.coinCache, _ = lru.New[mw.SecretKey, *lru.Cache[chainhash.Hash, *mweb.Coin]](10)

	s.db, err = walletdb.Create(
		"bdb", filepath.Join(args.DataDir, "neutrino.db"), false, time.Minute)
	if err != nil {
		return
	}

	cfg := neutrino.Config{
		DataDir:     args.DataDir,
		Database:    s.db,
		ChainParams: GetJunkMainNetParams(),
	}

	switch args.Chain {
	case "testnet":
		cfg.ChainParams = GetJunkTestNet4Params()
	case "regtest":
		cfg.ChainParams = chaincfg.RegressionNetParams
	default:
		cfg.ChainParams = GetJunkMainNetParams()
	}

	if args.PeerAddr != "" {
		cfg.AddPeers = []string{args.PeerAddr}
	}

	if args.ProxyAddr != "" {
		url, err := url.Parse(args.ProxyAddr)
		if err != nil {
			return nil, err
		}
		dialer, err := proxy.FromURL(url, proxy.Direct)
		if err != nil {
			return nil, err
		}
		cfg.Dialer = func(addr net.Addr) (net.Conn, error) {
			return dialer.Dial(addr.Network(), addr.String())
		}
	}

	log := btclog.NewBackend(&lumberjack.Logger{
		Filename:   filepath.Join(args.DataDir, "logs", "debug.log"),
		MaxSize:    10,
		MaxBackups: 10,
		Compress:   true,
	}).Logger("")
	log.SetLevel(btclog.LevelDebug)
	neutrino.UseLogger(log)

	// Junkcoin Core (net_processing) disconnects inbound peers whose
	// subver is neither "JKC:3.1.1" nor "JunkcoinCore:4.*" (it is meant
	// to reject Litecoin peers). The default neutrino user agent
	// (/ltcwire:x/neutrino:y/) got our header/broadcast connections
	// closed right after the version message ("no peers replied to inv
	// message"). Identify as JunkcoinCore so the testnet/mainnet nodes
	// accept the light client; match the running node's major version.
	neutrino.UserAgentName = "JunkcoinCore:4.0.3.1"

	s.cs, err = neutrino.NewChainService(cfg)
	if err != nil {
		return
	}
	s.cp = s.cs.ChainParams()

	s.cs.RegisterMwebUtxosCallback(s.utxoHandler)
	return s, s.cs.Start()
}

func (s *Server) Start(port int) (int, error) {
	return s.StartAddr(fmt.Sprintf("127.0.0.1:%d", port))
}

func (s *Server) StartAddr(addr string) (int, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}
	if addr[len(addr)-2:] == ":0" {
		go s.serve(lis)
		return lis.Addr().(*net.TCPAddr).Port, nil
	}
	return 0, s.serve(lis)
}

func (s *Server) StartUnix(path string) error {
	os.Remove(path)
	lis, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	go s.serve(lis)
	return nil
}

func (s *Server) serve(lis net.Listener) error {
	if err := s.server.Serve(lis); err != nil {
		return err
	}
	if err := s.cs.Stop(); err != nil {
		return err
	}
	return s.db.Close()
}

func (s *Server) Stop() {
	s.server.Stop()
}

func (s *Server) Status(context.Context,
	*proto.StatusRequest) (*proto.StatusResponse, error) {

	bh, bhHeight, err := s.cs.BlockHeaders.ChainTip()
	if err != nil {
		return nil, err
	}

	heightMap, err := s.cs.MwebCoinDB.GetLeavesAtHeight()
	if err != nil {
		return nil, err
	}

	var mhHeight uint32
	for height := range heightMap {
		if height > mhHeight {
			mhHeight = height
		}
	}

	lfs, err := s.cs.MwebCoinDB.GetLeafset()
	if err != nil {
		return nil, err
	}

	return &proto.StatusResponse{
		BlockHeaderHeight: int32(bhHeight),
		MwebHeaderHeight:  int32(mhHeight),
		MwebUtxosHeight:   int32(lfs.Height),
		BlockTime:         uint32(bh.Timestamp.Unix()),
	}, nil
}

func (s *Server) recordMempoolTx(tx *wire.MsgTx) {
	if tx.Mweb == nil {
		return
	}
	_ = walletdb.Update(s.db, func(wtx walletdb.ReadWriteTx) error {
		mempoolBucket, err := wtx.CreateTopLevelBucket([]byte("mweb-mempool"))
		if err != nil {
			return err
		}
		spentBucket, err := wtx.CreateTopLevelBucket([]byte("mweb-spent-mempool"))
		if err != nil {
			return err
		}
		for _, output := range tx.Mweb.TxBody.Outputs {
			var buf bytes.Buffer
			if err := output.Serialize(&buf); err == nil {
				outputId := output.Hash()
				_ = mempoolBucket.Put(outputId[:], buf.Bytes())
			}
		}
		for _, in := range tx.Mweb.TxBody.Inputs {
			_ = spentBucket.Put(in.OutputId[:], []byte{1})
			_ = mempoolBucket.Delete(in.OutputId[:])
		}
		return nil
	})
}

func (s *Server) utxoHandler(lfs *mweb.Leafset, utxos []*wire.MwebNetUtxo) {
	walletdb.Update(s.db, func(tx walletdb.ReadWriteTx) error {
		bucket, err := tx.CreateTopLevelBucket([]byte("mweb-mempool"))
		if err != nil {
			return err
		}
		spentBucket, _ := tx.CreateTopLevelBucket([]byte("mweb-spent-mempool"))

		for _, utxo := range utxos {
			if utxo.Height == 0 {
				var buf bytes.Buffer
				if err = utxo.Output.Serialize(&buf); err != nil {
					return err
				}
				if err = bucket.Put(utxo.OutputId[:], buf.Bytes()); err != nil {
					return err
				}
			} else {
				if err = bucket.Delete(utxo.OutputId[:]); err != nil {
					return err
				}
				if spentBucket != nil {
					_ = spentBucket.Delete(utxo.OutputId[:])
				}
			}
		}
		if lfs != nil && spentBucket != nil {
			_ = tx.DeleteTopLevelBucket([]byte("mweb-spent-mempool"))
		}
		return nil
	})

	var leaves []uint64
	for _, utxo := range utxos {
		if utxo.Height > 0 {
			leaves = append(leaves, utxo.LeafIndex)
		}
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for scanSecret, us := range s.utxoChan {
		utxos := s.filterUtxos(&scanSecret, utxos)
		for u := range us {
			u.notify(lfs, utxos, leaves)
		}
	}
}

func (s *Server) filterUtxos(scanSecret *mw.SecretKey,
	utxos []*wire.MwebNetUtxo) (result []*proto.Utxo) {

	for _, utxo := range utxos {
		coin, err := s.rewindOutput(utxo.Output, scanSecret)
		if err != nil {
			continue
		}
		addr := ltcutil.NewAddressMweb(coin.Address, &s.cp)
		bh, err := s.cs.BlockHeaders.FetchHeaderByHeight(uint32(utxo.Height))
		if err != nil {
			bh = &wire.BlockHeader{Timestamp: time.Unix(0, 0)}
		}
		result = append(result, &proto.Utxo{
			Height:    utxo.Height,
			Value:     coin.Value,
			Address:   addr.String(),
			OutputId:  hex.EncodeToString(utxo.OutputId[:]),
			BlockTime: uint32(bh.Timestamp.Unix()),
		})
	}
	return
}

func (s *Server) Utxos(req *proto.UtxosRequest,
	stream proto.Rpc_UtxosServer) (err error) {

	scanSecret := (*mw.SecretKey)(req.ScanSecret)
	u := s.newUtxoStreamer(scanSecret)
	s.mtx.Lock()
	if s.utxoChan[*scanSecret] == nil {
		s.utxoChan[*scanSecret] = map[*utxoStreamer]struct{}{}
	}
	s.utxoChan[*scanSecret][u] = struct{}{}
	s.mtx.Unlock()

	defer func() {
		close(u.quit)
		s.mtx.Lock()
		delete(s.utxoChan[*scanSecret], u)
		if len(s.utxoChan[*scanSecret]) == 0 {
			delete(s.utxoChan, *scanSecret)
		}
		s.mtx.Unlock()
	}()

	heightMap, err := s.cs.MwebCoinDB.GetLeavesAtHeight()
	if err != nil {
		return
	}
	var heights []uint32
	for height := range heightMap {
		heights = append(heights, height)
	}
	slices.Sort(heights)
	index, _ := slices.BinarySearch(heights, uint32(req.FromHeight))
	leaf := uint64(0)
	if index > 0 {
		leaf = heightMap[heights[index-1]]
	}

	u.lfs, err = s.cs.MwebCoinDB.GetLeafset()
	if err != nil {
		return
	}
	for leaves := []uint64{}; leaf < u.lfs.Size; leaf++ {
		if u.lfs.Contains(leaf) {
			leaves = append(leaves, leaf)
		}
		if len(leaves) == 1000 || leaf == u.lfs.Size-1 {
			utxos, err := s.cs.MwebCoinDB.FetchLeaves(leaves)
			if err != nil {
				return err
			}
			for _, utxo := range s.filterUtxos(scanSecret, utxos) {
				if err = stream.Send(utxo); err != nil {
					return err
				}
			}
			leaves = leaves[:0]
		}
	}
	for ; err == nil; err = stream.Send(<-u.ch) {
	}
	return
}

func (s *Server) Addresses(ctx context.Context,
	req *proto.AddressRequest) (*proto.AddressResponse, error) {

	resp := sign.Addresses(&sign.AddressesRequest{
		Scan:     req.ScanSecret,
		SpendPub: req.SpendPubkey,
		From:     req.FromIndex,
		To:       req.ToIndex,
	}, &s.cp)
	return &proto.AddressResponse{Address: resp.Address}, nil
}

func Addresses(scanSecret, spendPubKey []byte, i, j int32) string {
	resp := sign.Addresses(&sign.AddressesRequest{
		Scan:     scanSecret,
		SpendPub: spendPubKey,
		From:     uint32(i),
		To:       uint32(j),
	}, &junkMainNet)
	return strings.Join(resp.Address, ",")
}

func (s *Server) Spent(ctx context.Context,
	req *proto.SpentRequest) (*proto.SpentResponse, error) {

	resp := &proto.SpentResponse{}
	for _, outputIdStr := range req.OutputId {
		outputId, err := hex.DecodeString(outputIdStr)
		if err != nil {
			return nil, err
		}
		if !s.cs.MwebUtxoExists((*chainhash.Hash)(outputId)) {
			resp.OutputId = append(resp.OutputId, outputIdStr)
		}
	}
	return resp, nil
}

func (s *Server) fetchCoin(outputId chainhash.Hash) (*wire.MwebOutput, error) {
	if s.cs == nil {
		// Bare server (no chain state): treat every coin as unknown so the
		// caller classifies the input as transparent (peg-in funding).
		return nil, mwebdb.ErrCoinNotFound
	}
	output, err := s.cs.MwebCoinDB.FetchCoin(&outputId)
	if err == mwebdb.ErrCoinNotFound {
		slices.Reverse(outputId[:])
		output, err = s.cs.MwebCoinDB.FetchCoin(&outputId)
	}
	if err == mwebdb.ErrCoinNotFound {
		err = walletdb.View(s.db, func(tx walletdb.ReadTx) error {
			bucket := tx.ReadBucket([]byte("mweb-mempool"))
			if bucket == nil {
				return err
			}
			b := bucket.Get(outputId[:])
			if b == nil {
				slices.Reverse(outputId[:])
				b = bucket.Get(outputId[:])
			}
			if b == nil {
				return err
			}
			output = &wire.MwebOutput{}
			return output.Deserialize(bytes.NewReader(b))
		})
	}
	return output, err
}

func (s *Server) rewindOutput(output *wire.MwebOutput,
	scanSecret *mw.SecretKey) (coin *mweb.Coin, err error) {

	cache, ok := s.coinCache.Get(*scanSecret)
	if !ok {
		cache, _ = lru.New[chainhash.Hash, *mweb.Coin](100)
		s.coinCache.Add(*scanSecret, cache)
	}
	coin, ok = cache.Get(*output.Hash())
	if !ok {
		coin, err = mweb.RewindOutput(output, scanSecret)
		if err == nil {
			cache.Add(*output.Hash(), coin)
		}
	}
	if coin != nil {
		c := mweb.Coin(*coin)
		coin = &c
	}
	return
}

func (s *Server) Create(ctx context.Context,
	req *proto.CreateRequest) (*proto.CreateResponse, error) {

	var (
		tx         wire.MsgTx
		txIns      []*wire.TxIn
		pegouts    []*wire.TxOut
		coins      []*mweb.Coin
		addrIndex  []uint32
		recipients []*mweb.Recipient
		pegin      uint64
		sumCoins   uint64
		sumOutputs uint64
	)

	err := tx.Deserialize(bytes.NewReader(req.RawTx))
	if err != nil {
		return nil, err
	}

	keychain := &mweb.Keychain{
		Scan:  (*mw.SecretKey)(req.ScanSecret),
		Spend: (*mw.SecretKey)(req.SpendSecret),
	}

	for _, txIn := range tx.TxIn {
		output, err := s.fetchCoin(txIn.PreviousOutPoint.Hash)
		switch err {
		case nil:
			coin, err := s.rewindOutput(output, keychain.Scan)
			if err != nil {
				return nil, err
			}

			index := txIn.PreviousOutPoint.Index
			coin.CalculateOutputKey(keychain.SpendKey(index))
			coins = append(coins, coin)
			addrIndex = append(addrIndex, index)
			sumCoins += coin.Value

		case mwebdb.ErrCoinNotFound:
			txIns = append(txIns, txIn)

		default:
			return nil, err
		}
	}

	for _, txOut := range tx.TxOut {
		sumOutputs += uint64(txOut.Value)
		if !txscript.IsMweb(txOut.PkScript) {
			pegouts = append(pegouts, txOut)
			continue
		}

		_, addrs, _, err := txscript.ExtractPkScriptAddrs(txOut.PkScript, &s.cp)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, &mweb.Recipient{
			Value:   uint64(txOut.Value),
			Address: addrs[0].(*ltcutil.AddressMweb).StealthAddress(),
		})
	}

	if len(coins) == 0 && len(recipients) == 0 {
		return &proto.CreateResponse{RawTx: req.RawTx}, nil
	}

	fee := mweb.EstimateFee(tx.TxOut, ltcutil.Amount(req.FeeRatePerKb), false)
	if sumOutputs+fee > sumCoins {
		pegin = sumOutputs + fee - sumCoins
	} else {
		fee = sumCoins - sumOutputs
	}

	if !req.DryRun {
		if *keychain.Spend == (mw.SecretKey{}) {
			if s.ledgerTx == nil || s.ledgerTx.Tx == nil {
				s.ledgerTx = &ledger.TxContext{
					Coins:      coins,
					AddrIndex:  addrIndex,
					Recipients: recipients,
					Fee:        fee,
					Pegin:      pegin,
					Pegouts:    pegouts,
				}
				return &proto.CreateResponse{}, nil
			}
			tx.Mweb = s.ledgerTx.Tx
			coins = s.ledgerTx.NewCoins
			s.ledgerTx = nil
		} else {
			tx.Mweb, coins, err = mweb.NewTransaction(
				coins, recipients, fee, pegin, pegouts, nil, nil)
		}
		if err != nil {
			return nil, err
		}
	} else {
		tx.Mweb = &wire.MwebTx{
			TxBody: &wire.MwebTxBody{
				Kernels: []*wire.MwebKernel{{}},
			},
		}
	}

	tx.TxIn = txIns
	if pegin > 0 {
		tx.TxOut = pegouts
		tx.AddTxOut(mweb.NewPegin(pegin, tx.Mweb.TxBody.Kernels[0].Hash()))
	} else {
		// MWEB-only transactions (peg-outs and mweb-to-mweb sends) must not
		// carry transparent outputs: the node expects vin=0/vout=0, and the
		// peg-out recipients are taken from the kernel's PegOutCoins.
		tx.TxOut = nil
	}

	var buf bytes.Buffer
	if err = tx.Serialize(&buf); err != nil {
		return nil, err
	}

	resp := &proto.CreateResponse{RawTx: buf.Bytes()}
	for _, coin := range coins {
		resp.OutputId = append(resp.OutputId, hex.EncodeToString(coin.OutputId[:]))
	}

	return resp, nil
}

func (s *Server) LedgerExchange(ctx context.Context,
	req *proto.LedgerApdu) (*proto.LedgerApdu, error) {

	if s.ledgerTx == nil {
		return nil, errors.New("nil ledger tx")
	}
	if err := s.ledgerTx.Process(req.Data); err != nil {
		return nil, err
	}
	if s.ledgerTx.Tx != nil {
		return &proto.LedgerApdu{}, nil
	}
	return &proto.LedgerApdu{Data: s.ledgerTx.Request()}, nil
}

func (s *Server) Broadcast(ctx context.Context,
	req *proto.BroadcastRequest) (*proto.BroadcastResponse, error) {

	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(req.RawTx)); err != nil {
		return nil, err
	}

	if err := s.cs.SendTransaction(&tx); err != nil {
		return nil, err
	}

	// Record the transaction in the local mempool cache only after the node
	// accepted it for relay. Recording before the send left the outputs of a
	// failed broadcast in `mweb-mempool` forever: the wallet then saw a
	// phantom balance and coin selection could pick outputs that never
	// existed on chain, making every subsequent spend fail.
	if tx.Mweb != nil {
		s.recordMempoolTx(&tx)
		var utxos []*wire.MwebNetUtxo
		for _, output := range tx.Mweb.TxBody.Outputs {
			utxos = append(utxos, &wire.MwebNetUtxo{
				Output:   output,
				OutputId: output.Hash(),
			})
		}
		go s.utxoHandler(nil, utxos)
	}

	return &proto.BroadcastResponse{Txid: tx.TxHash().String()}, nil
}

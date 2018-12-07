package btc

import (
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/conf"
)

const Symbol = "btc"

type Extractor struct {
	logger      *zap.SugaredLogger
	peer        *peer.Peer
	chainParams *chaincfg.Params
	bs          blocc.BlockStore
	ts          blocc.TxStore
	ms          blocc.MetricStore
	mb          blocc.TxMsgBus

	throttleBlocks chan struct{}
	throttleTxns   chan struct{}

	txLifetime time.Duration

	sync.WaitGroup
}

func Extract(bs blocc.BlockStore, ts blocc.TxStore, ms blocc.MetricStore, mb blocc.TxMsgBus) (*Extractor, error) {

	e := &Extractor{
		logger: zap.S().With("package", "blocc.btc"),
		bs:     bs,
		ts:     ts,
		ms:     ms,
		mb:     mb,

		throttleBlocks: make(chan struct{}, config.GetInt("extractor.throttle_blocks")),
		throttleTxns:   make(chan struct{}, config.GetInt("extractor.throttle_transactions")),

		txLifetime: config.GetDuration("extractor.transaction_lifetime"),
	}

	var err error

	// Initialize the BlockStore for BTC
	if bs != nil {
		err = e.bs.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init BlockStore: %s", err)
		}
	}

	// Initialize the TxStore for BTC
	if ts != nil {
		err = e.ts.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init TxStore: %s", err)
		}
	}

	// Initialize the MetricStire for BTC
	if ms != nil {
		err = e.ms.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init MetricStore: %s", err)
		}
	}

	// Create an array of chains such that we can pick the one we want
	chains := []*chaincfg.Params{
		&chaincfg.MainNetParams,
		&chaincfg.RegressionNetParams,
		&chaincfg.SimNetParams,
		&chaincfg.TestNet3Params,
	}
	// Find the selected chain
	for _, cp := range chains {
		if config.GetString("bitcoind.chain") == cp.Name {
			e.chainParams = cp
			break
		}
	}
	if e.chainParams == nil {
		return nil, fmt.Errorf("Could not find chain %s", config.GetString("bitcoind.chain"))
	}

	// When we get a verack message we are ready to process
	ready := make(chan struct{})

	peerConfig := &peer.Config{
		UserAgentName:    conf.Executable, // User agent name to advertise.
		UserAgentVersion: conf.GitVersion, // User agent version to advertise.
		ChainParams:      e.chainParams,
		Services:         wire.SFNodeWitness,
		TrickleInterval:  time.Second * 10,
		Listeners: peer.MessageListeners{
			OnBlock: e.OnBlock,
			OnTx:    e.OnTx,
			OnInv:   e.OnInv,
			OnVerAck: func(p *peer.Peer, msg *wire.MsgVerAck) {
				e.logger.Debug("Got VerAck")
				close(ready)
			},
		},
	}

	// Do we want to see debug messages
	if config.GetBool("bitcoind.debug_messages") {
		peerConfig.Listeners.OnRead = e.OnRead
		peerConfig.Listeners.OnWrite = e.OnWrite
	}

	// Create peer connection
	e.peer, err = peer.NewOutboundPeer(peerConfig, net.JoinHostPort(config.GetString("bitcoind.host"), config.GetString("bitcoind.port")))
	if err != nil {
		return nil, fmt.Errorf("Could not create outbound peer: %v", err)
	}

	// Establish the connection to the peer address and mark it connected.
	conn, err := net.Dial("tcp", e.peer.Addr())
	if err != nil {
		return nil, fmt.Errorf("Could not Dial peer: %v", err)
	}

	// Start it up
	e.peer.AssociateConnection(conn)

	// Wait until ready or timeout
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("Never got verack ready message")
	}

	e.logger.Infow("Connected to peer", "peer", e.peer.Addr())

	// Get Some Block
	// Genesis
	// e.RequestBlocks("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f", "0")
	// e.RequestBlocks("0000000000000000001dc99dba99a662fbd923c5c50efec19782be8fe1de1d7f", "0")

	// Get the mempool from the peer
	e.RequestMemPool()

	return e, nil

}

// RequestBlocks will send a GetBlocks Message to the peer
func (e *Extractor) RequestBlocks(start string, stop string) error {

	startHash, err := chainhash.NewHashFromStr(start)
	if err != nil {
		return fmt.Errorf("NewHashFromStr: error %v\n", err)
	}
	var locator blockchain.BlockLocator = []*chainhash.Hash{startHash}

	// Stophash - All zero means fetch 500
	stopHash, err := chainhash.NewHashFromStr(stop)
	if err != nil {
		return fmt.Errorf("NewHashFromStr: error %v\n", err)
	}

	err = e.peer.PushGetBlocksMsg(locator, stopHash)
	if err != nil {
		return fmt.Errorf("PushGetBlocksMsg: error %v\n", err)
	}

	return nil

}

// RequestMemPool will send a request for the peers mempool
func (e *Extractor) RequestMemPool() {
	e.peer.QueueMessage(wire.NewMsgMemPool(), nil)
}

// OnTx is called when we receive a transaction
func (e *Extractor) OnTx(p *peer.Peer, msg *wire.MsgTx) {
	e.throttleTxns <- struct{}{}
	go func() {
		e.handleTx(nil, 0, msg)
		<-e.throttleTxns
	}()
}

// OnBlock is called when we receive a block message
func (e *Extractor) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {
	e.throttleBlocks <- struct{}{}
	go func() {
		e.handleBlock(msg, len(buf))
		<-e.throttleBlocks
	}()
}

// OnInv is called when the peer reports it has an inventory item
func (e *Extractor) OnInv(p *peer.Peer, msg *wire.MsgInv) {

	// OnInv is invoked when a peer receives an inv bitcoin message. This is essentially the peer saying I have this piece of information
	// We immediately request that piece of information if it's a transaction or a block
	for _, iv := range msg.InvList {
		switch iv.Type {
		case wire.InvTypeTx:
			e.logger.Debugw("Got Inv", "type", iv.Type, "txid", iv.Hash.String())
			msg := wire.NewMsgGetData()
			err := msg.AddInvVect(iv)
			if err != nil {
				e.logger.Errorw("AddInvVect", "error", err)
			}
			p.QueueMessage(msg, nil)
		case wire.InvTypeBlock:
			e.logger.Debugw("Got Inv", "type", iv.Type, "txid", iv.Hash.String())
			msg := wire.NewMsgGetData()
			err := msg.AddInvVect(iv)
			if err != nil {
				e.logger.Errorw("AddInvVect", "error", err)
			}
			p.QueueMessage(msg, nil)
		}
	}
}

// OnRead is a low level function to capture raw messages coming in
func (e *Extractor) OnRead(p *peer.Peer, bytesRead int, msg wire.Message, err error) {
	e.logger.Debugw("Got Message", "type", reflect.TypeOf(msg), "size", bytesRead, "error", err)
}

// OnWrite is a low level function to capture raw message going out
func (e *Extractor) OnWrite(p *peer.Peer, bytesWritten int, msg wire.Message, err error) {
	e.logger.Debugw("Sent Message", "type", reflect.TypeOf(msg), "size", bytesWritten, "error", err)
}

package btc

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/wire"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/conf"
)

type Extractor struct {
	logger      *zap.SugaredLogger
	peer        *peer.Peer
	chainParams *chaincfg.Params
	bs          BlockStore
	mp          TxMemPool

	sync.WaitGroup
}

func Extract(bs BlockStore, mp TxMemPool) (*Extractor, error) {

	e := &Extractor{
		logger: zap.S().With("package", "block.btc"),
		bs:     bs,
		mp:     mp,
	}

	// Initialize the blockstore for BTC
	err := e.bs.InitBTC()
	if err != nil {
		return nil, fmt.Errorf("Could not InitBTC BlockStore: %s", err)
	}

	err = e.mp.Init()
	if err != nil {
		return nil, fmt.Errorf("Could not Init TxMemPool: %s", err)
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
				close(ready)
				fmt.Printf("OnVerAck: peer:%v msg:%v\n", p, msg)
			},
			// OnRead:    e.OnRead,
			// OnWrite:   e.OnWrite,
		},
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

	// Get Some Block
	// e.RequestBlocks("0000000000000000001dc99dba99a662fbd923c5c50efec19782be8fe1de1d7f", "0")

	// Get the mempool
	e.RequestMemPool()

	return e, nil

}

//
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

func (e *Extractor) RequestMemPool() {
	e.peer.QueueMessage(wire.NewMsgMemPool(), nil)
}

func (e *Extractor) OnTx(p *peer.Peer, msg *wire.MsgTx) {

	e.handleTx(msg)
	e.logger.Debugw("TX Cached",
		"tx", msg.TxHash(),
	)

}

func (e *Extractor) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {

	e.handleBlock(msg, len(buf))
	e.logger.Debugw("Block Inserted",
		"block", msg.BlockHash(),
		"length", len(buf),
	)

}

func (e *Extractor) OnInv(p *peer.Peer, msg *wire.MsgInv) {

	// OnInv is invoked when a peer receives an inv bitcoin message.
	fmt.Printf("OnInv: peer:%v msg:%v\n", p, msg)
	for _, iv := range msg.InvList {
		fmt.Printf("InvVect: %s\n", iv)
		switch iv.Type {
		case wire.InvTypeTx:
			fmt.Printf("InvTypeTx Hash: %s\n", iv.Hash)
			msg := wire.NewMsgGetData()
			err := msg.AddInvVect(iv)
			if err != nil {
				fmt.Printf("AddInvVect error: %v\n", err)
			}
			p.QueueMessage(msg, nil)
		case wire.InvTypeBlock:
			fmt.Printf("InvTypeBlock Hash: %s\n", iv.Hash)
			msg := wire.NewMsgGetData()
			err := msg.AddInvVect(iv)
			if err != nil {
				fmt.Printf("AddInvVect error: %v\n", err)
			}
			p.QueueMessage(msg, nil)
		}
	}
}

// func (e *Extractor) OnRead(p *peer.Peer, bytesRead int, msg wire.Message, err error) {
// 	fmt.Printf("Got message(%d): %v error:%v\n", bytesRead, msg, err)
// }

// func (e *Extractor) OnWrite(p *peer.Peer, bytesWritten int, msg wire.Message, err error) {
// 	fmt.Printf("Sent message(%d): %v error:%v\n", bytesWritten, msg, err)
// }

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

const (
	Symbol            = "btc"
	ScriptTypeUnknown = "unknown"
)

type Extractor struct {
	// Internal stuff
	logger      *zap.SugaredLogger
	peer        *peer.Peer
	chainParams *chaincfg.Params

	// Stores/Pool/Bus
	blockChainStore blocc.BlockChainStore
	validBlockStore blocc.ValidBlockStore
	txBus           blocc.TxBus

	// Frequently Used Settings
	blockFetch                   bool
	blockStoreRaw                bool
	blockConcurrent              int64
	blockValidationInterval      time.Duration
	blockValidationHeightDelta   int64
	blockValidationHeightHoldOff int64

	txFetch                 bool
	txConcurrent            chan struct{}
	txStoreRaw              bool
	txResolvePrevious       bool
	txIgnoreMissingPrevious bool

	// BlockHeaderCache
	blockHeaderCache         blocc.BlockHeaderCache
	blockHeaderCacheLifetime time.Duration

	// BlockHeaderTxMon
	blockHeaderTxMon                 blocc.BlockHeaderTxMonitor
	blockHeaderTxMonBlockWaitTimeout time.Duration
	blockHeaderTxMonBHLifetime       time.Duration
	blockHeaderTxMonTxLifetime       time.Duration

	// How long transactions will sit in the txPool
	txPoolLifetime time.Duration

	// Sync Setting for requesting/waiting for headers to be returns
	waitHeaders chan struct{}

	// A flag to indicate the last processed block had an unknown height
	lastBlockHeightUnknown bool

	sync.WaitGroup
	sync.RWMutex
}

func Extract(blockChainStore blocc.BlockChainStore, txBus blocc.TxBus) (*Extractor, error) {

	e := &Extractor{
		logger:          zap.S().With("package", "blocc.btc"),
		blockChainStore: blockChainStore,
		validBlockStore: blocc.NewValidBlockStoreMem(),
		txBus:           txBus,

		blockFetch:                   txBus == nil,
		blockStoreRaw:                config.GetBool("extractor.btc.block_store_raw"),
		blockConcurrent:              config.GetInt64("extractor.btc.block_concurrent"),
		blockValidationInterval:      config.GetDuration("extractor.btc.block_validation_interval"),
		blockValidationHeightDelta:   config.GetInt64("extractor.btc.block_validation_height_delta"),
		blockValidationHeightHoldOff: config.GetInt64("extractor.btc.block_validation_height_holdoff"),

		txFetch:           txBus != nil,
		txConcurrent:      make(chan struct{}, config.GetInt64("extractor.btc.transaction_concurrent")),
		txStoreRaw:        config.GetBool("extractor.btc.transaction_store_raw"),
		txResolvePrevious: config.GetBool("extractor.btc.transaction_resolve_previous"),

		blockHeaderCache:         blocc.NewBlockHeaderCacheMem(),
		blockHeaderCacheLifetime: config.GetDuration("extractor.btc.bhcache_lifetime"),

		// If we forced the block chain height, we must assume there will be missing transactions
		// If we don't ignore them, we will just retry the same blocks over and over
		txIgnoreMissingPrevious: config.GetInt64("extractor.btc.block_start_height") > blocc.HeightUnknown,

		blockHeaderTxMon:                 blocc.NewBlockHeaderTxMonitorMem(),
		blockHeaderTxMonBlockWaitTimeout: config.GetDuration("extractor.btc.bhtxn_monitor_block_wait_timeout"),
		blockHeaderTxMonBHLifetime:       config.GetDuration("extractor.btc.bhtxn_monitor_block_header_lifetime"),
		blockHeaderTxMonTxLifetime:       config.GetDuration("extractor.btc.bhtxn_monitor_transaction_lifetime"),

		txPoolLifetime: config.GetDuration("extractor.btc.transaction_pool_lifetime"),
	}

	// Output Config
	e.logger.Infow("Starting Extractor",

		"extractor.btc.block_store_raw", e.blockStoreRaw,
		"extractor.btc.block_concurrent", e.blockConcurrent,
		"extractor.btc.block_validation_interval", e.blockValidationInterval,
		"extractor.btc.transaction_resolve_previous", e.txResolvePrevious,

		"extractor.btc.bhcache_lifetime", e.blockHeaderCacheLifetime,

		"extractor.btc.bhtxn_monitor_block_wait_timeout", e.blockHeaderTxMonBlockWaitTimeout,
		"extractor.btc.bhtxn_monitor_block_header_lifetime", e.blockHeaderTxMonBHLifetime,
		"extractor.btc.bhtxn_monitor_transaction_lifetime", e.blockHeaderTxMonTxLifetime,

		"extractor.btc.transaction_pool_lifetime", e.txPoolLifetime,
		"extractor.btc.transaction_store_raw", e.txStoreRaw,
	)
	time.Sleep(2 * time.Second)

	var err error

	// Initialize the BlockChainStore for BTC
	if e.blockChainStore != nil {
		err = e.blockChainStore.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init BlockChainStore: %s", err)
		}
	}

	if e.blockHeaderCache != nil {
		err = e.blockHeaderCache.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init BlockHeaderCache: %s", err)
		}
	}

	// Initialize the TxBus for BTC
	if e.txBus != nil {
		err = e.txBus.Init(Symbol)
		if err != nil {
			return nil, fmt.Errorf("Could not Init TxBus: %s", err)
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
		if config.GetString("extractor.btc.chain") == cp.Name {
			e.chainParams = cp
			break
		}
	}
	if e.chainParams == nil {
		return nil, fmt.Errorf("Could not find chain %s", config.GetString("extractor.btc.chain"))
	}

	// Connect to the peer
	err = e.Connect()
	if err != nil {
		return nil, err
	}

	// Did we provide a blockchain store? If so, we're processing blocks, go fetch the block chain
	if e.blockFetch {

		// Starting point for fetching data - get the highest validated block
		validBlockHeader, err := e.blockChainStore.GetBlockHeaderTopByStatuses(Symbol, []string{blocc.StatusValid})
		if err != nil && err != blocc.ErrNotFound {
			e.logger.Fatalw("blockChainStore.GetBlockHeaderTopByStatuses", "error", err)
		}
		// We have no data or we requested to start at a specific height
		if validBlockHeader == nil || err == blocc.ErrNotFound || config.GetInt64("extractor.btc.block_start_height") != -1 {
			// Set to the start block if we don't have any or for some reason we were requested to start higher
			validBlockHeader = &blocc.BlockHeader{
				BlockId: config.GetString("extractor.btc.block_start_id"),
				Height:  config.GetInt64("extractor.btc.block_start_height"),
			}
		}

		// If we're starting at the genesis block, insert it
		if validBlockHeader.Height == blocc.HeightUnknown { // = -1 = Genesis
			// Store the block ID when we need to reference it
			e.blockHeaderCache.InsertBlockHeader(Symbol, &blocc.BlockHeader{
				BlockId: e.chainParams.GenesisBlock.BlockHash().String(),
				Time:    e.chainParams.GenesisBlock.Header.Timestamp.Unix(),
				Height:  0,
			}, e.blockHeaderCacheLifetime)
			// Set the genesis block as the valid block
			e.validBlockStore.SetValidBlock(&blocc.BlockHeader{Height: blocc.HeightUnknown})
			// Store the block itself, this will update the block chain height to the genesis block
			e.handleBlock(e.chainParams.GenesisBlock)
		} else {
			// Otherwise we're at another/existing block chain height, the blockChainStore is at that height either natrually for forced
			e.blockHeaderCache.InsertBlockHeader(Symbol, validBlockHeader, e.blockHeaderCacheLifetime)
			e.blockHeaderTxMon.AddBlockHeader(validBlockHeader, e.blockHeaderTxMonBHLifetime)
			e.validBlockStore.SetValidBlock(validBlockHeader)
		}

		// Fetch the block chain from here
		go e.fetchBlockChain()

	} else {
		// The block fetcher automatically implenents logic to disconnect and reconnect to peers

	}

	// Get the mempool from the peer when we're not tracking blocks (we're tracking transactions)
	if e.txFetch {
		// Clear the entire mempool
		err := e.blockChainStore.DeleteTransactionsByBlockIdAndTime(Symbol, blocc.BlockIdMempool, nil, nil)
		if err != nil {
			e.logger.Fatalf("Could not empty the mempool:%v", err)
		}
		e.RequestMemPool()

		// Monitor the mempool and resolve any missing transaction inputs
		go func() {
			for {
				time.Sleep(time.Minute)
				err = e.ResolveTxInputs(Symbol, blocc.BlockIdMempool)
				if err != nil {
					e.logger.Errorf("ResolveTxInputs Error: %v", err)
				}
			}
		}()
	}

	// Close the peer if stop signal comes in and clean everything up
	go func() {
		conf.Stop.Add(1) // Hold shutdown until everything flushed
		<-conf.Stop.Chan()
		e.peer.Disconnect()
		e.Wait()                      // Wait until all in progress blocks are handled
		e.blockHeaderTxMon.Shutdown() // Shutdown the monitor
		if e.blockFetch {             // We're tracking blocks
			// Get the current valid block
			valid := e.validBlockStore.GetValidBlock()
			// If we're behind more than blockValidationHeightDelta blocks mark them valid if they are stored
			if valid != nil && valid.Height < int64(e.peer.LastBlock())-e.blockValidationHeightDelta {
				e.logger.Info("Flushing BlockChainStore")
				_, err = e.validateBlocksSimple(valid.Height)
				if err != nil {
					e.logger.Errorf("Flushing BlockChainStore:%v", err)
				}
			}
		}
		conf.Stop.Done()
	}()

	return e, nil

}

// Establish the connection to the peer address and mark it connected.
func (e *Extractor) Connect() error {

	// When we get a verack message we are ready to process
	var err error
	ready := make(chan struct{})

	peerConfig := &peer.Config{
		UserAgentName:    conf.Executable, // User agent name to advertise.
		UserAgentVersion: conf.GitVersion, // User agent version to advertise.
		ChainParams:      e.chainParams,
		Services:         wire.SFNodeWitness,
		TrickleInterval:  time.Second * 10,
		Listeners: peer.MessageListeners{
			OnBlock:   e.OnBlock,
			OnTx:      e.OnTx,
			OnInv:     e.OnInv,
			OnHeaders: e.OnHeaders,
			OnVerAck: func(p *peer.Peer, msg *wire.MsgVerAck) {
				e.logger.Debug("Got VerAck")
				close(ready)
			},
		},
	}

	// Do we want to see debug messages
	if config.GetBool("extractor.btc.debug") {
		peerConfig.Listeners.OnRead = e.OnRead
		peerConfig.Listeners.OnWrite = e.OnWrite
	}

	// Reset extractor state
	e.Lock()
	e.lastBlockHeightUnknown = false
	e.Unlock()

	// Create peer connection
	e.peer, err = peer.NewOutboundPeer(peerConfig, net.JoinHostPort(config.GetString("extractor.btc.host"), config.GetString("extractor.btc.port")))
	if err != nil {
		return fmt.Errorf("Could not create outbound peer: %v", err)
	}

	conn, err := net.Dial("tcp", e.peer.Addr())
	if err != nil {
		return fmt.Errorf("Could not Dial peer: %v", err)
	}

	e.peer.AssociateConnection(conn)

	// Wait until ready or timeout
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("Never got verack ready message")
	}

	e.logger.Infow("Connected to peer", "peer", e.peer.Addr(), "height", e.peer.StartingHeight(), "last_block_height", e.peer.LastBlock())

	return nil

}

// Disconnect will disconnect a peer
func (e *Extractor) Disconnect() {
	e.peer.Disconnect()
}

// RequestHeaders will make a GetHeaders request of the peer
func (e *Extractor) RequestHeaders(start string, stop string) (<-chan struct{}, error) {

	startHash, err := chainhash.NewHashFromStr(start)
	if err != nil {
		return nil, fmt.Errorf("NewHashFromStr: error %v\n", err)
	}
	var locator blockchain.BlockLocator = []*chainhash.Hash{startHash}

	// Stophash - All zero means fetch as many as we can
	stopHash, err := chainhash.NewHashFromStr(stop)
	if err != nil {
		return nil, fmt.Errorf("NewHashFromStr: error %v\n", err)
	}

	// Setup the get headers signal
	e.Lock()
	if e.waitHeaders == nil {
		e.waitHeaders = make(chan struct{})
	}
	defer e.Unlock()

	err = e.peer.PushGetHeadersMsg(locator, stopHash)
	if err != nil {
		return nil, fmt.Errorf("PushGetHeadersMsg: error %v\n", err)
	}

	return e.waitHeaders, nil

}

// OnHeaders is called when the peer sends headers
func (e *Extractor) OnHeaders(p *peer.Peer, msg *wire.MsgHeaders) {
	e.logger.Debugw("Got Headers", "len", len(msg.Headers))

	// Put all the headers in the cache
	go func() {
		for _, h := range msg.Headers {
			prevBlockHeader, err := e.blockHeaderCache.GetBlockHeaderByBlockId(Symbol, h.PrevBlock.String())
			if err != nil || prevBlockHeader == nil {
				e.logger.Warnw("Could not find prevBlock when parsing headers", "error", err, "prevBlockNil", prevBlockHeader == nil)
				continue
			}
			e.blockHeaderCache.InsertBlockHeader(Symbol, &blocc.BlockHeader{
				BlockId:     h.BlockHash().String(),
				Height:      prevBlockHeader.Height + 1,
				PrevBlockId: prevBlockHeader.BlockId,
				Time:        h.Timestamp.Unix(),
			}, e.blockHeaderCacheLifetime)
		}
		e.Lock()
		if e.waitHeaders != nil {
			close(e.waitHeaders)
			e.waitHeaders = nil
		}
		e.Unlock()
	}()
}

// RequestBlocks will send a GetBlocks Message to the peer
func (e *Extractor) RequestBlocks(start string, stop string) error {

	startHash, err := chainhash.NewHashFromStr(start)
	if err != nil {
		return fmt.Errorf("NewHashFromStr: error %v", err)
	}
	var locator blockchain.BlockLocator = []*chainhash.Hash{startHash}

	// Stophash - All zero means fetch 500
	stopHash, err := chainhash.NewHashFromStr(stop)
	if err != nil {
		return fmt.Errorf("NewHashFromStr: error %v", err)
	}

	err = e.peer.PushGetBlocksMsg(locator, stopHash)
	if err != nil {
		return fmt.Errorf("PushGetBlocksMsg: error %v", err)
	}

	return nil

}

// OnBlock is called when we receive a block message
func (e *Extractor) OnBlock(p *peer.Peer, msg *wire.MsgBlock, buf []byte) {

	// If we're only handling transaction, all we really need is the block height
	if e.txFetch {
		// Fetch the previous block, use it's height to update the current block height being saved for mempool transactions
		prevBlk, err := e.blockChainStore.GetBlockByBlockId(Symbol, msg.Header.PrevBlock.String(), blocc.BlockIncludeHeader)
		if err == blocc.ErrNotFound {
			e.Lock()
			e.lastBlockHeightUnknown = true
			e.Unlock()
			return
		} else if err != nil {
			e.logger.Errorf("Error OnBlock e.GetBlockByBlockId: %v", err)
			return
		}
		e.Lock()
		e.lastBlockHeightUnknown = false
		e.Unlock()
		e.peer.UpdateLastBlockHeight(int32(prevBlk.Height + 1))
		return
	}
	// Otherwise handle the block
	go e.handleBlock(msg)
}

// RequestMemPool will send a request for the peers mempool
func (e *Extractor) RequestMemPool() {
	e.peer.QueueMessage(wire.NewMsgMemPool(), nil)
}

// OnTx is called when we receive a transaction
func (e *Extractor) OnTx(p *peer.Peer, msg *wire.MsgTx) {

	go func() {
		// See if we can resolve transactions
		prevOutPoints := make(map[string]*blocc.Tx)
		txIdsInThisBlock := make(map[string]struct{})

		for _, vin := range msg.TxIn {
			// Get the prevOutPoint hash
			hash := vin.PreviousOutPoint.Hash.String()
			// If the cache already has it, populate it. If it's missing it will be nil
			prevOutPoints[hash] = <-e.blockHeaderTxMon.WaitForTxId(hash, 0)
		}

		// Handle only so many concurrent transactions
		e.txConcurrent <- struct{}{}
		defer func() {
			<-e.txConcurrent
		}()

		// Resolve the previous out points
		err := e.getPrevOutPoints(prevOutPoints, nil, txIdsInThisBlock)
		if err != nil {
			e.logger.Errorf("Error OnTx e.getPrevOutPoints: %v", err)
			return
		}

		e.handleTx(nil, blocc.HeightUnknown, msg, prevOutPoints)
	}()
}

// OnInv is called when the peer reports it has an inventory item
func (e *Extractor) OnInv(p *peer.Peer, msg *wire.MsgInv) {

	// OnInv is invoked when a peer receives an inv bitcoin message. This is essentially the peer saying I have this piece of information
	// We immediately request that piece of information if it's a transaction or a block
	for _, iv := range msg.InvList {
		switch iv.Type {
		case wire.InvTypeTx:
			// We're only going to handle transactions when we're not tracking blocks
			if e.txFetch {
				e.logger.Debugw("Got Inv", "type", iv.Type, "txid", iv.Hash.String())
				msg := wire.NewMsgGetData()
				// Request the witness version
				iv.Type |= wire.InvWitnessFlag
				err := msg.AddInvVect(iv)
				if err != nil {
					e.logger.Errorw("AddInvVect", "error", err)
				}
				p.QueueMessage(msg, nil)
			}
		case wire.InvTypeBlock:

			blockHash := iv.Hash.String()
			// Check to see if we've already handled this block
			bh := <-e.blockHeaderTxMon.WaitForBlockId(blockHash, 0)
			if bh == nil {
				e.logger.Debugw("Got Inv", "type", iv.Type, "block_id", blockHash)
				msg := wire.NewMsgGetData()
				// Request the witness version
				iv.Type |= wire.InvWitnessFlag
				err := msg.AddInvVect(iv)
				if err != nil {
					e.logger.Errorw("AddInvVect", "error", err)
				}
				p.QueueMessage(msg, nil)
			} else {
				e.logger.Debugw("Duplicate Inv", "type", iv.Type, "block_id", blockHash)
			}
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

package esearch

import (
	"encoding/json"
	"fmt"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

func (e *esearch) Init(symbol string) error {

	// Index
	indexExists, err := e.client.IndexExists(e.index + "-" + symbol).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to check if index exists: %v", err)
	}
	if indexExists {
		return nil
	}

	// Create the index
	createIndex, err := e.client.CreateIndex(e.index + "-" + symbol).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to create Elasticsearch index: %v", err)
	}
	if !createIndex.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement that Elasticsearch index was created")
	}

	// Create an alias to the linked index
	resp, err := e.client.Alias().Add(e.index+"-"+symbol, e.index).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Unable to link index: %s -> %s error: %v", e.index, e.index+"-"+symbol, err)
	}
	if !resp.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement on index link: %s -> %s", e.index, e.index+"-"+symbol)
	}
	return nil

}

// Insert
func (e *esearch) InsertBlock(symbol string, b *blocc.Block) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(symbol)).
		Type(e.index).
		Id(b.BlockId).
		Doc(b))
	return nil

}

// Upsert
func (e *esearch) UpsertBlock(symbol string, b *blocc.Block) error {

	e.bulk.Add(elastic.NewBulkUpdateRequest().
		Index(e.indexName(symbol)).
		Type(e.index).
		Id(b.BlockId).
		Doc(b).
		DocAsUpsert(true))
	return nil

}

func (e *esearch) InsertTransaction(symbol string, t *blocc.Tx) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(symbol)).
		Type(e.index).
		// Id(t.TxId).
		Doc(t))
	return nil

}

func (e *esearch) Flush(symbol string) error {

	err := e.bulk.Flush()
	if err != nil {
		return err
	}
	_, err = e.client.Flush(e.indexName(symbol)).Do(e.ctx)
	if err != nil {
		return err
	}

	return nil

}

func (e *esearch) GetBlockHeight(symbol string) (string, int64, error) {

	res, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		Sort("height", false).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include("height").Include("block_id")).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return "", 0, err
	}

	if res.Hits.TotalHits == 0 {
		return "", 0, store.ErrNotFound
	}

	var b struct {
		Id     string `json:"block_id"`
		Height int64  `json:"height"`
	}

	err = json.Unmarshal(*res.Hits.Hits[0].Source, &b)

	return b.Id, b.Height, nil
}

func (e *esearch) GetBlockByHeight(symbol string, height int64) (*blocc.Block, error) {

	res, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		Query(elastic.NewTermQuery("height", height)).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, store.ErrNotFound
	}

	// Unmarshal the block
	b := new(blocc.Block)
	err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	return b, err

}

func (e *esearch) GetBlockByBlockId(symbol string, blockId string) (*blocc.Block, error) {

	res, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		Query(elastic.NewTermQuery("block_id", blockId)).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, store.ErrNotFound
	}

	// Unmarshal the block
	b := new(blocc.Block)
	err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	return b, err

}

func (e *esearch) GetBlockIdByHeight(symbol string, height int64) (string, error) {

	res, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		Query(elastic.NewTermQuery("height", height)).
		FetchSource(false).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return "", fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return "", store.ErrNotFound
	}

	return res.Hits.Hits[0].Id, err

}

func (e *esearch) GetHeightByBlockId(symbol string, blockId string) (int64, error) {

	res, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		Query(elastic.NewTermQuery("block_id", blockId)).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include("height")).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return 0, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return 0, store.ErrNotFound
	}

	// Unmarshal the block
	b := new(blocc.Block)
	err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	return b.Height, err

}

func (e *esearch) FindBlocks() ([]*blocc.Block, error) {
	return nil, nil
}

func (e *esearch) indexName(symbol string) string {
	return e.index + "-" + symbol
}

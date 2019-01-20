package esearch

import (
	"encoding/json"
	"fmt"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

// Insert
func (e *esearch) InsertBlock(symbol string, b *blocc.Block) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		// Routing(b.BlockId).
		Id(b.BlockId).
		Doc(b))

	return nil

}

// Upsert
func (e *esearch) UpsertBlock(symbol string, b *blocc.Block) error {

	e.bulk.Add(elastic.NewBulkUpdateRequest().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		// Routing(b.BlockId).
		Id(b.BlockId).
		Doc(b).
		DocAsUpsert(true))

	return nil

}

func (e *esearch) FlushBlocks(symbol string) error {

	err := e.bulk.Flush()
	if err != nil {
		return err
	}
	_, err = e.client.Flush(e.indexName(IndexTypeBlock, symbol)).Do(e.ctx)
	if err != nil {
		return err
	}

	return nil

}

func (e *esearch) GetBlockHeight(symbol string) (string, int64, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
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

	if b.Height > res.Hits.TotalHits+1 {
		return b.Id, b.Height, fmt.Errorf("Missing Blocks Detected height:%d blocks:%d", b.Height, res.Hits.TotalHits)
	} else if b.Height+1 < res.Hits.TotalHits {
		return b.Id, b.Height, fmt.Errorf("Missing Blocks Detected height:%d blocks:%d", b.Height, res.Hits.TotalHits)
	}

	return b.Id, b.Height, nil
}

func (e *esearch) GetBlockByHeight(symbol string, height int64, includeRaw bool) (*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	b := new(blocc.Block)

	query := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("height", height))).
		TerminateAfter(1).
		From(0).Size(1)

	if !includeRaw {
		query.FetchSourceContext(elastic.NewFetchSourceContext(true).Exclude("raw"))
	}

	res, err := query.Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, store.ErrNotFound
	}

	// Unmarshal the block
	err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	return b, err

}

func (e *esearch) GetBlockByBlockId(symbol string, blockId string, includeRaw bool) (*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// query := e.client.Search().
	// 	Index(e.indexName(IndexTypeBlock, symbol)).
	// 	Type(DocType).
	// 	Routing(blockId).
	// 	Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("block_id", blockId))).
	// 	TerminateAfter(1).
	// 	From(0).Size(1)

	query := e.client.Get().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(blockId)

	if !includeRaw {
		query.FetchSourceContext(elastic.NewFetchSourceContext(true).Exclude("raw"))
	}

	res, err := query.Do(e.ctx)
	if elastic.IsNotFound(err) {
		return nil, store.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	// if res.Hits.TotalHits == 0 {
	// 	return nil, store.ErrNotFound
	// }

	// Unmarshal the block
	b := new(blocc.Block)
	// err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	err = json.Unmarshal(*res.Source, b)
	return b, err

}

func (e *esearch) GetBlockIdByHeight(symbol string, height int64) (string, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("height", height))).
		TerminateAfter(1).
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

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// res, err := e.client.Search().
	// 	Index(e.indexName(IndexTypeBlock, symbol)).
	// 	Type(DocType).
	// 	Routing(blockId).
	// 	Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("block_id", blockId))).
	// 	FetchSourceContext(elastic.NewFetchSourceContext(true).Include("height")).
	// 	From(0).Size(1).Do(e.ctx)

	res, err := e.client.Get().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(blockId).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include("height")).
		Do(e.ctx)

	if elastic.IsNotFound(err) {
		return 0, store.ErrNotFound
	} else if err != nil {
		return 0, fmt.Errorf("Could not get block: %v", err)
	}

	// if res.Hits.TotalHits == 0 {
	// 	return 0, store.ErrNotFound
	// }

	// Unmarshal the block
	b := new(blocc.Block)
	// err = json.Unmarshal(*res.Hits.Hits[0].Source, b)
	err = json.Unmarshal(*res.Source, b)
	return b.Height, err

}

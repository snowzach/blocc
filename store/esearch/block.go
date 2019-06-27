package esearch

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

// InsertBlock replaces a block
func (e *esearch) InsertBlock(symbol string, b *blocc.Block) error {

	request := elastic.NewBulkIndexRequest().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(b.BlockId).
		Doc(b)

	request.Source() // Convert to json so we can modify b
	e.bulk.Add(request)

	return nil

}

// UpdateBlock updates status and data based on blockId
func (e *esearch) UpdateBlock(symbol string, blockId string, status string, nextBlockId string, data map[string]string, metric map[string]float64) error {

	// Build block update
	var block = make(map[string]interface{})

	if status != "" {
		block["status"] = status
	}

	if nextBlockId != "" {
		block["next_block_id"] = nextBlockId
	}

	if data != nil {
		block["data"] = data
	}

	if metric != nil {
		block["metric"] = metric
	}

	request := elastic.NewBulkUpdateRequest().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(blockId).
		Doc(&block).
		DocAsUpsert(true)
	// Force create the source so we can manipulate block
	request.Source()
	// Add it to the bulk handler
	e.bulk.Add(request)

	return nil

}

// UpdateBlockStatusByHeight updates status between heights
func (e *esearch) UpdateBlockStatusByStatusesAndHeight(symbol string, statuses []string, startHeight int64, endHeight int64, status string) error {

	query := elastic.NewBoolQuery()

	// If we specify statuses
	if statuses != nil {
		// Convert it to an interface
		statusesInterface := make([]interface{}, len(statuses), len(statuses))
		for i, status := range statuses {
			statusesInterface[i] = status
		}
		if len(statusesInterface) > 0 {
			query.Filter(elastic.NewTermsQuery("status", statusesInterface...))
		}
	}

	// If we specify the height
	if startHeight != blocc.HeightUnknown && endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").From(startHeight).To(endHeight).IncludeLower(true).IncludeUpper(true))
	} else if startHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Gte(startHeight))
	} else if endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Lte(endHeight))
	}

	// If it's already the same status, don't bother
	query.MustNot(elastic.NewTermQuery("status", status))

	// Update the status where the status is not already and it's between height
	_, err := e.client.UpdateByQuery().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(query).
		Script(elastic.NewScript(`ctx._source['status'] = '` + status + `'`).Lang("painless")).
		ScrollSize(2500).
		Do(e.ctx)

	return err

}

// DeleteBlockByBlockId removes a block by BlockId
func (e *esearch) DeleteBlockByBlockId(symbol string, blockId string) error {

	_, err := e.client.Delete().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(blockId).
		Refresh("true").
		Do(e.ctx)
	return err
}

// DeleteAboveBlockHeight removes a block by BlockId
func (e *esearch) DeleteAboveBlockHeight(symbol string, above int64) error {

	_, err := e.client.DeleteByQuery().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(elastic.NewRangeQuery("height").Gt(above)).
		Refresh("true").
		Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Could not DeleteByQuery blocks: %f", err)
	}

	_, err = e.client.DeleteByQuery().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Query(elastic.NewRangeQuery("block_height").Gt(above)).
		Refresh("true").
		Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Could not DeleteByQuery tx: %f", err)
	}

	return nil
}

// FlushBlocks flushes transactions and refreshes the index
func (e *esearch) FlushBlocks(symbol string) error {

	err := e.bulk.Flush()
	if err != nil {
		return err
	}
	_, err = e.client.Refresh(e.indexName(IndexTypeBlock, symbol)).Do(e.ctx)
	if err != nil {
		return err
	}

	// Return the bulk error (if there was one)
	e.Lock()
	err = e.lastBulkError
	e.Unlock()
	return err

}

// GetBlockHeaderTopByStatuses will return the top block header as it stands
func (e *esearch) GetBlockHeaderTopByStatuses(symbol string, statuses []string) (*blocc.BlockHeader, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	query := elastic.NewBoolQuery()
	if statuses != nil {
		// Convert it to an interface
		statusesInterface := make([]interface{}, len(statuses), len(statuses))
		for i, status := range statuses {
			statusesInterface[i] = status
		}
		if len(statusesInterface) > 0 {
			query.Filter(elastic.NewTermsQuery("status", statusesInterface...))
		}
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Sort("height", false).
		Query(query).
		FetchSourceContext(elastic.NewFetchSourceContext(true).Include("height").Include("block_id").Include("prev_block_id").Include("time")).
		From(0).Size(1).Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	var bh = new(blocc.BlockHeader)
	err = json.Unmarshal(*res.Hits.Hits[0].Source, &bh)
	if err != nil {
		return nil, fmt.Errorf("Could not parse BlockHeader: %s", err)
	}

	// Return the height but also an error indicating the height and the number of blocks do not match up (ie missing data)
	if bh.Height > res.Hits.TotalHits+1 {
		return bh, fmt.Errorf("Validation Error: Missing Blocks Detected height:%d blocks:%d", bh.Height, res.Hits.TotalHits)
	} else if bh.Height+1 < res.Hits.TotalHits {
		return bh, fmt.Errorf("Validation Error: Missing Blocks Detected height:%d blocks:%d", bh.Height, res.Hits.TotalHits)
	}

	return bh, nil
}

// GetBlockByBlockId gets a block by blockId
func (e *esearch) GetBlockByBlockId(symbol string, blockId string, include blocc.BlockInclude) (*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Get().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Id(blockId).
		FetchSourceContext(blockFetchSourceContext(include)).
		Do(e.ctx)
	if elastic.IsNotFound(err) {
		return nil, blocc.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	// Unmarshal the block
	b := new(blocc.Block)
	err = json.Unmarshal(*res.Source, b)
	return b, err

}

// GetBlockByTip gets the top block
func (e *esearch) GetBlockTopByStatuses(symbol string, statuses []string, include blocc.BlockInclude) (*blocc.Block, error) {

	// Determine the tip
	bh, err := e.GetBlockHeaderTopByStatuses(symbol, statuses)
	// If it's not a validation error, there's still data
	if err != nil && !blocc.IsValidationError(err) {
		return nil, err
	}

	blk, newErr := e.GetBlockByBlockId(symbol, bh.BlockId, include)
	if newErr != nil && !blocc.IsValidationError(newErr) {
		return nil, newErr
	}

	// Return the block and original error if any
	return blk, err

}

// FindBlocksByHeight fetches blocks by height
func (e *esearch) FindBlocksByHeight(symbol string, height int64, include blocc.BlockInclude) ([]*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	query := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("height", height))
	// If height is zero, we also need to search for blocks with missing height field since it will omitempty
	if height == 0 {
		query = elastic.NewBoolQuery().
			Should(elastic.NewTermQuery("height", height)).
			Should(elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery("height")))
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(query).
		FetchSourceContext(blockFetchSourceContext(include)).
		From(0).Size(e.countMax).Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Block, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Block)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Block: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil
}

// FindBlocksByPrevBlockId returns blocks by previous blockId
func (e *esearch) FindBlocksByPrevBlockId(symbol string, prevBlockId string, include blocc.BlockInclude) ([]*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("prev_block_id", prevBlockId))).
		FetchSourceContext(blockFetchSourceContext(include)).
		From(0).Size(e.countMax).Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Block, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Block)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Block: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil

}

// FindBlocksByTxId returns blocks with TxId
func (e *esearch) FindBlocksByTxId(symbol string, txId string, include blocc.BlockInclude) ([]*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("tx_id", txId))).
		FetchSourceContext(blockFetchSourceContext(include)).
		From(0).Size(e.countMax).
		Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Block, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Block)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Block: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil

}

// FindBlocksByBlockIdsAndTime returns blocks optionally by blockId, time and pagination by descending time
func (e *esearch) FindBlocksByBlockIdsAndTime(symbol string, blockIds []string, start *time.Time, end *time.Time, include blocc.BlockInclude, offset int, count int) ([]*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// Convert it to an interface
	blockIdsInterface := make([]interface{}, len(blockIds), len(blockIds))
	for i, blockId := range blockIds {
		blockIdsInterface[i] = blockId
	}

	query := elastic.NewBoolQuery()
	if len(blockIdsInterface) > 0 {
		query.Filter(elastic.NewTermsQuery("block_id", blockIdsInterface...))
	}
	if start != nil && end != nil {
		query.Filter(elastic.NewRangeQuery("time").From(start.Unix()).To(end.Unix()).IncludeLower(true).IncludeUpper(true))
	} else if start != nil {
		query.Filter(elastic.NewRangeQuery("time").Gte(start.Unix()))
	} else if end != nil {
		query.Filter(elastic.NewRangeQuery("time").Lte(end.Unix()))
	}

	// Max results
	if count == store.CountMax {
		count = e.countMax
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Sort("time", false).
		Query(query).
		FetchSourceContext(blockFetchSourceContext(include)).
		From(offset).Size(count).
		Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Block, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Block)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Block: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil
}

// FindBlocksByStatusAndHeight returns blocks by status and height ascending height
func (e *esearch) FindBlocksByStatusAndHeight(symbol string, statuses []string, startHeight int64, endHeight int64, include blocc.BlockInclude, offset int, count int) ([]*blocc.Block, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	query := elastic.NewBoolQuery()
	if statuses != nil {
		// Convert it to an interface
		statusesInterface := make([]interface{}, len(statuses), len(statuses))
		for i, status := range statuses {
			statusesInterface[i] = status
		}
		if len(statusesInterface) > 0 {
			query.Filter(elastic.NewTermsQuery("status", statusesInterface...))
		}
	}

	if startHeight != blocc.HeightUnknown && endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").From(startHeight).To(endHeight).IncludeLower(true).IncludeUpper(true))
	} else if startHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Gte(startHeight))
	} else if endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Lte(endHeight))
	}

	// Max results
	if count == store.CountMax {
		count = e.countMax
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Sort("height", true).
		Query(query).
		FetchSourceContext(blockFetchSourceContext(include)).
		From(offset).Size(count).
		Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Block, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Block)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Block: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil
}

// This adds a fetch filter to the document so we only fetch what we need
func blockFetchSourceContext(include blocc.BlockInclude) *elastic.FetchSourceContext {
	// Don't filter anything
	if include == blocc.BlockIncludeAll {
		return elastic.NewFetchSourceContext(true)
	}
	// It's easier to exclude things
	excludes := make([]string, 0)

	if include&blocc.BlockIncludeData == 0 {
		excludes = append(excludes, "data.*")
	}
	if include&blocc.BlockIncludeRaw == 0 {
		excludes = append(excludes, "raw")
	}
	if include&blocc.BlockIncludeTxIds == 0 {
		excludes = append(excludes, "tx_id")
	}

	return elastic.NewFetchSourceContext(true).Exclude(excludes...)

}

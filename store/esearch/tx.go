package esearch

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

// InsertTransaction inserts a transaction to the database
func (e *esearch) InsertTransaction(symbol string, t *blocc.Tx) error {

	request := elastic.NewBulkIndexRequest().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Id(t.TxId).
		Doc(t)

	request.Source() // Turn it into JSON such that we can modify the tx

	e.bulk.Add(request)

	return nil

}

// UpsertTransaction updates block data (essentially merging data object)
func (e *esearch) UpsertTransaction(symbol string, t *blocc.Tx) error {

	request := elastic.NewBulkUpdateRequest().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Id(t.TxId).
		Doc(t).
		DocAsUpsert(true)
	// Turn it into JSON such that we can modify the tx
	request.Source()
	// Add it to the bulk handler
	e.bulk.Add(request)

	return nil

}

// DeleteTransactionsByBlockIdAndTime will remove transactions by BlockId
func (e *esearch) DeleteTransactionsByBlockIdAndTime(symbol string, blockId string, start *time.Time, end *time.Time) error {

	query := elastic.NewBoolQuery().Filter(elastic.NewTermQuery("block_id", blockId))

	if start != nil && end != nil {
		query.Filter(elastic.NewRangeQuery("time").From(start.Unix()).To(end.Unix()).IncludeLower(true).IncludeUpper(true))
	} else if start != nil {
		query.Filter(elastic.NewRangeQuery("time").Gte(start.Unix()))
	} else if end != nil {
		query.Filter(elastic.NewRangeQuery("time").Lte(end.Unix()))
	}

	_, err := e.client.DeleteByQuery().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Query(query).
		Refresh("true").
		Do(e.ctx)
	return err
}

// GetTxByTxId will return a transaction by txId
func (e *esearch) GetTxByTxId(symbol string, txId string, include blocc.TxInclude) (*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Get().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Id(txId).
		FetchSourceContext(txFetchSourceContext(include)).
		Do(e.ctx)

	if elastic.IsNotFound(err) {
		return nil, blocc.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("Could not get tx: %v", err)
	}

	// Unmarshal the Tx
	tx := new(blocc.Tx)
	err = json.Unmarshal(*res.Source, tx)
	return tx, err

}

// GetTxsByTxIds returns multiple transaction by multiple txIds
func (e *esearch) GetTxsByTxIds(symbol string, txIds []string, include blocc.TxInclude) ([]*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	index := e.indexName(IndexTypeTx, symbol)
	query := e.client.MultiGet()
	fsc := txFetchSourceContext(include)

	// Add all the ids
	for _, txId := range txIds {
		query.Add(elastic.NewMultiGetItem().Index(index).Type(DocType).Id(txId).FetchSource(fsc))
	}

	res, err := query.Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get block: %v", err)
	}

	txs := make([]*blocc.Tx, 0)

	for _, doc := range res.Docs {
		if doc.Source == nil {
			continue
		}
		t := new(blocc.Tx)
		err = json.Unmarshal(*doc.Source, t)
		txs = append(txs, t)
	}

	return txs, err

}

// GetTxsByBlockId returns multiple transaction by block Id
func (e *esearch) GetTxsByBlockId(symbol string, blockId string, include blocc.TxInclude) ([]*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Sort("height", true).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("block_id", blockId))).
		FetchSourceContext(txFetchSourceContext(include)).
		From(0).Size(e.countMax).
		Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Tx, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Tx)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Tx: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil

}

// GetTxCountByBlockId will return the numbner of trasnaction by block Id in the database
func (e *esearch) GetTxCountByBlockId(symbol string, blockId string) (int64, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("block_id", blockId))).
		FetchSource(false).
		Size(0).
		Do(e.ctx)
	if err != nil {
		return 0, err
	}

	return int64(res.Hits.TotalHits), nil
}

// FindTxsByTxIdsAndTime will find multiple transactions by optionally multiple txids, time and pagination
func (e *esearch) FindTxsByTxIdsAndTime(symbol string, txIds []string, start *time.Time, end *time.Time, include blocc.TxInclude, offset int, count int) ([]*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// Convert it to an interface
	txidsInterface := make([]interface{}, len(txIds), len(txIds))
	for i, txid := range txIds {
		txidsInterface[i] = txid
	}

	query := elastic.NewBoolQuery()
	if len(txidsInterface) != 0 {
		query.Filter(elastic.NewTermsQuery("_id", txidsInterface...))
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
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Sort("time", false).
		Query(query).
		FetchSourceContext(txFetchSourceContext(include)).
		From(offset).Size(count).
		Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Tx, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Tx)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Tx: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil

}

// FindTxsByAddressesAndTime will find transactions by optiojnally addresses , time and pagination
func (e *esearch) FindTxsByAddressesAndTime(symbol string, addresses []string, start *time.Time, end *time.Time, filter int, include blocc.TxInclude, offset int, count int) ([]*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// Convert it to an interface
	addressesInterface := make([]interface{}, len(addresses), len(addresses))
	for i, address := range addresses {
		addressesInterface[i] = address
	}

	query := elastic.NewBoolQuery()
	// Address filtering
	if filter&blocc.TxFilterAddressInput != 0 && filter&blocc.TxFilterAddressOutput != 0 {
		// We use a copy_to mapping on in.out.address and out.address to just a field called address to cover any address in a tx
		query.Filter(elastic.NewTermsQuery("address", addressesInterface...))
	} else if filter&blocc.TxFilterAddressInput != 0 {
		query.Filter(elastic.NewTermsQuery("in.out.address", addressesInterface...))
	} else if filter&blocc.TxFilterAddressOutput != 0 {
		query.Filter(elastic.NewTermsQuery("out.address", addressesInterface...))
	}
	// Time Filtering
	if start != nil && end != nil {
		query.Filter(elastic.NewRangeQuery("time").From(start.Unix()).To(end.Unix).IncludeLower(true).IncludeUpper(true))
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
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Sort("time", false).
		Query(query).
		FetchSourceContext(txFetchSourceContext(include)).
		From(offset).Size(count).
		Do(e.ctx)
	if err != nil {
		return nil, err
	}

	if res.Hits.TotalHits == 0 {
		return nil, blocc.ErrNotFound
	}

	ret := make([]*blocc.Tx, len(res.Hits.Hits), len(res.Hits.Hits))

	for i, hit := range res.Hits.Hits {
		tx := new(blocc.Tx)
		err := json.Unmarshal(*hit.Source, &tx)
		if err != nil {
			return nil, fmt.Errorf("Could not parse Tx: %s", err)
		}
		ret[i] = tx
	}

	return ret, nil

}

// GetMemPoolStats returns the size and count of the mempool (blockId = blocc.BlockIdMempool)
func (e *esearch) GetMemPoolStats(symbol string) (int64, int64, error) {

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Sort("time", false).
		Query(elastic.NewTermQuery("block_id", blocc.BlockIdMempool)).
		Aggregation("size", elastic.NewSumAggregation().Field("size")).
		Size(0).
		Do(e.ctx)
	if err != nil {
		return 0, 0, err
	}

	count := int64(res.Hits.TotalHits)
	var size int64
	if flSize, found := res.Aggregations.Sum("size"); found {
		size = int64(*flSize.Value)
	}
	return size, count, nil

}

// GetAddressStats returns statistics for an address
func (e *esearch) GetAddressStats(symbol string, address string) (int64, int64, int64, error) {

	agg := elastic.NewScriptedMetricAggregation().
		Params(map[string]interface{}{"address": address}).
		InitScript(elastic.NewScript(`state.input_value = 0L; state.output_value = 0L;`)).
		MapScript(elastic.NewScript(`
			for (input in params._source.in) {
				if (input.out != null && input.out.address != null && input.out.address.length > 0) {
					for (addr in input.out.address) {
						if (addr == params.address) {
							state.input_value += input.out.value;
						}
					}
				}
			}
			for (output in params._source.out) {
				if (output.address != null && output.address.length > 0) {
					for (addr in output.address) {
						if (addr == params.address) {
							state.output_value += output.value;
						}
					}
				}
			}
		`)).
		CombineScript(elastic.NewScript("return state;")).
		ReduceScript(elastic.NewScript(`
			Map ret = new HashMap();
			ret.input_value = 0L;
			ret.output_value = 0L;
			for (state in states) { 
				ret.input_value += state.input_value;
				ret.output_value += state.output_value; 
			}
			return ret;
		`))

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Query(elastic.NewTermQuery("address", address)).
		Aggregation("stats", agg).
		Size(0).
		Do(e.ctx)
	if err != nil {
		return 0, 0, 0, err
	}

	stats, ok := res.Aggregations["stats"]
	if !ok {
		return 0, 0, 0, fmt.Errorf("Did not find address stats")
	}

	var aggValue struct {
		Value struct {
			InputValue  int64 `json:"input_value"`
			OutputValue int64 `json:"output_value"`
		} `json:"value"`
	}

	// Capture the value
	if err := json.Unmarshal(*stats, &aggValue); err != nil {
		return 0, 0, 0, fmt.Errorf("Could not process aggregation data: %v", err)
	}

	return int64(res.Hits.TotalHits), aggValue.Value.OutputValue, aggValue.Value.InputValue, nil

}

// FlushTransactions will flush inserts and refreshes the indexes
func (e *esearch) FlushTransactions(symbol string) error {

	err := e.bulk.Flush()
	if err != nil {
		return err
	}
	_, err = e.client.Refresh(e.indexName(IndexTypeTx, symbol)).Do(e.ctx)
	if err != nil {
		return err
	}

	// Return the bulk error (if there was one)
	e.Lock()
	err = e.lastBulkError
	e.Unlock()
	return err

}

// This adds a fetch filter to the document so we only fetch what we need
func txFetchSourceContext(include blocc.TxInclude) *elastic.FetchSourceContext {
	// Don't filter anything
	if include == blocc.TxIncludeAll {
		return elastic.NewFetchSourceContext(true)
	}
	// It's easier to exclude things
	excludes := make([]string, 0)

	if include&blocc.TxIncludeData == 0 {
		excludes = append(excludes, "data.*")
	}
	if include&blocc.TxIncludeRaw == 0 {
		excludes = append(excludes, "raw")
	}
	if include&blocc.TxIncludeIn == 0 {
		excludes = append(excludes, "in.*")
	}
	if include&blocc.TxIncludeOut == 0 {
		excludes = append(excludes, "out.*")
	}

	return elastic.NewFetchSourceContext(true).Exclude(excludes...)

}

package esearch

import (
	"encoding/json"
	"fmt"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/store"
)

func (e *esearch) InsertTransaction(symbol string, t *blocc.Tx) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		// Routing(t.TxId).
		Id(t.TxId).
		Doc(t))

	return nil

}

func (e *esearch) GetTxByTxId(symbol string, txId string, includeRaw bool) (*blocc.Tx, error) {

	e.throttleSearches <- struct{}{}
	defer func() {
		<-e.throttleSearches
	}()

	// query := e.client.Search().
	// 	Index(e.indexName(IndexTypeTx, symbol)).
	// 	Type(DocType).
	// 	Query(elastic.NewBoolQuery().Filter(elastic.NewTermQuery("tx_id", txId))).
	// 	Routing(txId).
	// 	FetchSource(false).
	// 	From(0).Size(1)

	query := e.client.Get().
		Index(e.indexName(IndexTypeTx, symbol)).
		Type(DocType).
		Id(txId)

	if !includeRaw {
		query.FetchSourceContext(elastic.NewFetchSourceContext(true).Exclude("raw"))
	}

	res, err := query.Do(e.ctx)
	if elastic.IsNotFound(err) {
		return nil, store.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("Could not get tx: %v", err)
	}

	// if res.Hits.TotalHits == 0 {
	// 	return nil, store.ErrNotFound
	// }

	// Unmarshal the Tx
	tx := new(blocc.Tx)
	// err = json.Unmarshal(*res.Hits.Hits[0].Source, tx)
	err = json.Unmarshal(*res.Source, tx)
	return tx, err

}

func (e *esearch) FlushTransactions(symbol string) error {

	err := e.bulk.Flush()
	if err != nil {
		return err
	}
	_, err = e.client.Flush(e.indexName(IndexTypeTx, symbol)).Do(e.ctx)
	if err != nil {
		return err
	}

	return nil

}

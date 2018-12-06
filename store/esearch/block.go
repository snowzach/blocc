package esearch

import (
	"fmt"

	"github.com/olivere/elastic"

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
func (e *esearch) InsertBlock(symbol string, b *store.Block) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(symbol)).
		Type(e.index).
		Id(b.BlockId).
		Doc(b))
	return nil

}

// Upsert
func (e *esearch) UpsertBlock(symbol string, b *store.Block) error {

	_, err := e.client.Update().
		Index(e.indexName(symbol)).
		Type(e.index).
		Id(b.BlockId).
		Doc(b).
		Do(e.ctx)
	return err

}

func (e *esearch) InsertTransaction(symbol string, t *store.Tx) error {

	e.bulk.Add(elastic.NewBulkIndexRequest().
		Index(e.indexName(symbol)).
		Type(e.index).
		Id(t.TxId).
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

func (e *esearch) GetBlockHeight(symbol string) (int64, error) {

	// Get the highest height block
	search := elastic.NewSearchSource()
	search.Aggregation("height", elastic.NewMaxAggregation().Field("height"))
	search.Size(0)
	resp, err := e.client.Search().
		Index(e.indexName(symbol)).
		Type(e.index).
		SearchSource(search).Do(e.ctx)
	if err != nil {
		return 0, err
	}

	e.logger.Warnw("AGG", "agg", resp)

	return 0, nil
}

func (e *esearch) FindBlocks() ([]*store.Block, error) {
	return nil, nil
}

func (e *esearch) indexName(symbol string) string {
	return e.index + "-" + symbol
}

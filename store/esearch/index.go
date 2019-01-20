package esearch

import (
	"fmt"
)

const (
	IndexTypeBlock = "block"
	IndexTypeTx    = "tx"
)

func (e *esearch) Init(symbol string) error {

	err := e.EnsureIndex(IndexTypeBlock, symbol)
	if err != nil {
		return err
	}
	err = e.EnsureIndex(IndexTypeTx, symbol)
	if err != nil {
		return err
	}
	return nil

}

func (e *esearch) EnsureIndex(names ...string) error {

	indexName := e.indexName(names...)

	// Index
	indexExists, err := e.client.IndexExists(indexName).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to check if index exists: %v", err)
	}
	if indexExists {
		return nil
	}

	// Create the index
	createIndex, err := e.client.CreateIndex(indexName).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to create Elasticsearch index: %v", err)
	}
	if !createIndex.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement that Elasticsearch index was created")
	}

	// Create an alias to the linked index
	resp, err := e.client.Alias().Add(indexName, e.index).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Unable to link index: %s -> %s error: %v", e.index, indexName, err)
	}
	if !resp.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement on index link: %s -> %s", e.index, indexName)
	}
	return nil
}

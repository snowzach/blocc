package esearch

import (
	"fmt"

	"git.coinninja.net/backend/blocc/blocc/btc"
)

func (e *esearch) InitBTC() error {

	// Index
	indexExists, err := e.client.IndexExists(e.index + "-" + btc.Symbol).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to check if index exists: %v", err)
	}
	if indexExists {
		return nil
	}

	// Create the index
	createIndex, err := e.client.CreateIndex(e.index + "-" + btc.Symbol).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to create Elasticsearch index: %v", err)
	}
	if !createIndex.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement that Elasticsearch index was created")
	}

	// Create an alias to the linked index
	resp, err := e.client.Alias().Add(e.index+"-"+btc.Symbol, e.index).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Unable to link index: %s -> %s error: %v", e.index, e.index+"-"+btc.Symbol, err)
	}
	if !resp.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement on index link: %s -> %s", e.index, e.index+"-"+btc.Symbol)
	}
	return nil

}

// Insert
func (e *esearch) InsertBTCBlock(b *btc.Block) error {

	_, err := e.client.Index().
		Index(e.index + "-" + btc.Symbol).
		Type(e.index).
		Id(b.Hash).
		BodyJson(b).
		Do(e.ctx)
	return err

}

// Upsert
func (e *esearch) UpsertBTCBlock(b *btc.Block) error {

	_, err := e.client.Update().
		Index(e.index + "-" + btc.Symbol).
		Type(e.index).
		Id(b.Hash).
		Doc(b).
		Do(e.ctx)
	return err

}

func (e *esearch) FindBTCBlocks() ([]*btc.Block, error) {
	return nil, nil
}

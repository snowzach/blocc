package esearch6

import (
	"fmt"
	"strings"

	"github.com/spf13/cast"
)

// Constants used for elastic
const (
	IndexTypeBlock = "block"
	IndexTypeTx    = "tx"
)

// Init will initializes the elastic index
func (e *esearch) Init(symbol string) error {

	err := e.EnsureIndex(IndexTypeBlock, symbol)
	if err != nil {
		return err
	}

	err = e.CheckIndex(IndexTypeBlock, symbol)
	if err != nil {
		return err
	}

	err = e.EnsureIndex(IndexTypeTx, symbol)
	if err != nil {
		return err
	}

	err = e.CheckIndex(IndexTypeTx, symbol)
	if err != nil {
		return err
	}

	return nil

}

// EnsureIndex is used to ensure an index exists with proper settings
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

// CheckIndex validates that an index is working properly
func (e *esearch) CheckIndex(names ...string) error {

	indexName := e.indexName(names...)

	indexes, err := e.client.IndexGetSettings(indexName).FlatSettings(true).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to check index settings: %v", err)
	}

	settings, found := indexes[indexName]
	if !found {
		return fmt.Errorf("Failed to find index settings")
	}

	for setting, value := range settings.Settings {
		if strings.Contains(setting, "read_only") && cast.ToBool(value) {
			return fmt.Errorf("Index %s is read_only", indexName)
		}
	}

	return nil

}

package esearch

import (
	"git.coinninja.net/backend/blocc/blocc/btc"
)

// Insert
func (e *esearch) InsertBlockBTC(b *btc.Block) error {

	_, err := e.client.Index().
		Index(e.index).
		Type("block").
		BodyJson(b).
		Id(b.Hash).
		Do(e.ctx)
	return err

}

// Upsert
func (e *esearch) UpsertBlockBTC(b *btc.Block) error {

	_, err := e.client.Update().
		Index(e.index).
		Id(b.Hash).
		Type("block").
		Doc(b).
		Do(e.ctx)
	return err

}

func (e *esearch) FindBlocksBTC() ([]*btc.Block, error) {
	return nil, nil
}

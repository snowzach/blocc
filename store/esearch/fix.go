package esearch

import (
	"context"
	"encoding/json"

	"github.com/olivere/elastic"
	"github.com/spf13/cast"

	"git.coinninja.net/backend/blocc/blocc"
	"git.coinninja.net/backend/blocc/conf"
)

var fixSymbol = "btc"

// Fix is used to make repairs... Not normally used
func (e *esearch) Fix() {

	top, err := e.GetBlockHeaderTopByStatuses(fixSymbol, nil)
	if err != nil {
		e.logger.Fatal("Could not GetBlockHeaderTopByStatus", "error", err)
	}
	e.logger.Infow("Starting fix", "height", top.Height)

	// var height = int64(0)

	for !conf.Stop.Bool() {

		// query := elastic.NewBoolQuery()

		// query.Filter(elastic.NewQueryStringQuery("-nex"))
		// // query.Filter(elastic.NewRangeQuery("time").From(0).To(1000).IncludeLower(true).IncludeUpper(true))
		// s, _ := query.Source()
		// j, _ := json.Marshal(s)
		// e.logger.Fatalf("SOURCE: %v", string(j))

		res, err := e.client.Search().
			Index(e.indexName(IndexTypeBlock, fixSymbol)).
			Type(DocType).
			Query(elastic.NewQueryStringQuery("-next_block_id:* AND height:<"+cast.ToString(top.Height-1))).
			// Query(elastic.NewQueryStringQuery("height:>"+cast.ToString(height))).
			// Query(elastic.NewQueryStringQuery("(-_exists_:tx_count) OR (NOT status:valid) OR (-_exists_:status)")).
			// Query(elastic.NewQueryStringQuery("_exists_:data.tx_count OR _exists_:address")).
			FetchSourceContext(blockFetchSourceContext(blocc.BlockIncludeHeader)).
			Sort("height", false).
			Size(1000).Do(context.Background())
		if err != nil {
			e.logger.Fatalw("Search error", "error", err)
		}

		if res.Hits.TotalHits == 0 {
			break
		}

		var b blocc.Block
		var nextBlockIdByBlockId = make(map[string]string)

		for _, hit := range res.Hits.Hits {

			err := json.Unmarshal(*hit.Source, &b)
			if err != nil {
				e.logger.Fatalf("Could not unmarshal block: %v", err)
			}

			e.logger.Infof("Handling: %d", b.Height)

			nextBlockIdByBlockId[b.PrevBlockId] = b.BlockId

			nextBlockId, ok := nextBlockIdByBlockId[b.BlockId]
			if !ok {
				blks, err := e.FindBlocksByPrevBlockId(fixSymbol, b.BlockId, blocc.BlockIncludeHeader)
				if err != nil || len(blks) == 0 {
					e.logger.Fatalf("FindBlocksByPrevBlockId: %v", err)
				}
				nextBlockId = blks[0].BlockId
			}

			data := map[string]string{
				"next_block_id": nextBlockId,
			}

			request := elastic.NewBulkUpdateRequest().
				Index(e.indexName(IndexTypeBlock, fixSymbol)).
				Type(DocType).
				Id(b.BlockId).
				// Script(elastic.NewScript("ctx._source.remove('address'); ctx._source.data.remove('tx_count');").Lang("painless"))
				Doc(data).
				DocAsUpsert(true)

				// Add it to the bulk handler
			e.bulk.Add(request)

			// height = b.Height
		}

		err = e.FlushBlocks(fixSymbol)
		if err != nil {
			e.logger.Fatalf("FlushBlocksErr: %v", err)
		}

	}

	// e.FlushBlocks(fixSymbol)

}

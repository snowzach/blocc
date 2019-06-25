package esearch

import (
	"fmt"

	"github.com/olivere/elastic"

	"git.coinninja.net/backend/blocc/blocc"
)

// AverageBlockDataFieldByHeight returns blocks by status and height ascending height
func (e *esearch) AverageBlockDataFieldByHeight(symbol string, field string, omitZero bool, startHeight int64, endHeight int64) (float64, error) {

	query := elastic.NewBoolQuery()

	// Make sure the field has a value
	query.Filter(elastic.NewExistsQuery(field))

	// Filter by blockHeight
	if startHeight != blocc.HeightUnknown && endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").From(startHeight).To(endHeight).IncludeLower(true).IncludeUpper(true))
	} else if startHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Gte(startHeight))
	} else if endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Lte(endHeight))
	}

	// Skip zeros
	if omitZero {
		query.MustNot(elastic.NewTermsQuery(field, "0", "0.0"))
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(query).
		Aggregation("thisagg", elastic.NewAvgAggregation().Script(elastic.NewScript(fmt.Sprintf(`Double.parseDouble(doc["%s"].value)`, field)))).
		Size(0).
		Do(e.ctx)
	if err != nil {
		return 0, err
	}

	if res.Hits.TotalHits == 0 {
		return 0, blocc.ErrNotFound
	}

	if value, found := res.Aggregations.Avg("thisagg"); found {
		return *value.Value, nil
	}

	return 0, blocc.ErrNotFound
}

// AverageBlockDataFieldByHeight returns blocks by status and height ascending height
func (e *esearch) PercentileBlockDataFieldByHeight(symbol string, field string, percentile float64, omitZero bool, startHeight int64, endHeight int64) (float64, error) {

	query := elastic.NewBoolQuery()

	// Make sure the field has a value
	query.Filter(elastic.NewExistsQuery(field))

	// Filter by blockHeight
	if startHeight != blocc.HeightUnknown && endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").From(startHeight).To(endHeight).IncludeLower(true).IncludeUpper(true))
	} else if startHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Gte(startHeight))
	} else if endHeight != blocc.HeightUnknown {
		query.Filter(elastic.NewRangeQuery("height").Lte(endHeight))
	}

	// Skip zeros
	if omitZero {
		query.MustNot(elastic.NewTermsQuery(field, "0", "0.0"))
	}

	res, err := e.client.Search().
		Index(e.indexName(IndexTypeBlock, symbol)).
		Type(DocType).
		Query(query).
		Aggregation("thisagg", elastic.NewPercentilesAggregation().Percentiles(percentile).Script(elastic.NewScript(fmt.Sprintf(`Double.parseDouble(doc["%s"].value)`, field)))).
		Size(0).
		Do(e.ctx)
	if err != nil {
		return 0, err
	}

	if res.Hits.TotalHits == 0 {
		return 0, blocc.ErrNotFound
	}

	if percentiles, found := res.Aggregations.Percentiles("thisagg"); found {
		// Only one value in the map, grab it and return it
		for _, value := range percentiles.Values {
			return value, nil
		}
	}

	return 0, blocc.ErrNotFound
}

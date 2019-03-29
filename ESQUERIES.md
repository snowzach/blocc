GET /blocc-block-btc/_search
{
  "query": {
    "query_string": {
      "query": "height:*"
    }
  },
  "_source": { "excludes": "raw"},
  "size": 1
}

GET /blocc-tx-btc/_search
{
  "query": {
    "bool": {
      "filter": {
        "term": {
          "tx_id": {
            "value": "1370c5420340e218702c136711f21d2ca4662fa9ccbebb8c9e3b3ab9fd2f8f04"
          }        
        }
      }
    }
  }
}

GET /blocc-tx-btc/_search
{
  "query": {
    "bool": {
      "must_not": [
        { "exists": { "field": "in.out" } },
        {"term": { "in.tx_id":"0000000000000000000000000000000000000000000000000000000000000000" }}
      ],
      "filter": {
        "query_string": {
          "query": "height:*"
        }        
      }
    }
  },
  "_source": { "excludes": "raw"},
  "size": 20
}

## Mempool Transaction Fee/VSize Histogram
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "block_id:mempool"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 0,
  "aggs": {
    "fee_vsize_pctile": {
      "percentiles": {
        "script" : {
          "source": "params['_source']['metric']['fee_vsize'];" 
        },
        "percents" : [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,25,50,75,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99]
      }
    },
    "fee_size_average": {
      "percentiles": {
        "script" : {
          "source": "Double.parseDouble(params['_source']['data']['fee'])/doc['size'].value" 
        },
        "percents" : [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,25,50,75,80,81,82,83,84,85,86,87,88,89,90,91,92,93,94,95,96,97,98,99]
      }
    }
  }
}

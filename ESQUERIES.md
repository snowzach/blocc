# Field definitions
This is a list of the fields that are available in elasticsearch and the ones you would typically search on

## Index blocc-block-btc
* block_id - The block hash
* height - The block height if known (otherwise -1)
* tx_count - The number of transactions in the block
* size - The size of the block
* prev_block_id - The previous block id
* next_block_id - The next block id if known
* time - The block unix timestamp
* data.* - Various metadata fields
* raw - Base64 Encoded raw block

## Index block-tx-btc
* block_id - The block hash if part of a block or `mempool` if in the mempool
* block_height - The block height if known (otherwise -1)
* tx_id - The transaction id
* height - The height of the transaction in a block if known
* size - The size of the block
* time - The transaction unix timestamp (the time it was received or the block time)
* data.* - Various metadata fields
* raw - Base64 encoded raw transaction
* in - Array of inputs
  * tx_id - Previous transaction output txid
  * height - Height of the output in the previous transaction output
  * out - The output that corresponds to txid and height
  * data.* - Various metadata fields related to the input
* out - Array of outputs
  * address - Array of addresses if known
  * raw - Base64 encoded output script 
  * type - The type of script if it could be decoded
  * value - The amount of the transaction
  * data.* - Various metadata fields related to the output

To search for a transaction using use the queries:
  An address as an input: in.out.address:<address>
  An address as an output: out.address:<address>
  Transactions in block_id: block_id:<block_id>
There is a special field called `address` on a transaction that combines both input and output addresses for faster searching

# Show block count and return the top block (hide raw and transaction ids)
POST /blocc-block-btc/_search
{
  "query": {
    "query_string": {
      "query": "height:*"
    }
  },
  "_source": {"excludes": ["raw","tx_id"]},
  "sort": [{"height": {"order": "desc"}}], 
  "size": 1
}

# Search for all transactions in the mempool (only return 1 but show count)
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "block_id:mempool"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 1
}


# Search for transaction id
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "tx_id:e591f7966e75bacdeeb175d8855507202fecb34d28bb183d52ed480f25ff7da6"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 1
}

# Search for address transactions (address is a hidden field that contains all input and output addresses)
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "address:1J9dijMtYCxkEcpV19JVjbHLzMJ2cZezJN"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 1
}

# Search for input address transactions (in.out.address)
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "in.out.address:1J9dijMtYCxkEcpV19JVjbHLzMJ2cZezJN"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 1
}

# Search for output address transactions (in.out.address)
POST /blocc-tx-btc/_search
{
  "query": {
    "query_string": {
      "query": "out.address:1J9dijMtYCxkEcpV19JVjbHLzMJ2cZezJN"
    }
  },
  "_source": {"excludes": ["raw"]},
  "size": 1
}


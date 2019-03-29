# BLOCC

This is a bitcoin node client that monitors blocks and transactions as well as logs stats and metrics with a GRPC/REST API

## Compiling
This is designed as a go module aware program and thus requires go 1.11 or better
You can clone it anywhere, just run `make` inside the cloned directory to build

## Requirements
The system requires Redis and Elasitcsearch to function. 
 - Elasticsearch serves as the blockchain store as well as the memory pool.
 - Redis is used for caching as well as pub/sub for live streaming of mempool data

## Initial Indexing
If you start the system from scratch, it will build an index in elasticsearch and start indexing the block chain.
This is a VERY memory intesive operation and it's tuned by default to run on a n1-highmem-8 GCS instance with 52GB of memory
You can control memory usage by throttling how many blocks it processes at once with the setting `extractor.btc.block_concurrent`
The lower the number, the less memory it should use but the slower it will index. 
Once your blockchain is caught up the memory usage should be much lower as it never will need to process a couple blocks at once.

## Configuration
The configuration can be specified in a number of ways. By default you can create a json file and call it with the -c option
you can also specify environment variables that align with the config file values.

Example:
```json
{
	"logger": {
        "level": "debug"
	}
}
```
Can be set via an environment variable:
```
LOGGER_LEVEL=debug
```

### Options:
| Setting                                           | Description                                                           | Default         |
| ------------------------------------------------- | --------------------------------------------------------------------- | --------------- |
| logger.level                                      | The default logging level                                             | "info"          |
| logger.encoding                                   | Logging format (console or json)                                      | "console"       |
| logger.color                                      | Enable color in console mode                                          | true            |
| logger.disable_caller                             | Hide the caller source file and line number                           | false           |
| logger.disable_stacktrace                         | Hide a stacktrace on debug logs                                       | true            |
| ---                                               | ---                                                                   | ---             |
| pidfile                                           | Write a pidfile (only if specified)                                   | ""              |
| profiler.enabled                                  | Enable the debug pprof interface                                      | "false"         |
| profiler.host                                     | The profiler host address to listen on                                | ""              |
| profiler.port                                     | The profiler port to listen on                                        | "6060"          |
| ---                                               | ---                                                                   | ---             |
| server.host                                       | The host address to listen on (blank=all addresses)                   | ""              |
| server.port                                       | The port number to listen on                                          | 8080            |
| server.tls                                        | Enable https/tls                                                      | false           |
| server.devcert                                    | Generate a development cert                                           | false           |
| server.certfile                                   | The HTTPS/TLS server certificate                                      | "server.crt"    |
| server.keyfile                                    | The HTTPS/TLS server key file                                         | "server.key"    |
| server.log_requests                               | Log API requests                                                      | true            |
| server.profiler_enabled                           | Enable the profiler                                                   | false           |
| server.profiler_path                              | Where should the profiler be available                                | "/debug"        |
| ---                                               | ---                                                                   | ---             |
| server.rest.enums_as_ints                         | gRPC Gateway enums as ints                                            | false           |
| server.rest.emit_defaults                         | gRPC Gateway emit default values                                      | true            |
| server.rest.orig_names                            | gRPC Gateway use original names                                       | true            |
| ---                                               | ---                                                                   | ---             |
| server.default_symbol                             | Select which symbol coin if not specified                             | btc             |
| server.default_count                              | The number of items returned by default from api req                  | 20              |
| server.cache_duration                             | How long should cached items be held                                  | "7s"            |
| ---                                               | ---                                                                   | ---             |
| server.legacy.btc_avg_fee_as_min                  | Return the average fee as a min fee (for fixing transactions)         | true            |
| ---                                               | ---                                                                   | ---             |
| elasticsearch.request_log                         | Log elasticsearch request/response timings                            | false           |
| elasticsearch.debug                               | Enabled debugging elastic request/responses                           | false           |
| elasticsearch.sniff                               | Enable monitoring elastic hosts                                       | true            |
| elasticsearch.healthcheck_timeout                 | Default timeout for elasticsearch health check                        | "10s"           |
| elasticsearch.host                                | Base elasticsearch hostname                                           | "elasticsearch" |
| elasticsearch.port                                | Base elasticsearch port number                                        | "9200"          |
| elasticsearch.retries                             | Connect retries                                                       | 5               |
| elasticsearch.sleep_between_retries               | Sleep between connect retries (seconds)                               | "5s"            |
| elasticsearch.count_max                           | The max number of items that can be returned                          | 10000           |
| elasticsearch.index                               | Elasticsearch base index name                                         | "blocc"         |
| elasticsearch.request_timeout                     | How long the client will wait for a response to reqs                  | "12m"           |
| elasticsearch.bulk_workers                        | Number of bulk workers to start                                       | 2               |
| elasticsearch.throttle_searches                   | Number of simultaneous searches                                       | 60              |
| elasticsearch.bulk_workers                        | Number of bulk workers to start                                       | 5               |
| elasticsearch.bulk_flush_interval                 | How often to flush bulk requests                                      | "5s"            |
| elasticsearch.bulk_stats                          | Periodically show bulk stats                                          | false           |
| elasticsearch.bulk_stats_interval                 | How often to show bulk stats                                          | "60s            |
| elasticsearch.wipe_confirm                        | Wipe elastic database/indexes                                         | false           |
| ---                                               | ---                                                                   | ---             |
| elasticsearch.block.template_file                 | Mapping file for type. Blank=Use Embedded defaults                    | ""              |
| elasticsearch.block.index_replicas                | Type index replicas                                                   | 0               |
| elasticsearch.block.index_shards                  | Type index shards                                                     | 5               |
| elasticsearch.block.refresh_interval              | Type refresh interval                                                 | "30s"           |
| elasticsearch.tx.template_file                    | Mapping file for type. Blank=Use Embedded defaults                    | ""              |
| elasticsearch.tx.index_replicas                   | Type index replicas                                                   | 0               |
| elasticsearch.tx.index_shards                     | Type index shards                                                     | 25              |
| elasticsearch.tx.refresh_interval                 | Type refresh interval                                                 | "30s"           |
| ---                                               | ---                                                                   | ---             |
| redis.host                                        | Host for redis                                                        | "redis"         |
| redis.port                                        | Port for redis                                                        | "6379"          |
| redis.password                                    | Redis password                                                        | "redis"         |
| redis.index                                       | Redis index                                                           | 0               |
| redis.master_name                                 | Redis Master Name (for Sentinal)                                      | ""              |
| ---                                               | ---                                                                   | ---             |
| extractor.btc.host                                | Host for bitcoind node                                                | "bitcoind"      |
| extractor.btc.port                                | Port for bitcoind node                                                | "8333"          |
| extractor.btc.chain                               | Which chain to monitor                                                | "mainnet"       |
| extractor.btc.debug                               | Enable debug messages                                                 | false           |
| ---                                               | ---                                                                   | ---             |
| extractor.btc.block                               | Should we extract blocks to the block store                           | false           |
| extractor.btc.block_concurrent                    | How many concurrent blocks to process                                 | 60              |
| extractor.btc.block_store_raw                     | Should we store the raw block in the block store                      | true            |
| extractor.btc.block_start_id                      | Starting block ID (default Genesis Block)                             | "00...."        |
| extractor.btc.block_start_height                  | Starting block height (-1 = genesis block)                            | -1              |
| extractor.btc.block_headers_request_count         | How many headers are expected on get headers request                  | 2000            |
| extractor.btc.block_request_count                 | How many blocks are expected on get blocks request                    | 500             |
| extractor.btc.block_request_timeout               | How long to wait for completion when parsing chunk of blocks          | "180m"          |
| extractor.btc.block_timeout                       | How long to wait without any blocks when following chain              | "5m"            |
| extractor.btc.block_validation_interval           | How often to validate blocks in the block store                       | "10m"           |
| extractor.btc.block_validation_height_delta       | Assume blocks this far from head are valid if no errors               | 100             |
| extractor.btc.block_validation_height_holdoff     | Hold off this many blocks from chain head in case of forks            | 10              |
| ---                                               | ---                                                                   | ---             |
| extractor.btc.transaction                         | Should we extract incoming transactions into the txpool               | false           |
| extractor.btc.transaction_resolve_previous        | Should we resolve previous outputs                                    | true            |
| extractor.btc.transaction_pool_lifetime           | How long should transactions live in the pool                         | "336h"          |
| extractor.btc.transaction_store_raw               | Should we store raw transactions in the block chain store             | true            |
| ---                                               | ---                                                                   | ---             |
| extractor.btc.bhcache_lifetime                    | How long should headers remain in the cache (0=forever unless purged) | 0               |
| ---                                               | ---                                                                   | ---             |
| extractor.btc.bhtxn_monitor_block_header_lifetime | How long block headers remain in the monitor                          | "240m"          |
| extractor.btc.bhtxn_monitor_transaction_lifetime  | How long should transactions remain in the montior                    | "4m"            |
| extractor.btc.bhtxn_monitor_block_wait_timeout    | How long to wait for a previous block when following chain            | "240m"          |
| ---                                               | ---                                                                   | ---             |


## TLS/HTTPS
You can enable https by setting the config option server.tls = true and pointing it to your keyfile and certfile.
To create a self-signed cert: `openssl req -new -newkey rsa:2048 -days 3650 -nodes -x509 -keyout server.key -out server.crt`

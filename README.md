# BLOCC

This is a bitcoin node client that monitors blocks and transactions as well as logs stats and metrics with a GRPC/REST API

## Compiling
This is designed as a go module aware program and thus requires go 1.11 or better
You can clone it anywhere, just run `make` inside the cloned directory to build

## Requirements
The system requires Redis to function. It's used as both a mempool and Pub/Sub bus for monitoring realtime transactions

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
| Setting                              | Description                                            | Default         |
|--------------------------------------|--------------------------------------------------------|-----------------|
| logger.level                         | The default logging level                              | "info"          |
| logger.encoding                      | Logging format (console or json)                       | "console"       |
| logger.color                         | Enable color in console mode                           | true            |
| logger.disable_caller                | Hide the caller source file and line number            | false           |
| logger.disable_stacktrace            | Hide a stacktrace on debug logs                        | true            |
| ---                                  | ---                                                    | ---             |
| pidfile                              | Write a pidfile (only if specified)                    | ""              |
| profiler.enabled                     | Enable the debug pprof interface                       | "false"         |
| profiler.host                        | The profiler host address to listen on                 | ""              |
| profiler.port                        | The profiler port to listen on                         | "6060"          |
| ---                                  | ---                                                    | ---             |
| server.host                          | The host address to listen on (blank=all addresses)    | ""              |
| server.port                          | The port number to listen on                           | 8900            |
| server.tls                           | Enable https/tls                                       | false           |
| server.devcert                       | Generate a development cert                            | false           |
| server.certfile                      | The HTTPS/TLS server certificate                       | "server.crt"    |
| server.keyfile                       | The HTTPS/TLS server key file                          | "server.key"    |
| server.log_requests                  | Log API requests                                       | true            |
| server.profiler_enabled              | Enable the profiler                                    | false           |
| server.profiler_path                 | Where should the profiler be available                 | "/debug"        |
| ---                                  | ---                                                    | ---             |
| server.rest.enums_as_ints            | gRPC Gateway enums as ints                             | false           |
| server.rest.emit_defaults            | gRPC Gateway emit default values                       | true            |
| server.rest.orig_names               | gRPC Gateway use original names                        | true            |
| ---                                  | ---                                                    | ---             |
| server.default_symbol                | Select which symbol coin if not specified              | btc             |
| ---                                  | ---                                                    | ---             |
| elasticsearch.mapping_file           | Contains elastic mapping (blank=use embedded)          | ""              |
| elasticsearch.request_log            | Log elasticsearch request/response timings             | false           |
| elasticsearch.debug                  | Enabled debugging elastic request/responses            | false           |
| elasticsearch.sniff                  | Enable monitoring elastic hosts                        | true            |
| elasticsearch.host                   | Base elasticsearch hostname                            | "elasticsearch" |
| elasticsearch.port                   | Base elasticsearch port number                         | "9200"          |
| elasticsearch.retries                | Connect retries                                        | 5               |
| elasticsearch.sleep_between_retries  | Sleep between connect retries (seconds)                | "5s"            |
| elasticsearch.index                  | Elasticsearch base index name                          | "blocc"         |
| elasticsearch.index_replicas         | Default elastic index replicas                         | 0               |
| elasticsearch.index_shards           | Default elastic index shards                           | 5               |
| elasticsearch.refresh_interval       | How often to refresh indexes                           | "30s"           |
| elasticsearch.wipe_confirm           | Wipe elastic database/indexes                          | false           |
| ---                                  | ---                                                    | ---             |
| redis.host                           | Host for redis                                         | "redis"         |
| redis.port                           | Port for redis                                         | "6379"          |
| redis.password                       | Redis password                                         | "redis"         |
| redis.index                          | Redis index                                            | 0               |
| ---                                  | ---                                                    | ---             |
| extractor.btc.host                   | Host for bitcoind node                                 | "bitcoind"      |
| extractor.btc.port                   | Port for bitcoind node                                 | "8333"          |
| extractor.btc.chain                  | Which chain to monitor                                 | "mainnet"       |
| extractor.btc.debug_messages         | Raw message debugging                                  | false           |
| extractor.btc.start_hash             | Starting hash for extractor (blank=genesis)            | ""              |
| extractor.btc.start_height           | The starting height for the specified block            | 0               |
| extractor.btc.throttle_blocks        | Number of blocks to process simultaneously             | 30              |
| extractor.btc.throttle_transactions  | Number of tx to process simultaneously                 | 100             |
| extractor.btc.transaction_lifetime   | How long to keep transaction in mempool (336h=2 weeks) | "336h"          |
| extractor.btc.store_raw_blocks       | Store raw data in blockstore                           | false           |
| extractor.btc.store_raw_transactions | Store raw transactions in blockstore                   | false           |


## Data Storage
Transactions are stored in a mempool in redis as well as streamed via redis pubsub


## TLS/HTTPS
You can enable https by setting the config option server.tls = true and pointing it to your keyfile and certfile.
To create a self-signed cert: `openssl req -new -newkey rsa:2048 -days 3650 -nodes -x509 -keyout server.key -out server.crt`

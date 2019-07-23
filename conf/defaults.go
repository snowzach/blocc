package conf

import (
	config "github.com/spf13/viper"
)

func init() {

	// Logger Defaults
	config.SetDefault("logger.level", "info")
	config.SetDefault("logger.encoding", "console")
	config.SetDefault("logger.color", true)
	config.SetDefault("logger.dev_mode", true)
	config.SetDefault("logger.disable_caller", false)
	config.SetDefault("logger.disable_stacktrace", true)

	// Pidfile
	config.SetDefault("pidfile", "")

	// Profiler config
	config.SetDefault("profiler.enabled", false)
	config.SetDefault("profiler.host", "")
	config.SetDefault("profiler.port", "6060")

	// Server Configuration
	config.SetDefault("server.host", "")
	config.SetDefault("server.port", "8080")
	config.SetDefault("server.tls", false)
	config.SetDefault("server.devcert", false)
	config.SetDefault("server.certfile", "server.crt")
	config.SetDefault("server.keyfile", "server.key")
	config.SetDefault("server.log_requests", true)
	config.SetDefault("server.profiler_enabled", false)
	config.SetDefault("server.profiler_path", "/debug")
	// GRPC JSON Marshaler Options
	config.SetDefault("server.rest.enums_as_ints", false)
	config.SetDefault("server.rest.emit_defaults", true)
	config.SetDefault("server.rest.orig_names", true)
	// Other options
	config.SetDefault("server.default_symbol", "btc")
	config.SetDefault("server.default_count", 20)
	config.SetDefault("server.cache_duration", "7s")
	// Legacy API Options
	config.SetDefault("server.legacy.btc_avg_fee_as_min", true)
	config.SetDefault("server.legacy.btc_min_fee_max", 100)
	config.SetDefault("server.legacy.btc_avg_fee_max", 500)
	config.SetDefault("server.legacy.btc_max_fee_max", -1)
	config.SetDefault("server.legacy.btc_use_p10_fee", false)
	config.SetDefault("server.legacy.btc_fee_testing", false)

	// Set Defaults - Elasticsearch
	config.SetDefault("elasticsearch.request_log", false)
	config.SetDefault("elasticsearch.debug", false)
	config.SetDefault("elasticsearch.sniff", false)
	config.SetDefault("elasticsearch.healthcheck_timeout", "0s")
	config.SetDefault("elasticsearch.host", "") // Override back to host when ready to use
	config.SetDefault("elasticsearch.port", "9200")
	config.SetDefault("elasticsearch.retries", 5)
	config.SetDefault("elasticsearch.version", 6)
	config.SetDefault("elasticsearch.sleep_between_retries", "5s")
	config.SetDefault("elasticsearch.count_max", 10000)
	config.SetDefault("elasticsearch.index", "blocc")
	config.SetDefault("elasticsearch.request_timeout", "12m")
	config.SetDefault("elasticsearch.throttle_searches", 60)
	config.SetDefault("elasticsearch.bulk_workers", 5)
	config.SetDefault("elasticsearch.bulk_flush_interval", "5s")
	config.SetDefault("elasticsearch.bulk_stats", false)
	config.SetDefault("elasticsearch.bulk_stats_interval", "60s")
	config.SetDefault("elasticsearch.wipe_confirm", false)

	config.SetDefault("elasticsearch.block.template_file", "") // Defaults to loading embedded template-block.json if not specified
	config.SetDefault("elasticsearch.block.index_shards", 10)
	config.SetDefault("elasticsearch.block.index_replicas", 0)
	config.SetDefault("elasticsearch.block.refresh_interval", "15s")
	config.SetDefault("elasticsearch.tx.template_file", "") // Defaults to loading embedded template-tx.json if not specified
	config.SetDefault("elasticsearch.tx.index_shards", 24)
	config.SetDefault("elasticsearch.tx.index_replicas", 0)
	config.SetDefault("elasticsearch.tx.refresh_interval", "15s")

	config.SetDefault("elasticsearch.fix_aggregation_size", 5000)

	// Redis Settings
	config.SetDefault("redis.host", "redis")
	config.SetDefault("redis.port", "6379")
	config.SetDefault("redis.master_name", "")
	config.SetDefault("redis.password", "")
	config.SetDefault("redis.index", 0)

	// BTC extractor settings
	config.SetDefault("extractor.btc.host", "bitcoind")
	config.SetDefault("extractor.btc.port", 8333)
	config.SetDefault("extractor.btc.chain", "mainnet")
	config.SetDefault("extractor.btc.debug", false)

	config.SetDefault("extractor.btc.block", false)
	config.SetDefault("extractor.btc.block_concurrent", 30)
	config.SetDefault("extractor.btc.block_store_raw", true)
	config.SetDefault("extractor.btc.block_start_id", "0000000000000000000000000000000000000000000000000000000000000000")
	config.SetDefault("extractor.btc.block_start_height", -1)
	config.SetDefault("extractor.btc.block_headers_request_count", 2000)
	config.SetDefault("extractor.btc.block_request_count", 500)
	config.SetDefault("extractor.btc.block_request_timeout", "180m")
	config.SetDefault("extractor.btc.block_timeout", "5m")
	config.SetDefault("extractor.btc.block_validation_interval", "10m")
	config.SetDefault("extractor.btc.block_validation_height_delta", 100)
	config.SetDefault("extractor.btc.block_validation_height_holdoff", 10)

	config.SetDefault("extractor.btc.transaction", false)
	config.SetDefault("extractor.btc.transaction_concurrent", 1000)
	config.SetDefault("extractor.btc.transaction_resolve_previous", true)
	config.SetDefault("extractor.btc.transaction_pool_lifetime", "336h") // 14 days
	config.SetDefault("extractor.btc.transaction_store_raw", true)

	config.SetDefault("extractor.btc.bhcache_lifetime", "0")

	config.SetDefault("extractor.btc.bhtxn_monitor_block_header_lifetime", "240m")
	config.SetDefault("extractor.btc.bhtxn_monitor_transaction_lifetime", "4m")
	config.SetDefault("extractor.btc.bhtxn_monitor_block_wait_timeout", "60m")

}

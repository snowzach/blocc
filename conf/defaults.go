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
	config.SetDefault("server.cache_duration", "7s")

	// Database Settings
	config.SetDefault("storage.type", "postgres")
	config.SetDefault("storage.username", "postgres")
	config.SetDefault("storage.password", "password")
	config.SetDefault("storage.host", "postgres")
	config.SetDefault("storage.port", 5432)
	config.SetDefault("storage.database", "gogrpcapi")
	config.SetDefault("storage.sslmode", "disable")
	config.SetDefault("storage.retries", 5)
	config.SetDefault("storage.sleep_between_retries", "7s")
	config.SetDefault("storage.max_connections", 80)
	config.SetDefault("storage.wipe_confirm", false)

	// Set Defaults - Elasticsearch
	config.SetDefault("elasticsearch.request_log", false)
	config.SetDefault("elasticsearch.debug", false)
	config.SetDefault("elasticsearch.sniff", true)
	config.SetDefault("elasticsearch.host", "") // Override back to host when ready to use
	config.SetDefault("elasticsearch.port", "9200")
	config.SetDefault("elasticsearch.retries", 5)
	config.SetDefault("elasticsearch.sleep_between_retries", "5s")
	config.SetDefault("elasticsearch.index", "blocc")
	config.SetDefault("elasticsearch.bulk_workers", 2)
	config.SetDefault("elasticsearch.bulk_stats", false)
	config.SetDefault("elasticsearch.bulk_stats_interval", "60s")
	config.SetDefault("elasticsearch.wipe_confirm", false)

	config.SetDefault("elasticsearch.block.template_file", "") // Defaults to loading embedded template-block.json if not specified
	config.SetDefault("elasticsearch.block.index_shards", 5)
	config.SetDefault("elasticsearch.block.index_replicas", 0)
	config.SetDefault("elasticsearch.block.refresh_interval", "30s")
	config.SetDefault("elasticsearch.tx.template_file", "") // Defaults to loading embedded template-tx.json if not specified
	config.SetDefault("elasticsearch.tx.index_shards", 25)
	config.SetDefault("elasticsearch.tx.index_replicas", 0)
	config.SetDefault("elasticsearch.tx.refresh_interval", "30s")
	config.SetDefault("elasticsearch.out.template_file", "") // Defaults to loading embedded template-tx.json if not specified
	config.SetDefault("elasticsearch.out.index_shards", 20)
	config.SetDefault("elasticsearch.out.index_replicas", 0)
	config.SetDefault("elasticsearch.out.refresh_interval", "30s")

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
	config.SetDefault("extractor.btc.debug_messages", false)
	config.SetDefault("extractor.btc.blocks", false)
	config.SetDefault("extractor.btc.transactions", false)
	config.SetDefault("extractor.btc.start_block_id", "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f")
	config.SetDefault("extractor.btc.start_block_height", 0)
	config.SetDefault("extractor.btc.throttle_blocks", 500)
	config.SetDefault("extractor.btc.throttle_transactions", 700)
	config.SetDefault("extractor.btc.transaction_lifetime", "336h") // 14 days
	config.SetDefault("extractor.btc.store_raw_blocks", false)
	config.SetDefault("extractor.btc.store_raw_transactions", false)

}

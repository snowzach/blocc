package esearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/olivere/elastic"
	config "github.com/spf13/viper"
	"go.uber.org/zap"

	"git.coinninja.net/backend/blocc/conf"
)

type esearch struct {
	logger *zap.SugaredLogger

	url    string
	client *elastic.Client
	bulk   *elastic.BulkProcessor
	ctx    context.Context

	index string
}

// NewES creates a connection to Elasticsearch to interact with
func New() (*esearch, error) {

	e := &esearch{
		logger: zap.S().With("package", "blockstore.esearch"),
		ctx:    context.Background(),
		index:  config.GetString("elasticsearch.index"),
	}

	if config.GetString("elasticsearch.host") != "" && config.GetString("elasticsearch.port") != "" {
		e.url = fmt.Sprintf("http://%s:%s", config.GetString("elasticsearch.host"), config.GetString("elasticsearch.port"))
	} else {
		return nil, fmt.Errorf("Unable to determine address of Elasticsearch")
	}

	// Setup Elastic Options
	esOptions := []elastic.ClientOptionFunc{
		elastic.SetErrorLog(&errorLogger{logger: e.logger}),
		elastic.SetURL(e.url),
	}
	if config.GetBool("elasticsearch.request_log") {
		esOptions = append(esOptions, elastic.SetInfoLog(&infoLogger{logger: e.logger}))
	}
	if config.GetBool("elasticsearch.debug") {
		esOptions = append(esOptions, elastic.SetTraceLog(&traceLogger{logger: e.logger}))
	}
	if config.GetBool("elasticsearch.sniff") {
		esOptions = append(esOptions,
			elastic.SetSniff(true),
			elastic.SetSnifferCallback(func(node *elastic.NodesInfoNode) bool {
				// If this node has only one role (master) don't use it for requests
				if len(node.Roles) == 1 && node.Roles[0] == "master" {
					return false
				}
				return true
			}),
		)

	} else {
		esOptions = append(esOptions, elastic.SetSniff(false))
	}

	var err error

	for retries := config.GetInt("elasticsearch.retries"); retries > 0 && !conf.StopFlag; retries-- {
		e.client, err = elastic.NewClient(esOptions...)
		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				e.logger.Warnw("Connection to elasticsearch timed out. Sleeping and retry.",
					"host", config.GetString("elasticsearch.host"),
					"port", config.GetString("elasticsearch.post"),
				)
				time.Sleep(config.GetDuration("elasticsearch.sleep_between_retries"))
				continue
			} else {
				return nil, fmt.Errorf("Could not connect to elasticsearch: %s", err)
			}
		}
		break
	}

	// Aborted before connected
	if conf.StopFlag {
		return nil, fmt.Errorf("Connection to elasticsearch aborted")
	}

	// Unable to connect to elastic
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to elasticsearch: %v", err)
	}

	// Remove indexes if we are set to wipe...
	if config.GetBool("elasticsearch.wipe_confirm") {
		e.logger.Warnw("We are about to wipe existing indexes. You have 4 seconds to press Ctrl-C",
			"index", e.index+"*",
		)
		time.Sleep(4 * time.Second)

		// If the stop flag hasn't been set, we're ready to go
		if conf.StopFlag {
			return nil, fmt.Errorf("Wipe aborted.")
		}

		// Handle the wipe
		err = e.wipe()
		if err != nil {
			return nil, fmt.Errorf("Could not wipe: %v", err)
		}

		err = e.ApplyIndexTemplate()
		if err != nil {
			return nil, fmt.Errorf("Could not ApplyIndexTemplate: %v", err)
		}

	}

	// Start up the bulk processor
	e.bulk, err = e.client.BulkProcessor().
		Name("bulk").
		FlushInterval(5 * time.Second).
		Do(e.ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not start BulkProcessor: %s", err)
	}

	return e, nil
}

// This applies the index template but it should only be done once so this will be called by the master upon startup
func (e *esearch) ApplyIndexTemplate() error {

	// Load the index mapping
	var mapping = make(map[string]interface{})

	// Get the default mapping from the mapping file
	rawMapping, err := ioutil.ReadFile(config.GetString("elasticsearch.mapping_file"))
	if err != nil {
		return fmt.Errorf("Could not retrieve mapping from %s error: %s", config.GetString("elasticsearch.mapping_file"), err)
	}

	// Copy the mapping structure to a map we can modify
	err = json.Unmarshal(rawMapping, &mapping)
	if err != nil {
		return fmt.Errorf("Could not parse mapping JSON from %s error %s", config.GetString("elasticsearch.mapping_file"), err)
	}

	// Update the default mapping settings based on passed in options
	settings := mapping["settings"].(map[string]interface{})
	settings["number_of_shards"] = config.GetInt("elasticsearch.index_shards")
	settings["number_of_replicas"] = config.GetInt("elasticsearch.index_replicas")
	settings["refresh_interval"] = config.GetString("elasticsearch.refresh_interval")

	// Create an index template
	mapping["index_patterns"] = []string{e.index + "-*"}

	// Remove the existing index template (if exists)
	deleteTemplateRepsonse, err := e.client.IndexDeleteTemplate(e.index).Do(e.ctx)
	if elastic.IsNotFound(err) {
		// We're good
	} else if err != nil {
		return fmt.Errorf("Failed to remove Elasticsearch template '%s' error: %v", e.index+"*", err)
	} else if !deleteTemplateRepsonse.Acknowledged {
		return fmt.Errorf("Failed to receive Elasticsearch delete template response")
	}

	// Create the new index template
	createTemplate, err := e.client.IndexPutTemplate(e.index).BodyJson(mapping).Do(e.ctx)
	if err != nil {
		return fmt.Errorf("Failed to create Elasticsearch template: %v", err)
	}
	if !createTemplate.Acknowledged {
		return fmt.Errorf("Failed to receive acknowledgement that Elasticsearch template was created")
	}

	return nil

}

// Force a refresh of an index
func (e *esearch) Refresh() error {
	_, err := e.client.Refresh(e.index + "*").
		Do(e.ctx)
	return err
}

// Wipe the current elastic
func (e *esearch) wipe() error {

	// Delete indexes
	deleteIndexResp, err := e.client.DeleteIndex(e.index + "-*").Do(e.ctx)
	if elastic.IsNotFound(err) {
		// We're good
	} else if elastic.IsStatusCode(err, 400) {
		// This means that it's an alias and not an index, also okay
	} else if err != nil {
		return fmt.Errorf("Failed to remove Elasticsearch base index '%s' error: %v", e.index+"*", err)
	} else if !deleteIndexResp.Acknowledged {
		return fmt.Errorf("Failed to receive Elasticsearch delete indexes response")
	}

	// Delete Aliases
	deleteAliasesRepsonse, err := e.client.Alias().Remove("*", e.index+"-*").Do(e.ctx)
	if elastic.IsNotFound(err) {
		// We're good
	} else if err != nil {
		return fmt.Errorf("Failed to remove Elasticsearch partition aliases '%s' error: %v", e.index+"*", err)
	} else if !deleteAliasesRepsonse.Acknowledged {
		return fmt.Errorf("Failed to receive Elasticsearch delete partition aliases response")
	}

	return nil
}

func parseElasticError(err error) error {
	if elasticError, ok := err.(*elastic.Error); ok && elasticError != nil {
		errString := elasticError.Details.Type + "/" + elasticError.Details.Reason
		for _, subErr := range elasticError.Details.RootCause {
			if elasticError.Details.Type != subErr.Type {
				errString += ":" + subErr.Type + "/" + subErr.Reason
			}
		}
		return errors.New(errString)
	}
	return err
}

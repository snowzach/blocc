# Elasticsearch for BLOCC
This describes how to setup elasticsearch for blocc, the block chain indexer service

## NOTE WARNING NOTE WARNING
If you configure a cluster from scratch and start up the block chain indexer it will create indexes from scratch. The indexes will have
replication turned off while it builds the initial block chain. This is for speed as replicas drastically slow down indexing. YOU MUST RE-ENALBE REPLICAS otherwise if you lose a node your index will hang or possibly be corrupt. (It should come back up when it remounts the persistent disk but it's much better to have a replica.) See below how to adjust replicas.

The indexes will also have the refresh interval somewhat high.. Around 30s. This means that any changes to the index will not be queriable for 30s. This also speeds up initial indexing. See below on how to decrease this.

# Overview
At a high level, elasticsearch is setup as 3 deployments to kubernetes
* Elasticsearch Master - These are the coordination nodes that monitor and maintain cluster state
* Elasticsearch Data - These handle all data and coordination
* Kibana - This is the GUI/Monitoring Tool for elasticsearch

Terraform will deploy the kuberenetes configuration and manage the pods. Elasticsearch is deployed and maintained using helm charts

# Kubernetes (via terraform)
The terraform configuration as recommended is contained in this repo, if you deploy as it is, it will
setup 3 sets of node pools:
* np-elasticsearch-master - 3 n1-standard-1 nodes for running elasticsearch master 
* np-elasticsearch-pd - A number of nodes (default 12) to run data operations using persistent disks
* np-kibana - A single node to host kibana
There is also another set of nodes called `np-elasticsearch-ssd` that use locally attached SSD. This could be used for building the initial block chain as they are very fast. By default the node count will be set to zero. If you wish to use these, set the node count to whatever you wish. They will use 2 SSD's at 375GB each. The full block chain is about 1.3TB and there is one replica so approx 2.6TB of disk as of March 2019. I would recommend that in each set of data nodes (pd and ssd) you have at least enough disk space to store the entire block chain (so 2.6TB) By default the persistent disk nodes are configured to have 375GB each.

If you wish to enable these, it's designed to use 6 by default so make sure there are 6 nodes in your node pool for `elasticsearch-ssd`

# Deploy Elasticsearch (via helm charts)
Once the node pools are deployed, you can apply the helm charts to bring up elasticsearch
From the `es-helm-charts/elasticsearch` directory
1. Create the master nodes
```
helm create -n elasticsearch-master -f master.yaml .
```
2. Create the data nodes using persistent disk
```
helm create -n elasticsearch-pd -f data-pd.yaml .
```
3. Deploy Kibana from `es-helm-charts/kibana`
```
helm create -n kibana -f blocc.yaml .
```
And if you decide you want to use the SSD nodes, you can deploy those as well
4. Deploy SSD data nodes from `es-helm-charts/elasticsearch`
```
helm create -n elasticsearch-ssdd -f data-ssd.yaml .
```

It should create a load balancer for both elasticsearch and kibana that you can reach from the Office or VPN (see terraform firewall config)
* Kibana is the service called `blocc-kibana-lb` at http://<EXTERNAL-IP>:5601
* Elasticsearch is the service called `blocc-esdata-lb` at http://<EXTERNAL-IP>:9200

# Cluster initial configuration
Once the cluster is up, make sure you can reach Kibana. Go to the dev tools option on the left menu and apply some initial configuration

This will configure the cluster to be a little faster at recovering if a node goes down. Using persistent disks should ensure there's not a lot of issues there however, if you wish to use both SSD and Persistent disk, the awareness.attributes will ensure that shards are balanced accross multiple node types. If you run SSD and PD, it will result in one copy of the shard being on each one.
```
PUT /_cluster/settings
{
  "persistent" : {
    "cluster" : {
      "routing" : {
        "allocation" : {
          "cluster_concurrent_rebalance" : "4",
          "node_concurrent_recoveries" : "4",
          "awareness.attributes": "node_group"
        }
      }
    },
    "indices" : {
      "recovery" : {
        "max_bytes_per_sec" : "100mb"
      }
    }
  }
}
```

## Backup configuration - Google Cloud Storage
- Setup the Client - Create a service account and then get the crededtials file in Google Console based on instruction here
    https://www.elastic.co/guide/en/elasticsearch/plugins/master/repository-gcs-usage.html

- Create an elasticsearch.keystore file using your service_account json file by running an elastic container anywhere in docker
```
docker exec -it -v /path/to/service_accoun.json:/tmp/service_account.json docker.elastic.co/elasticsearch/elasticsearch:6.6.1 bash
cd /usr/share/elasticsearch/config
elasticsearch-keystore create
elasticsearch-keystore add-file gcs.client.default.credentials_file /tmp/service_account.json
cat /usr/share/elasticsearch/config/elasticsearch.keystore | base64 -w0
```

- Copy that big mess of base64 goodness to a secrets file `elasticsearch-secrets.yml`
```
apiVersion: v1
kind: Secret
metadata:
  name: elasticsearch-secrets
type: Opaque
data:
  elasticsearch.keystore: <base64 goodness>
```

- Add that secret to your kube cluster. The helm-charts will leverage that secret and our image to load the keystore file
```
kubectl create -f ./elasticsearch-secrets.yml
```

NOTE: In this case we added the credentials file with the keystore key `gcs.client.default.credentials_file`
This indicates that the client is named `default`. You can add additional clients by adding service accounts with the following keystore key `gcs.client.<client>.credentials_file` so if you wanted one in prod and another in dev, create one for each

### Snapshots
-------------------
General information here
https://www.elastic.co/guide/en/elasticsearch/reference/current/modules-snapshots.html

Create a new backup repository in the elasticsearch cluster
```
PUT _snapshot/blocc
{
  "blocc" : {
    "type" : "gcs",
    "settings" : {
      "bucket" : "cn-blockchain-es",
      "client" : "default",
      "base_path" : "blocc-dev"
    }
  }
}
```

#### Perform a snapshot - this will auto name the snapshot with the date
```
PUT /_snapshot/blocc/%3Csnapshot-%7Bnow%2Fd%7D%3E
{
  "indices": "blocc-*"
}
```

#### Stop a snapshot
```
DELETE /_snapshot/blocc/snapshot-2018.05.11
```

#### Show snapshots in a repository
```
GET /_snapshot/blocc/_all
```

#### Restore a snapshot
You can restore a snapshot with replicas set to 0, this will increase the speed and then activate replicas once it's completed
```
POST /_snapshot/blocc/snapshot-2018.05.11/_restore
{
  "indices": "blocc-*",
  "index_settings": {
    "index.number_of_replicas": 0
  }
}
```

# Update replicas
Once you have setup your cluster you can set the number of replicas an index has. Essentially replicas work just like raid. You can
lose the same number of nodes as you have replicas.

The refresh interval is how often the changes 

The indexes are called `blocc-block-btc` and `blocc-tx-btx`. You can set the replicas and refresh interval from kibana in the dev console with:
```
PUT /blocc-*/_settings
{
    "index" : {
        "number_of_replicas" : 1,
        "refresh_interval" : "1s"
    }
}
```
This is the recommended running setting which will ensure that the cluster is fault tolerant and changes are queriable in a timely manner


# Troubleshooting
-----------------
1. DO NOT destroy more than one node at a time and ensure index status is green before you start. You must have at least one replica set
3. Look at shard health
    GET /_cat/shards
4. If it all goes to hell, explain the shard health
    GET /_cluster/allocation/explain
5. Restarted nodes and shards are still uphappy... try to retry reroute. This usually fixes a cluster stuck in red.
    POST /_cluster/reroute?retry_failed

You MAY need to use the load balancer endpoint and postman if kibana is unreachable. This is because kibana requires the elasticsearch service and if for whatever reason the cluster goes unavailable, you may not be able to reach it.

## Index is Read-Only
If you see that one of the tools is complaining about being readonly, there was something wrong with the system.
Generally this happens because you run out of disk. Once you have resolved any issue and you are sure the hardware is stable
you can restore the system with this: (This will reset EVERY index on elasticsearch)
```
PUT */_settings
{
  "index": {
    "blocks": {
      "read_only_allow_delete": "false"
    }
  }
}
```

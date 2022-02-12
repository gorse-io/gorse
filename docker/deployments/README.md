# Run Gorse on Kubernetes

## Prerequisite

Gorse depends on following software:

- *Redis* is used to store caches.
- One of *MySQL/PostgresSQL/ClickHouse/MongoDB* is used to store data.

The minimal versions of dependent software are as follows:

| Software    | Minimal Version |
|-------------|-----------------|
| Redis       | 5.0             |
| MySQL       | 5.7             |
| PostgresSQL | 10.0            |
| ClickHouse  | 21.10           |
| MongoDB     | 4.0             |

### Deploy a TiDB cluster

Since TiDB is a MySQL-compatible distributed database, a TiDB cluster is used to store data for Gorse.

1. Install TiDB operator.

```bash
# Apply CRDs
kubectl create -f https://raw.githubusercontent.com/pingcap/tidb-operator/master/manifests/crd.yaml

# Add helm charts repo
helm repo add pingcap https://charts.pingcap.org/

# Install operator
helm install tidb-operator pingcap/tidb-operator --version v1.3.0 \
  --namespace tidb-operator --create-namespace
```

2. Launch a TiDB cluster.

```bash
helm install tidb-cluster pingcap/tidb-cluster \
  --set schedulerName=default-scheduler \
  --set pd.storageClassName=standard \
  --set tikv.storageClassName=standard \
  --set pd.replicas=3 \
  --set tikv.replicas=3 \
  --set tidb.replicas=3 \
  --version v1.3.0
```

3. Forward a local port to the TiDB port.

```bash
kubectl port-forward svc/tidb-cluster-tidb 4000:4000
```

4. In another terminal window, connect the TiDB server with a MySQL client.

```bash
mysql -h 127.0.0.1 -P 4000 -u root
```

5. Create a database.

```sql
CREATE DATABASE gorse;
```

### Deploy a Redis cluster

A Redis cluster is used for caching.

1. Install Redis operator.

```bash
# Add helm charts repo
helm repo add ot-helm https://ot-container-kit.github.io/helm-charts/

# Install operator
helm install redis-operator ot-helm/redis-operator --version v0.8.0 \
    --namespace redis-operator --create-namespace
```

2. Launch a Redis cluster.

```bash
helm install redis-cluster ot-helm/redis-cluster --version v0.8.0 \
  --set redisCluster.clusterSize=3
```

## Quick Start

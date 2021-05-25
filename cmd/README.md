# Run Gorse Manually

Binary distributions have been provided for 64-bit Windows/Linux/Mac OS on the [release](https://github.com/zhenghaoz/gorse/releases) page. Due to the demand on large memories, 64-bit machines are highly recommended to deploy Gorse.

## Prerequisite

Gorse depends on following software:

- *Redis* is used to store caches.
- *MySQL/MongoDB* is used to store data.

## Run Gorse

- Install Gorse

**Option 1:** Download binary distributions (Linux)

```bash
wget https://github.com/zhenghaoz/gorse/releases/latest/download/gorse_linux_amd64.zip
unzip gorse_linux_amd64.zip
```

For Windows and MacOS (Intel Chip or Apple Silicon), download [gorse_windows_amd64.zip](https://github.com/zhenghaoz/gorse/releases/latest/download/gorse_windows_amd64.zip), [gorse_darwin_amd64.zip](https://github.com/zhenghaoz/gorse/releases/latest/download/gorse_darwin_amd64.zip) or [gorse_darwin_arm64.zip](https://github.com/zhenghaoz/gorse/releases/latest/download/gorse_darwin_arm64.zip) respectively.

**Option 2:** Build executable files via `go get`

```bash
go get github.com/zhenghaoz/gorse/...
```

Built binaries locate at `$(go env GOPATH)/bin`.

- Configuration

Create a configuration file [config.toml](https://github.com/zhenghaoz/gorse/blob/master/config/config.toml.template) in the working directory. Set `cache_store` and `data_store` in the configuration file [config.toml](https://github.com/zhenghaoz/gorse/blob/master/config/config.toml.template). 

```toml
# This section declares settings for the database.
[database]
# database for caching (support Redis only)
cache_store = "redis://localhost:6379"
# database for persist data (support MySQL/MongoDB)
data_store = "mysql://root@tcp(localhost:3306)/gorse?parseTime=true"
```

- Start the master node

```bash
./gorse-master -c config.toml
```

`-c` specify the path of the configuration file.

- Start the server node and worker node

```bash
./gorse-server --master-host 127.0.0.1 --master-port 8086 \
    --http-host 127.0.0.1 --http-port 8087
```

`--master-host` and `--master-port` are the RPC host and port of the master node. `--http-host` and `--http-port` are the HTTP host and port for RESTful APIs and metrics reporting of this server node.

```bash
./gorse-worker --master-host 127.0.0.1 --master-port 8086 \
    --http-host 127.0.0.1 --http-port 8089 -j 4
```

`--master-host` and `--master-port` are the RPC host and port of the master node. `--http-host` and `--http-port` are the HTTP host and port for metrics reporting of this worker node. `-j` is the number of working threads.


- Download the SQL file [github.sql](https://cdn.gorse.io/example/github.sql) and import to the MySQL instance.

```bash
mysql -h 127.0.0.1 -u root -proot_pass gorse < github.sql
```


- Play with Gorse:

| Entry | Link |
| --- | --- |
| Master Dashboard | http://127.0.0.1:8088/ |
| Server RESTful API | http://127.0.0.1:8087/apidocs |
| Server Prometheus Metrics | http://127.0.0.1:8087/metrics |
| Worker Prometheus Metrics | http://127.0.0.1:8089/metrics |


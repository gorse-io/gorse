# Run Gorse on Docker Compose

The best practice to manage Gorse nodes is using orchestration tools such as Docker Compose, etc.. There are Docker images of the master node, the server node and the worker node.

| Docker Image         | Image Size |
| ------------ | -------- |
| gorse-master | [![](https://img.shields.io/docker/image-size/zhenghaoz/gorse-master)](https://hub.docker.com/repository/docker/zhenghaoz/gorse-master) |
| gorse-server | [![](https://img.shields.io/docker/image-size/zhenghaoz/gorse-server)](https://hub.docker.com/repository/docker/zhenghaoz/gorse-server) |
| gorse-worker | [![](https://img.shields.io/docker/image-size/zhenghaoz/gorse-worker)](https://hub.docker.com/repository/docker/zhenghaoz/gorse-worker) |

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

## Quick Start

There is an example [docker-compose.yml](https://github.com/zhenghaoz/gorse/blob/master/docker/docker-compose.yml) consists of a master node, a server node and a worker node, a Redis instance, and a MySQL instance.

- Create a configuration file [config.toml](https://github.com/zhenghaoz/gorse/blob/master/docker/config.toml) (Docker Compose version) in the working directory.
- Setup the Gorse cluster using Docker Compose.

```bash
docker-compose up -d
```

- Download the SQL file [github.sql](https://cdn.gorse.io/example/github.sql) and import to the MySQL instance.

```
mysql -h 127.0.0.1 -u gorse -pgorse_pass gorse < github.sql
```

- Restart the master node to apply imported data.

```bash
docker-compose restart
```

- Play with Gorse: 

| Entry | Link |
| --- | --- |
| Master Dashboard | http://127.0.0.1:8088/ |
| Server RESTful API | http://127.0.0.1:8087/apidocs |
| Server Prometheus Metrics | http://127.0.0.1:8087/metrics |
| Worker Prometheus Metrics | http://127.0.0.1:8089/metrics |

# Contribution Guide

Welcome and thank you for considering contributing to Gorse!

Reading and following these guidelines will help us make the contribution process easy and effective for everyone involved. It also communicates that you agree to respect the time of the developers managing and developing these open source projects. In return, we will reciprocate that respect by addressing your issue, assessing changes, and helping you finalize your pull requests.

* [Getting Started](#getting-started)
  * [Setup Develop Environment](#setup-develop-environment)
  * [Option 1: Run an All-in-one Node](#option-1-run-an-all-in-one-node)
  * [Option 2: Run Nodes](#option-2-run-nodes)
  * [Run Unit Tests](#run-unit-tests)
* [Your First Contribution](#your-first-contribution)
  * [Contribution Workflow](#contribution-workflow)
* [Getting Help](#getting-help)

## Getting Started

### Setup Develop Environment

These following installations are required:

- **Go** (>= 1.18): Since Go features from 1.18 are used in Gorse, the version of the compiler must be greater than 1.18. GoLand or Visual Studio Code is highly recommended as the IDE to develop Gorse.

- **Docker Compose**: Multiple databases are required for unit tests. It's convenient to manage databases on Docker Compose.

```bash
cd storage

docker compose up -d
```

If you need import sample data, download the SQL file github.sql and import to the MySQL instance.

```bash
# Download sample data.
wget https://cdn.gorse.io/example/github.sql

# Import sample data.
mysql -h 127.0.0.1 -u gorse -pgorse_pass gorse < github.sql
```

### Option 1: Run an All-in-one Node

```bash
go run cmd/gorse-in-one/main.go --config config/config.toml
```

### Option 2: Run Nodes

- Start the master node with the configuration file.

```bash
go run cmd/gorse-master/main.go --config config/config.toml
```

- Start the worker node.

```bash
go run cmd/gorse-worker/main.go
```

- Start the server node.

```bash
go run cmd/gorse-server/main.go
```

### Run Unit Tests

Most logics in Gorse are covered by unit tests. Run unit tests by the following command:

```bash
go test -v ./...
```

The default database URLs are directed to these databases in `storage/docker-compose.yml`. Test databases could be overrode by setting following environment variables:

| Environment Value | Default Value                                |
|-------------------|----------------------------------------------|
| `MYSQL_URI`       | `mysql://root:password@tcp(127.0.0.1:3306)/` |
| `POSTGRES_URI`    | `postgres://gorse:gorse_pass@127.0.0.1/`     |
| `MONGO_URI`       | `mongodb://root:password@127.0.0.1:27017/`   |
| `CLICKHOUSE_URI`  | `clickhouse://127.0.0.1:8123/`               |
| `REDIS_URI`       | `redis://127.0.0.1:6379/`                    |

For example, use TiDB as a test database by:

```bash
MYSQL_URI=mysql://root:password@tcp(127.0.0.1:4000)/ go test -v ./...
```

## Your First Contribution

You can start by finding an existing issue with the [help wanted](https://github.com/gorse-io/gorse/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) label in the Gorse repository. These issues are well suited for new contributors. Issues can be claimed by publishing an `/assign` comment.

### Contribution Workflow

To contribute to the Gorse code base, please follow the workflow as defined in this section.

- Fork the repository to your own Github account
- Make commits and add test case if the change fixes a bug or adds new functionality.
- Run tests and make sure all the tests are passed.
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request.

This is a rough outline of what a contributor's workflow looks like. Thanks for your contributions!

## Getting Help

Join us in the [Discord](https://discord.gg/x6gAtNNkAE) and post your question in the `#developers` channel.

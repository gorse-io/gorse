# gorse: Go Recommender System Engine

<img width=160 src="https://gorse.io/zh/docs/img/gorse.png"/>

[![build](https://github.com/zhenghaoz/gorse/workflows/build/badge.svg)](https://github.com/zhenghaoz/gorse/actions?query=workflow%3Abuild)
[![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)
[![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse)
[![Discord](https://img.shields.io/discord/830635934210588743)](https://discord.com/channels/830635934210588743/)
<a target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=lOERnxfAM2U2rj4C9Htv9T68SLIXg6uk&jump_from=webapi"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="Gorse推荐系统交流群" title="Gorse推荐系统交流群"></a>

Gorse is an open source recommendation system written in Go. Gorse aims to be an universal open source recommender system that can be easily introduced into a wide variety of online services. By importing items, users and interaction data into Gorse, the system will automatically train models to generate recommendations for each user. Project features are as follows.

- **AutoML**: Choose the best recommendation model and stargety automatically by model searching in the background.
- **Distributed Recommendation**: Single node training, distributed prediction, and ability to achieve horizontal scaling in the recommendation stage.
- **RESTful API**: Provide RESTful APIs for data CRUD and recommendation requests.
- **Dashboard**: Provide dashboard for data import and export, monitoring, and cluster status checking.

## Quick Start

- [Run Gorse manually](https://github.com/zhenghaoz/gorse/tree/master/cmd)
- [Run Gorse with Docker Compose](https://github.com/zhenghaoz/gorse/tree/master/docker)
- [Use Gorse to recommend awesome GitHub repositories](https://github.com/zhenghaoz/gitrec)
- [Read official documents](https://docs.gorse.io/)

## Architecture

Gorse is a single node training and distributed prediction recommender system. Gorse stores data in MySQL or MongoDB, with intermediate data cached in Redis. The cluster consists of a master node, multiple worker nodes, and server nodes. The master node is responsible for model training, non-personalized item recommendation, configuration management, and membership management. The server node is responsible for exposing the RESTful APIs and online real-time recommendations. Worker nodes are responsible for offline recommendation for each user. In addition, administrator can perform system monitoring, data import and export, and system status checking via the dashboard on the master node.

<img width=480 src="https://github.com/zhenghaoz/gorse/blob/master/assets/architecture.png"/>

## Contributors

<a href="https://github.com/zhenghaoz/gorse/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=zhenghaoz/gorse" />
</a>

Any kind of contribution is expected: report a bug, give a advice or even create a pull request.

## Acknowledgments

`gorse` is inspired by following projects:

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)

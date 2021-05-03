Language: English | [中文](https://github.com/zhenghaoz/gorse/blob/master/README.zh-cn.md)

# gorse: Go Recommender System Engine

<img width=160 src="https://gorse.io/zh/docs/img/gorse.png"/>

[![build](https://github.com/zhenghaoz/gorse/workflows/build/badge.svg)](https://github.com/zhenghaoz/gorse/actions?query=workflow%3Abuild)
[![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)
[![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse)
[![Discord](https://img.shields.io/discord/830635934210588743)](https://discord.com/channels/830635934210588743/)
<a target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=lOERnxfAM2U2rj4C9Htv9T68SLIXg6uk&jump_from=webapi"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="Gorse推荐系统交流群" title="Gorse推荐系统交流群"></a>

Gorse is an open source recommendation system written in Go. Gorse aims to be an universal open source recommender system that can be easily introduced into a wide variety of online services. By importing items, users and interaction data into Gorse, the system will automatically train models to generate recommendations for each user. Project features are as follows.

- Supports multi-way matching (latest, popular, collaborative filtering based on matrix factorization) and FM-based personalized ranking.
- Single node training, distributed prediction, and ability to achieve horizontal scaling in the recommendation phase.
- Provide RESTful APIs for data CRUD and recommendation requests.
- Provide CLI tools for data import and export, model tuning, and cluster status checking.

## User Guide

- [Run Gorse with Docker Compose](https://github.com/zhenghaoz/gorse/tree/master/docker)
- [Quick Start](https://gorse.io/en/docs/chapter_2.html)
- [Configuration](https://gorse.io/en/docs/ch02-01-config.html)
- [Commands](https://gorse.io/en/docs/ch02-02-command.html)
- [RESTful APIs](https://gorse.io/en/docs/ch02-03-api.html)

## [Recommendation Principles](https://gorse.io/en/docs/ch01-01-principle.html)

The process of Gorse recommending items consists of two phases, **recall** and **sort**. The recall phase finds a collection of candidate items from the whole set of items for subsequent sorting. Due to the large number of items, the recommendation system is unable to perform the computational workload of sorting all items, so the recall phase mainly uses some simple strategies or models to collect the candidate items. At present, the system has implemented three types of recall methods, namely "recent popular items", "latest items" and "collaborative filtering". The sorting stage sorts the recalled items by removing duplicate items and historical items, and the sorting stage combines the items and user characteristics to make recommendations with more accuracy.

<img width=210 src="https://gorse.io/en/docs/img/dataflow.png"/>

## [System Architecture](https://gorse.io/en/docs/ch01-02-architect.html)

Gorse is a single node training and distributed prediction recommender system. Gorse stores data in MySQL or MongoDB, with intermediate data cached in Redis. The cluster consists of a master node, multiple worker nodes, and server nodes. The master node is responsible for ranking model training, collaborative filtering model training, non-personalized item matching, configuration management, and membership management. The server node is responsible for exposing the RESTful APIs and online real-time recommendations. Worker nodes are responsible for personalized matching for each user - currently only collaborative filtering is supported. In addition, administrator can perform model tuning, data import and export, and system status checking via the CLI.

<img width=480 src="https://gorse.io/en/docs/img/arch.png"/>

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

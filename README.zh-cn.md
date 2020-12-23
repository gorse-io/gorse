语言: [English](https://github.com/zhenghaoz/gorse) | 中文

# gorse: Go Recommender System Engine

<img width=160 src="https://img.sine-x.com/gorse.png"/>

| 持续集成 | 测试覆盖率 | 代码报告 | Go文档                                                       | 文档 |
|---|---|---|---|---|
| [![build](https://github.com/zhenghaoz/gorse/workflows/build/badge.svg)](https://github.com/zhenghaoz/gorse/actions?query=workflow%3Abuild) | [![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse) | [![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)  | [![Go Reference](https://pkg.go.dev/badge/github.com/zhenghaoz/gorse.svg)](https://pkg.go.dev/github.com/zhenghaoz/gorse) | [![Documentation Status](https://readthedocs.org/projects/gorse/badge/?version=latest)](https://gorse.readthedocs.io/en/latest/?badge=latest) |

`gorse` 是一个使用 Go 语言实现的推荐系统服务，系统整体架构如下：

<img width=540 src="https://img.sine-x.com/arch.png"/>

本项目以数据库为中心，构建出多节点分布式推荐系统，节点由主节点（Leader）、工作节点（Worker）和服务节点（Server）三种角色组成。

- **主节点**主要负责模型训练和用户划分。主节点定时拉取数据训练出召回模型和排序模型，分发给服务节点以及工作节点，已经将用户分组分发给召回模型进行候选物品生成。

- **工作节点**负责召回工作。工作节点使用召回模型（协同过滤）为每个用户生成候选物品，每个节点负责不同用户的候选物品生成。

- **服务节点**负责对外服务和实时排序。它实现了推荐系统的RESTful API，能够处理数据读写任务。最重要的是，在收到推荐请求的时候，使用CTR模型或者人工规则对候选物品进行排序。

除了以上的主要功能之外，系统还实现了以下的优化：

- [x] 提供用户友好的模型调参工具
- [x] 支持模型增量训练，加快模型收敛速度
- [ ] 支持SIMD和多线程训练，充分利用处理器性能

## 安装

可以以以下的不同方式安装

- 从[Release](https://github.com/zhenghaoz/gorse/releases)下载预编译的二进制可执行文件。
- 从DockerHub获取镜像

| 镜像         | 编译状态 |
| ------------ | -------- |
| gorse-server |          |
| gorse-leader |          |
| gorse-worker |          |

- 从源码编译

需要首先安装Go 编译器，然后使用 `go get` 安装

```
$ go get github.com/zhenghaoz/gorse/...
```

项目代码会被自动下载到本地， `gorse-cli` 、`gorse-leader`、`gorse-worker`和`gorse-server`四个程序被安装在` $GOBIN` 路径指定的文件夹中。

## 使用

```
gorse is an offline recommender system backend based on collaborative filtering written in Go.

Usage:
  gorse [flags]
  gorse [command]

Available Commands:
  export-feedback Export feedback to CSV
  export-items    Export items to CSV
  help            Help about any command
  import-feedback Import feedback from CSV
  import-items    Import items from CSV
  serve           Start a recommender sever
  test            Test a model by cross validation
  version         Check the version

Flags:
  -h, --help   help for gorse

Use "gorse [command] --help" for more information about a command.
```

### 评估推荐模型

*gorse*提供了评估模型的工具。可以运行 `gorse test -h` 获取访问 [在线文档](https://gorse.readthedocs.io/en/latest/usage/cross_validation.html) 来查看它的用法。例如我们可以评估一下BPR算法：

```bash
$ gorse test bpr --splitter k-fold --load-csv u.data --csv-sep $'\t' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr --n-negative 0
...
+--------------+----------+----------+----------+----------+----------+----------------------+
|              |  FOLD 1  |  FOLD 2  |  FOLD 3  |  FOLD 4  |  FOLD 5  |         MEAN         |
+--------------+----------+----------+----------+----------+----------+----------------------+
| Precision@10 | 0.321041 | 0.327128 | 0.321951 | 0.318664 | 0.317197 | 0.321196(±0.005931)  |
| Recall@10    | 0.212509 | 0.213825 | 0.213336 | 0.206255 | 0.210764 | 0.211338(±0.005083)  |
| NDCG@10      | 0.380665 | 0.385125 | 0.380003 | 0.369115 | 0.375538 | 0.378089(±0.008974)  |
| MAP@10       | 0.122098 | 0.123345 | 0.119723 | 0.116305 | 0.119468 | 0.120188(±0.003883)  |
| MRR@10       | 0.605354 | 0.601110 | 0.600359 | 0.577333 | 0.599930 | 0.596817(±0.019484)  |
+--------------+----------+----------+----------+----------+----------+----------------------+
```

示例中的 `u.data` 是 [MovieLens 100K](https://grouplens.org/datasets/movielens/100k/) 数据集中的用户-电影评分数据表， `u.item` 是 [MovieLens 100K](https://grouplens.org/datasets/movielens/100k/) 数据集中的物品数据表。有关命令行工具的使用，可见 Wiki 中的 [CLI-Tools](https://github.com/zhenghaoz/gorse/wiki/CLI-Tools)。

### 搭建推荐服务器

使用本项目构建一下推荐系统服务是相当容易的。

- **第一步**: 导入反馈和物品

```bash
$ gorse import-feedback ~/.gorse/gorse.db u.data --sep $'\t' --timestamp 2
$ gorse import-items ~/.gorse/gorse.db u.item --sep '|'
```

程序将反馈数据 `u.data` 和物品数据 `u.item` 导入到数据库文件 `~/.gorse/gorse.db`， 底层存储使用了 BoltDB，因此 `~/.gorse/gorse.db` 其实就是 BoltDB 的数据库文件。

- **第二步**: 启动服务程序

```bash
$ gorse serve -c config.toml
```

程序会加载配置文件 [config.toml](https://github.com/zhenghaoz/gorse/blob/master/example/file_config/config.toml) 之后启动一个推荐系统服务。之后，我们需要等待一段时间让推荐系统生成推荐给用户的候选物品列表以及物品的相似物品列表。配置文件的写法可以参考 Wiki 中的 [Configuration](https://github.com/zhenghaoz/gorse/wiki/Configuration)，配置文件中还设置了模型的参数，为了能够达到模型最佳效果，建议使用 [模型测试工具](https://github.com/zhenghaoz/gorse/wiki/CLI-Tools#cross-validation-tool) 验证模型推荐性能。

- **第三步**: 获取推荐结果

```bash
$ curl 127.0.0.1:8080/recommends/1?number=5
```

该命令从服务器请求获取 5 个推荐给 1 号用户的物品，返回的结果有可能是：

```
[
    {
        "ItemId": "919",
        "Popularity": 96,
        "Timestamp": "1995-01-01T00:00:00Z",
        "Score": 1
    },
    {
        "ItemId": "474",
        "Popularity": 194,
        "Timestamp": "1963-01-01T00:00:00Z",
        "Score": 0.9486470268850127
    },
    ...
]
```

其中，`"ItemId"` 为物品的ID，`"Score"` 是推荐模型生成的用于排序的分数。关于服务程序的更多 API 的使用方法，可以查看 Wiki 中的 [RESTful APIs](https://github.com/zhenghaoz/gorse/wiki/RESTful-APIs) 这一节。

## 模型

- **召回模型**



- **排序模型**



## 开发人员

[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/0)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/0)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/1)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/1)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/2)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/2)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/3)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/3)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/4)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/4)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/5)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/5)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/6)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/6)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/7)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/7)

欢迎向本项目进行贡献，包括提交BUG、建议或者代码。

## 致谢

本项目参考了以下项目的内容：

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)

## 缺陷

本项目存在一些缺陷，需要在后续迭代中解决：

- **单机模型**：目前支持数据并行，但是要求模型能够被单机训练和使用。
- **线上评估**：没有A/B测试和相关的在线推荐效果评估流程。

语言: [English](https://github.com/zhenghaoz/gorse) | 中文

# gorse: Go Recommender System Engine

<img width=160 src="https://img.sine-x.com/gorse.png"/>

| 持续集成 | 持续集成 (AVX2) | 测试覆盖率 | 代码报告 | 代码文档 | 使用文档 | 演示 |
|---|---|---|---|---|---|---|
| [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/1)](https://travis-ci.org/zhenghaoz/gorse) | [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/2)](https://travis-ci.org/zhenghaoz/gorse) | [![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse) | [![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)  | [![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse) | [![Documentation Status](https://readthedocs.org/projects/gorse/badge/?version=latest)](https://gorse.readthedocs.io/en/latest/?badge=latest) | [![Website](https://img.shields.io/website-up-down-green-red/https/steamlens.gorse.io.svg)](https://steamlens.gorse.io) |

`gorse` 是一个使用 Go 语言实现的、基于协同过滤算法的推荐系统后端。

本项目旨在于在小规模推荐任务下，提供一个高效的、易用的、编程语言无关的协同过滤推荐系统微服务。我们可以直接使用它构建一个简易的推荐系统，或者根据它生成的候选物品来构建更加精细的推荐服务。项目的主要特点如下：

- 实现了7个评分模型和4个排序模型。
- 支持数据加载、数据分割、模型训练、模型评估和参数搜索。
- 提供了数据导入导出工具、模型评估工具，以及最重要的 RESTful 推荐系统服务端。
- 使用SIMD加速向量计算，利用多线程加速数据处理过程。

<p align="center"><img width=540 src="https://img.sine-x.com/gorse-arch-zh-cn.png"/></p>

可以访问以下页面获取更多信息:

- 访问 [GoDoc](https://godoc.org/github.com/zhenghaoz/gorse) 查看代码的详细文档。
- 访问 [ReadTheDocs](https://gorse.readthedocs.io/) 查看教程、示例和使用指南。
- 访问 [SteamLens](https://github.com/zhenghaoz/SteamLens) 查看一个推荐Steam游戏的示例推荐系统。

## 安装

使用项目之前，需要首先按照 Go 编译器，然后使用 `go get` 安装

```bash
$ go get github.com/zhenghaoz/gorse/...
```

项目代码会被自动下载到本地，命令行程序 `gorse` 被安装在 $GOBIN 路径指定的文件夹中。

如果运行机器的 CPU 支持 AVX2 和 FMA3 指令集, 建议使用 `avx2` 标签编译项目以加速向量计算。不过，由于向量计算并不是性能瓶颈，所以加速的效果也是有限的。

```bash
$ go get -tags='avx2' github.com/zhenghaoz/gorse/...
```

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
$ gorse test bpr --load-csv u.data --csv-sep $'\t' --eval-precision --eval-recall --eval-ndcg --eval-map --eval-mrr
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
$ gorse import-feedback ~/.gorse/gorse.db u.data --sep $'\t'
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
        "ItemId": 202,
        "Score": 2.901297852545712
    },
    {
        "ItemId": 151,
        "Score": 2.8871064286482864
    },
    ...
]
```

其中，`"ItemId"` 为物品的ID，`"Score"` 是推荐模型生成的用于排序的分数。关于服务程序的更多 API 的使用方法，可以查看 Wiki 中的 [RESTful APIs](https://github.com/zhenghaoz/gorse/wiki/RESTful-APIs) 这一节。

### 在Go语言中使用

另外，可以在Go语言开发中导入gorse并使用。下面就是一个示例程序，该程序训练了一个推荐模型然后生成推荐结果：

```go
package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/base"
	"github.com/zhenghaoz/gorse/core"
	"github.com/zhenghaoz/gorse/model"
)

func main() {
	// Load dataset
	data := core.LoadDataFromBuiltIn("ml-100k")
	// Split dataset
	train, test := core.Split(data, 0.2)
	// Create model
	bpr := model.NewBPR(base.Params{
		base.NFactors:   10,
		base.Reg:        0.01,
		base.Lr:         0.05,
		base.NEpochs:    100,
		base.InitMean:   0,
		base.InitStdDev: 0.001,
	})
	// Fit model
	bpr.Fit(train, nil)
	// Evaluate model
	scores := core.EvaluateRank(bpr, test, train, 10, core.Precision, core.Recall, core.NDCG)
	fmt.Printf("Precision@10 = %.5f\n", scores[0])
	fmt.Printf("Recall@10 = %.5f\n", scores[1])
	fmt.Printf("NDCG@10 = %.5f\n", scores[1])
	// Generate recommendations for user(4):
	// Get all items in the full dataset
	items := core.Items(data)
	// Get user(4)'s ratings in the training dataset
	excludeItems := train.User(4)
	// Get top 10 recommended items (excluding rated items) for user(4) using BPR
	recommendItems, _ := core.Top(items, 4, 10, excludeItems, bpr)
	fmt.Printf("Recommend for user(4) = %v\n", recommendItems)
}
```

示例程序的输出应当为

```
2019/11/14 08:07:45 Fit BPR with hyper-parameters: n_factors = 10, n_epochs = 100, lr = 0.05, reg = 0.01, init_mean = 0, init_stddev = 0.001
2019/11/14 08:07:45 epoch = 1/100, loss = 55451.70899118173
...
2019/11/14 08:07:49 epoch = 100/100, loss = 10093.29427682404
Precision@10 = 0.31699
Recall@10 = 0.20516
NDCG@10 = 0.20516
Recommend for 4-th user = [288 313 245 307 328 332 327 682 346 879]
```

## 推荐模型

*gorse*中实现了11个推荐模型。

| 模型           | [数据]( <https://gorse.readthedocs.io/en/latest/introduction/data.html>) |      |          | [任务]( <https://gorse.readthedocs.io/en/latest/introduction/task.html>) |      | 多线程训练 |
| -------------- | ------------------------------------------------------------ | ---- | -------- | ------------------------------------------------------------ | ---- | ---------- |
|                | 显式（评分）                                                 | 隐式 | 隐式带权 | 评分                                                         | 排序 |            |
| BaseLine       | ✔️                                                            |      |          | ✔️                                                            | ✔️    |            |
| NMF            | ✔️                                                            |      |          | ✔️                                                            | ✔️    |            |
| SVD            | ✔️                                                            |      |          | ✔️                                                            | ✔️    |            |
| SVD++          | ✔️                                                            |      |          | ✔️                                                            | ✔️    | ✔️          |
| KNN            | ✔️                                                            |      |          | ✔️                                                            | ✔️    | ✔️          |
| CoClustering   | ✔️                                                            |      |          | ✔️                                                            | ✔️    | ✔️          |
| SlopeOne       | ✔️                                                            |      |          | ✔️                                                            | ✔️    | ✔️          |
| ItemPop        | ✔️                                                            | ✔️    |          |                                                              | ✔️    |            |
| KNN (Implicit) | ✔️                                                            | ✔️    |          | ✔️                                                            | ✔️    | ✔️          |
| WRMF           | ✔️                                                            | ✔️    | ✔️        |                                                              | ✔️    |            |
| BPR            | ✔️                                                            | ✔️    |          |                                                              | ✔️    |            |

- 在MovieLens 1M上对评分模型进行交叉验证 [[源代码](https://github.com/zhenghaoz/gorse/blob/master/example/benchmark_rating/main.go)].

| 模型         | RMSE    | MAE     | Time    | (AVX2)  |
| ------------ | ------- | ------- | ------- | ------- |
| SlopeOne     | 0.90683 | 0.71541 | 0:00:26 |         |
| CoClustering | 0.90701 | 0.71212 | 0:00:08 |         |
| KNN          | 0.86462 | 0.67663 | 0:02:07 |         |
| SVD          | 0.84252 | 0.66189 | 0:02:21 | 0:01:48 |
| SVD++        | 0.84194 | 0.66156 | 0:03:39 | 0:02:47 |

- 在MovieLens 100K上对排序模型进行交叉验证 [[源代码](https://github.com/zhenghaoz/gorse/blob/master/example/benchmark_ranking/main.go)].

| 模型    | [Precision@10](mailto:Precision%4010) | [Recall@10](mailto:Recall%4010) | [MAP@10](mailto:MAP%4010) | [NDCG@10](mailto:NDCG%4010) | [MRR@10](mailto:MRR%4010) | 用时    |
| ------- | ------------------------------------- | ------------------------------- | ------------------------- | --------------------------- | ------------------------- | ------- |
| ItemPop | 0.19081                               | 0.11584                         | 0.05364                   | 0.21785                     | 0.40991                   | 0:00:03 |
| KNN     | 0.28584                               | 0.19328                         | 0.11358                   | 0.34746                     | 0.57766                   | 0:00:41 |
| BPR     | 0.32083                               | 0.20906                         | 0.11848                   | 0.37643                     | 0.59818                   | 0:00:13 |
| WRMF    | 0.34727                               | 0.23665                         | 0.14550                   | 0.41614                     | 0.65439                   | 0:00:14 |

## 性能

本项目运行速度快于 [Surprise](http://surpriselib.com/)，和 [librec](https://www.librec.net/) 持平，但是需要的运行内存更少。

- 在 MovieLens 100K 数据集上对SVD模型进行交叉验证 \[[源代码](https://github.com/zhenghaoz/gorse/tree/master/example/benchmark_perf)\]:

<img width=320 src="https://img.sine-x.com/perf_time_svd_ml_100k.png"><img width=320 src="https://img.sine-x.com/perf_mem_svd_ml_100k.png">

- 在 MovieLens 1M 数据集上对SVD模型进行交叉验证 \[[源代码](https://github.com/zhenghaoz/gorse/tree/master/example/benchmark_perf)\]:

<img width=320 src="https://img.sine-x.com/perf_time_svd_ml_1m.png"><img width=320 src="https://img.sine-x.com/perf_mem_svd_ml_1m.png">

## 开发人员

[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/0)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/0)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/1)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/1)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/2)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/2)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/3)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/3)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/4)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/4)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/5)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/5)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/6)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/6)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/7)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/7)

欢迎向本项目进行贡献，包括提交BUG、建议或者代码。

## 致谢

本项目参考了以下项目的内容：

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)

## 缺陷

本项目存在一些缺陷，它的应用场景也是非常有限的：

- **无扩展**: 只能在单机上运行，因此也就无法处理大数据。
- **无特征**: 只使用用户和物品之间的交互记录进行推荐，无法利用用户特征或者物品特征。

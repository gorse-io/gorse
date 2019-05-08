语言: [English](https://github.com/zhenghaoz/gorse) | 中文

# gorse: Go Recommender System Engine

<img width=160 src="https://img.sine-x.com/gorse.png"/>

| 持续集成 | 持续集成 (AVX2) | 测试覆盖率 | 代码文档 | 代码报告 |
|---|---|---|---|---|
| [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/1)](https://travis-ci.org/zhenghaoz/gorse) | [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/2)](https://travis-ci.org/zhenghaoz/gorse) | [![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse) | [![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse) | [![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse) |

`gorse` 是一个使用 Go 语言实现的、基于协同过滤算法的推荐系统后端。本项目旨在于在小规模推荐任务下，提供一个高效的、易用的、编程语言无关的协同过滤推荐系统微服务。我们可以直接使用它构建一个简易的推荐系统，或者根据它生成的候选物品来构建更加精细的推荐服务。项目的主要特点如下：

- **完整的流程**: 支持数据加载、数据分割、模型训练、模型评估和参数搜索。
- **丰富的工具**: 提供了数据导入导出工具、模型评估工具，以及最重要的 RESTful 推荐系统服务端。
- **高效的优化**: 使用SIMD加速向量计算，利用多线程加速数据处理过程。

## 安装

使用项目之前，需要首先按照 Go 编译器，然后使用 `go get` 安装

```bash
go get github.com/zhenghaoz/gorse/...
```

项目代码会被自动下载到本地，命令行程序 `gorse` 被安装在 $GOBIN 路径指定的文件夹中。

如果运行机器的 CPU 支持 AVX2 和 FMA3 指令集, 建议使用 `avx2` 标签编译项目以加速向量计算。不过，由于向量计算并不是性能瓶颈，所以加速的效果也是有限的。

```bash
go get -tags='avx2' github.com/zhenghaoz/gorse/...
```

## 使用

使用本项目构建一下推荐系统服务是相当容易的。

- **第一步**: 导入反馈和物品

```bash
gorse import-feedback ~/.gorse/gorse.db u.data --sep $'\t'
gorse import-items ~/.gorse/gorse.db u.item --sep '|'
```

程序将反馈数据 `u.data` 和物品数据 `u.item` 导入到数据库文件 `~/.gorse/gorse.db`， 底层存储使用了 BoltDB，因此 `~/.gorse/gorse.db` 其实就是 BoltDB 的数据库文件。示例中的 `u.data` 是 MovieLens 100K 数据集中的用户-电影评分数据表， `u.item` 是 MovieLens 100K 数据集中的物品数据表。有关命令行工具的使用，可见 Wiki 中的 [CLI-Tools](https://github.com/zhenghaoz/gorse/wiki/CLI-Tools)。

- **第二步**: 启动服务程序

```bash
gorse server -c config.toml
```

程序会加载配置文件 `config.toml` 之后启动一个推荐系统服务。之后，我们需要等待一段时间让推荐系统生成推荐给用户的候选物品列表以及物品的相似物品列表。配置文件的写法可以参考 Wiki 中的 [Configuration](https://github.com/zhenghaoz/gorse/wiki/Configuration)，配置文件中还设置了模型的参数，为了能够达到模型最佳效果，建议使用 [模型测试工具](https://github.com/zhenghaoz/gorse/wiki/CLI-Tools#cross-validation-tool) 验证模型推荐性能。

- **第三步**: 获取推荐结果

```bash
curl 127.0.0.1:8080/recommends/1?number=5
```

该命令从服务器请求获取 5 个推荐给 1 号用户的物品，返回的结果有可能是：

```json
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

## 文档

- 访问 [GoDoc](https://godoc.org/github.com/zhenghaoz/gorse) 查看代码的详细文档。
- 访问 [Wiki](https://github.com/zhenghaoz/gorse/wiki) 查看教程、示例和概述。

## 性能

本项目运行速度快于 Surprise，和 librec 持平，但是需要的运行内存更少。

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

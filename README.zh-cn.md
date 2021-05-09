语言: [English](https://github.com/zhenghaoz/gorse) | 中文

# gorse: Go Recommender System Engine

<img width=160 src="https://gorse.io/zh/docs/img/gorse.png"/>

[![build](https://github.com/zhenghaoz/gorse/workflows/build/badge.svg)](https://github.com/zhenghaoz/gorse/actions?query=workflow%3Abuild)
[![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)
[![Go Reference](https://pkg.go.dev/badge/github.com/zhenghaoz/gorse.svg)](https://pkg.go.dev/github.com/zhenghaoz/gorse)
[![Discord](https://img.shields.io/discord/830635934210588743)](https://discord.com/channels/830635934210588743/)
<a target="_blank" href="https://qm.qq.com/cgi-bin/qm/qr?k=lOERnxfAM2U2rj4C9Htv9T68SLIXg6uk&jump_from=webapi"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="Gorse推荐系统交流群" title="Gorse推荐系统交流群"></a>

*Gorse*是一个Go语言编写的开源推荐系统。Gorse希望成为一个具有普适性开源推荐系统，可以方便地引入到各种各样的在线服务中。 将物品、用户和它们之间的交互反馈数据导入到Gorse中，系统将自动训练模型，为每个用户实时生成推荐。项目特点如下：

- 支持多路召回（最新、热门、基于矩阵分解的协同过滤）和基于FM的个性化排序。
- 单机训练，分布式预测，能够在推荐预测阶段实现水平扩展。
- 提供RESTful接口，用于数据的增删改查和推荐结果的获取。
- 提供CLI工具，用于数据导入导出、模型调参、集群状态检查。

## 使用指南

- [手动运行Gorse](https://github.com/zhenghaoz/gorse/tree/master/cmd)
- [借助Docker Compose运行Gorse](https://github.com/zhenghaoz/gorse/tree/master/docker)

## [推荐原理](https://gorse.io/zh/docs/ch01-01-principle.html)

Gorse推荐物品的过程由**召回**和**排序**两个阶段构成。召回阶段从全体物品中寻找出一个候选物品集合用于后续排序。由于物品的数量庞大，推荐系统无法完成对全体物品进行排序的计算量，因此召回阶段主要使用一些简单的策略或者模型去搜集候选物品。目前，系统已经实现了“最近热门物品”、“最新物品”和“协同过滤”三种召回方式。排序阶段将召回物品去掉重复物品和历史物品之后进行排序，排序阶段会结合物品和用户的特征进行推荐，通过更加精准。

<img width=210 src="https://gorse.io/zh/docs/img/dataflow.png"/>

## [系统架构](https://gorse.io/zh/docs/ch01-02-architect.html)

Gorse是一个单机训练分布式预测的推荐系统。Gorse将数据存储在MySQL或者MongoDB中，中间数据缓存在Redis中。集群又一个主节点、多个工作节点和服务节点构成。主节点负责排序模型训练、协同过滤模型训练、非个性化物品召回、配置管理和成员管理。服务节点负责暴露系统的HTTP接口，以及负责在线实时推荐。工作节点负责为每个用户进行个性化召回——目前仅支持协同过滤召回。另外，运维人员可以通过CLI进行模型调参、数据导入导出和系统状态查看。

<img width=480 src="https://gorse.io/zh/docs/img/arch.png"/>

## 开发人员

<a href="https://github.com/zhenghaoz/gorse/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=zhenghaoz/gorse" />
</a>

欢迎向本项目进行贡献，包括提交BUG、建议或者代码。

## 致谢

本项目参考了以下项目的内容：

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)


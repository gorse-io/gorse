# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/zhenghaoz/gorse.svg?branch=master)](https://travis-ci.org/zhenghaoz/gorse)
[![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse)
[![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)
[![](https://img.shields.io/badge/stability-experimental-orange.svg)](#)

`gorse` is a recommender system engine implemented by the go programming language. It provides

- **Algorithm**: Predict ratings based on collaborate filtering. Including matrix factorization and neighborhood-based method.
- **Data**: Load data from the built-in dataset or file. Split data to train set and test set.
- **Evaluator**: Evaluate models by cross-validation using RMSE or MAE.

## Installation

```bash
go get -t -v -u github.com/zhenghaoz/gorse
```

## Usage


## Document



## Benchmarks



## Acknowledge

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Ashley McNamara's gophers](https://github.com/ashleymcnamara/gophers)

## References

1. Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

2. G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, [LibRec: A Java Library for Recommender Systems](http://ceur-ws.org/Vol-1388/demo_paper1.pdf), in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

3. Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

4. Lemire, Daniel, and Anna Maclachlan. "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

5. George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

6. Guo, Guibing, Jie Zhang, and Neil Yorke-Smith. "A Novel Bayesian Similarity Measure for Recommender Systems." IJCAI. 2013.

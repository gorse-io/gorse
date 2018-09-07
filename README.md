# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/ZhangZhenghao/gorse.svg?branch=master)](https://travis-ci.org/ZhangZhenghao/gorse)
[![codecov](https://codecov.io/gh/ZhangZhenghao/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/ZhangZhenghao/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/ZhangZhenghao/gorse)](https://goreportcard.com/report/github.com/ZhangZhenghao/gorse)

## Benchmarks

Here are the average RMSE, MAE and total execution time of various algorithms (with their default parameters) on a 5-fold cross-validation procedure. The datasets are the [Movielens](http://grouplens.org/datasets/movielens/) 100k and 1M datasets. The folds are the same for all the algorithms.

|   [Movielens 100k](http://grouplens.org/datasets/movielens/100k)   |   RMSE   |   MAE    |    Time  |
| - | - | - | - |
| Random   | 1.518610 | 1.218645 | 00:01 |
| BaseLine | 0.943741 | 0.741738 | 00:01 |
| SVD      | 0.938568 | 0.736788 | 00:10 |
| SVD++    | 0.924482 | 0.722409 | 06:04 |

## References

1. Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

2. Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

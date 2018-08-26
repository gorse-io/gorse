# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/ZhangZhenghao/gorse.svg?branch=master)](https://travis-ci.org/ZhangZhenghao/gorse)
[![codecov](https://codecov.io/gh/ZhangZhenghao/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/ZhangZhenghao/gorse)

## Benchmarks

Here are the average RMSE, MAE and total execution time of various algorithms (with their default parameters) on a 5-fold cross-validation procedure. The datasets are the [Movielens](http://grouplens.org/datasets/movielens/) 100k and 1M datasets. The folds are the same for all the algorithms.

|   [Movielens 100k](http://grouplens.org/datasets/movielens/100k)   |   RMSE   |   MAE    |    Time  |
| - | - | - | - |
| Random   | 1.518610 | 1.218645 | 00:01 |
| BaseLine | 0.943741 | 0.741738 | 00:01 |
| SVD      | 0.938568 | 0.736788 | 00:10 |
| SVD++    | 0.924482 | 0.722409 | 06:04 |



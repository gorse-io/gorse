# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/ZhangZhenghao/gorse.svg?branch=master)](https://travis-ci.org/ZhangZhenghao/gorse)
[![codecov](https://codecov.io/gh/ZhangZhenghao/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/ZhangZhenghao/gorse)
[![Document](https://godoc.org/github.com/ZhangZhenghao/gorse?status.svg)](https://godoc.org/github.com/ZhangZhenghao/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/ZhangZhenghao/gorse)](https://goreportcard.com/report/github.com/ZhangZhenghao/gorse)

## Usage

```go
import "github.com/ZhangZhenghao/gorse/core"
```

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

2. G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, [LibRec: A Java Library for Recommender Systems](http://ceur-ws.org/Vol-1388/demo_paper1.pdf), in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

3. Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

4. Lemire, Daniel, and Anna Maclachlan. "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

5. George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

6. Li, Dongsheng, et al. "Mixture-Rank Matrix Approximation for Collaborative Filtering." Advances in Neural Information Processing Systems. 2017.

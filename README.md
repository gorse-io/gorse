# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/ZhangZhenghao/gorse.svg?branch=master)](https://travis-ci.org/ZhangZhenghao/gorse)
[![codecov](https://codecov.io/gh/ZhangZhenghao/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/ZhangZhenghao/gorse)
[![Document](https://godoc.org/github.com/ZhangZhenghao/gorse?status.svg)](https://godoc.org/github.com/ZhangZhenghao/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/ZhangZhenghao/gorse)](https://goreportcard.com/report/github.com/ZhangZhenghao/gorse)

## Installation

```bash
go get -t -v -u github.com/ZhangZhenghao/gorse/core
```

## Usage

```go
// Load build-in data
data := core.LoadDataFromBuiltIn("ml-100k")
// Create a recommender
algo := core.NewSVD()
// Cross validate
cv := core.CrossValidate(algo, data, []core.Evaluator{core.RMSE, core.MAE},5, 0, nil)
// Print RMSE & MAE
fmt.Printf("RMSE = %f, MAE = %f\n", 
           stat.Mean(cv[0].Tests, nil), 
           stat.Mean(cv[1].Tests, nil))
```

**Output**:

```
RMSE = 0.938904, MAE = 0.737349
```

## Benchmarks

|   [Movielens 100k](http://grouplens.org/datasets/movielens/100k)   |   RMSE   |   MAE    |    Time  | RMSE[Ref] |  MAE[Ref]  |
| - | - | - | - | - | - |
| Random        | 1.518610 | 1.218645 | 00:01   | 1.514[1] | 1.215[1] |
| BaseLine      | 0.943741 | 0.741738 | 00:01  | 0.944[1] | 0.748[1] |
| SVD           | 0.938904 | 0.737349 | 00:05  | 0.934[1] | 0.737[1] |
| SVD++         | <span style="color:red">0.933238</span> | 0.730298 | 05:03 | 0.92[1] | 0.722[1] |
| NMF[3]           | 0.970431 | 0.762025 | 00:07  | 0.963[1] | 0.758[1] |
| KNN           | 0.978720 | 0.773133 | 00:07  | 0.98[1] | 0.774[1] |
| Centered k-NN | 0.952906 | 0.751667 | 00:07  | 0.951[1] | 0.749[1] |
| k-NN Z-Score  | 0.953099 | 0.748451 | 00:07  |   |   |
| k-NN Baseline | 0.933491 | 0.734687 | 00:09  | 0.931[1] | 0.733[1] |
| Slope One[4]  | 0.940748 | 0.741195 | 00:09  | 0.946[1] | 0.743[1] |
| Co-Clustering[5] | 0.967668 | 0.760593 | 00:05  | 0.963[1] | 0.753[1] |

## References

1. Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

2. G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, [LibRec: A Java Library for Recommender Systems](http://ceur-ws.org/Vol-1388/demo_paper1.pdf), in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

3. Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

4. Lemire, Daniel, and Anna Maclachlan. "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

5. George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

6. Li, Dongsheng, et al. "Mixture-Rank Matrix Approximation for Collaborative Filtering." Advances in Neural Information Processing Systems. 2017.

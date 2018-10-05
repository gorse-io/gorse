# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/ZhangZhenghao/gorse.svg?branch=master)](https://travis-ci.org/ZhangZhenghao/gorse)
[![codecov](https://codecov.io/gh/ZhangZhenghao/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/ZhangZhenghao/gorse)
[![GoDoc](https://godoc.org/github.com/ZhangZhenghao/gorse?status.svg)](https://godoc.org/github.com/ZhangZhenghao/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/ZhangZhenghao/gorse)](https://goreportcard.com/report/github.com/ZhangZhenghao/gorse)
[![](https://img.shields.io/badge/stability-experimental-orange.svg)]()

`gorse` is a recommender system engine implemented by the go programming language. It provides

- **Algorithm**: Predict ratings based on collaborate filtering. Including matrix factorization and neighborhood-based method.
- **Data**: Load data from the built-in dataset or file. Split data to train set and test set.
- **Evaluator**: Evaluate models by cross-validation using RMSE or MAE.

## Installation

```bash
go get -t -v -u github.com/ZhangZhenghao/gorse/core
```

## Usage

```go
// Load build-in data
data := core.LoadDataFromBuiltIn("ml-100k")
// Create a recommender
algo := core.NewSVD(nil)
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

## Tutorial

- [实现一个推荐系统引擎(一)：评分预测](https://sine-x.com/gorse-1/)

## Benchmarks

All algorithms are tested on a PC with Intel(R) Core(TM) i5-4590 CPU (3.30GHz) and 16.0GB RAM. RMSE scores and MAE scores are used to check the correctness comparing to other implementation but not the best performance. Parameters are set as default values and identical to other implementation.

| ml-100k                                                      | RMSE  | MAE   | Time    |
| ------------------------------------------------------------ | ----- | ----- | ------- |
| [SVD](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#SVD) | 0.939 | 0.737 | 0:00:02 |
| [SVD++](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#SVDpp) | 0.925 | 0.722 | 0:01:41 |
| [NMF[3]](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NMF) | 0.970 | 0.761 | 0:00:02 |
| [Slope One[4]](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#SlopeOne) | 0.941 | 0.741 | 0:00:01 |
| [KNN](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNN) | 0.979 | 0.773 | 0:00:03 |
| [Centered k-NN](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNWithMean) | 0.953 | 0.752 | 0:00:03 |
| [k-NN Baseline](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNBaseLine) | 0.934 | 0.735 | 0:00:03 |
| [k-NN Z-Score](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNWithZScore) | 0.953 | 0.748 | 0:00:03 |
| [Co-Clustering[5]](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#CoClustering) | 0.968 | 0.761 | 0:00:00 |
| [BaseLine](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#BaseLine) | 0.944 | 0.742 | 0:00:00 |
| [Random](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#Random) | 1.518 | 1.218 | 0:00:00 |

| ml-1m                                                        | RMSE  | MAE   | Time    |
| ------------------------------------------------------------ | ----- | ----- | ------- |
| [SVD](https://godoc.org/github.com/ZhangZhenghao/gorse/core#SVD) | 0.877 | 0.688 | 0:00:29 |
| [SVD++](https://godoc.org/github.com/ZhangZhenghao/gorse/core#SVDpp) | 0.865 | 0.678 | 0:27:54 |
| [NMF[3]](https://godoc.org/github.com/ZhangZhenghao/gorse/core#NMF) | 0.918 | 0.726 | 0:00:44 |
| [Slope One[4]](https://godoc.org/github.com/ZhangZhenghao/gorse/core#SlopeOne) | 0.906 | 0.715 | 0:00:23 |
| [KNN](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNN) | 0.923 | 0.727 | 0:03:27 |
| [Centered k-NN](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNWithMean) | 0.929 | 0.739 | 0:03:40 |
| [k-NN Baseline](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNBaseLine) | 0.896 | 0.707 | 0:03:36 |
| [k-NN Z-Score](https://goDoc.org/github.com/ZhangZhenghao/gorse/core#NewKNNWithZScore) | 0.931 | 0.737 | 0:03:28 |
| [Co-Clustering[5]](https://godoc.org/github.com/ZhangZhenghao/gorse/core#CoClustering) | 0.917 | 0.720 | 0:00:04 |
| [BaseLine](https://godoc.org/github.com/ZhangZhenghao/gorse/core#BaseLine) | 0.910 | 0.717 | 0:00:03 |
| [Random](https://godoc.org/github.com/ZhangZhenghao/gorse/core#Random) | 1.505 | 1.207 | 0:00:01 |

## References

1. Hug, Nicolas. Surprise, a Python library for recommender systems. http://surpriselib.com, 2017.

2. G. Guo, J. Zhang, Z. Sun and N. Yorke-Smith, [LibRec: A Java Library for Recommender Systems](http://ceur-ws.org/Vol-1388/demo_paper1.pdf), in Posters, Demos, Late-breaking Results and Workshop Proceedings of the 23rd Conference on User Modelling, Adaptation and Personalization (UMAP), 2015.

3. Luo, Xin, et al. "An efficient non-negative matrix-factorization-based approach to collaborative filtering for recommender systems." IEEE Transactions on Industrial Informatics 10.2 (2014): 1273-1284.

4. Lemire, Daniel, and Anna Maclachlan. "Slope one predictors for online rating-based collaborative filtering." Proceedings of the 2005 SIAM International Conference on Data Mining. Society for Industrial and Applied Mathematics, 2005.

5. George, Thomas, and Srujana Merugu. "A scalable collaborative filtering framework based on co-clustering." Data Mining, Fifth IEEE international conference on. IEEE, 2005.

6. Li, Dongsheng, et al. "Mixture-Rank Matrix Approximation for Collaborative Filtering." Advances in Neural Information Processing Systems. 2017.

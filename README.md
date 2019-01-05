# gorse: Go Recommender System Engine

[![Build Status](https://travis-ci.org/zhenghaoz/gorse.svg?branch=master)](https://travis-ci.org/zhenghaoz/gorse)
[![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse)
[![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse)
[![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse)
[![codebeat badge](https://codebeat.co/badges/beb25c18-e35f-4783-b3ad-b518dc4ea78a)](https://codebeat.co/projects/github-com-zhenghaoz-gorse-master)
[![](https://img.shields.io/badge/acceleration-AVX2,%20FMA3-blue.svg)](#)

`gorse` is a recommender system engine implemented by the go programming language. It provides

- **Data**: Load data from built-in datasets or custom files.
- **Splitter**: Split dataset by [k-fold](https://godoc.org/github.com/zhenghaoz/gorse/core#NewKFoldSplitter), [ratio](https://godoc.org/github.com/zhenghaoz/gorse/core#NewRatioSplitter) or [leave-one-out](https://godoc.org/github.com/zhenghaoz/gorse/core#NewUserLOOSplitter).
- **Model**: [Recommendation models](https://godoc.org/github.com/zhenghaoz/gorse/model) based on collaborate filtering including matrix factorization, neighborhood-based method, Slope One and Co-Clustering.
- **Evaluator**: Implemented [RMSE](https://godoc.org/github.com/zhenghaoz/gorse/core#RMSE) and [MAE](https://godoc.org/github.com/zhenghaoz/gorse/core#MAE) for rating task. For ranking task, there are [Precision](https://godoc.org/github.com/zhenghaoz/gorse/core#NewPrecision), [Recall](https://godoc.org/github.com/zhenghaoz/gorse/core#NewRecall), [NDCG](https://godoc.org/github.com/zhenghaoz/gorse/core#NewNDCG), [MAP](https://godoc.org/github.com/zhenghaoz/gorse/core#NewMAP), [MRR](https://godoc.org/github.com/zhenghaoz/gorse/core#NewMRR) and [AUC](https://godoc.org/github.com/zhenghaoz/gorse/core#AUC).
- **Parameter Search**: Find best hyper-parameters using [grid search](https://godoc.org/github.com/zhenghaoz/gorse/core#GridSearchCV) or [random search](https://godoc.org/github.com/zhenghaoz/gorse/core#RandomSearchCV).
- **Persistence**: [Save](https://godoc.org/github.com/zhenghaoz/gorse/core#Save) a model or [load](https://godoc.org/github.com/zhenghaoz/gorse/core#Load) a model.

## Installation

```bash
go get github.com/zhenghaoz/gorse
```

## Usage

Examples and tutorials could be found in [wiki](https://github.com/zhenghaoz/gorse/wiki). Let's get started with a simple example:

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
	svd := model.NewSVD(base.Params{
		base.Lr:       0.007,
		base.NEpochs:  100,
		base.NFactors: 80,
		base.Reg:      0.1,
	})
	// Fit model
	svd.Fit(train)
	// Evaluate model
	fmt.Printf("RMSE = %.5f\n", core.RMSE(svd, test))
	// Predict a rating
	fmt.Printf("Predict(4,8) = %.5f\n", svd.Predict(4, 8))
}
```

The output would be:

```
RMSE = 0.91305
Predict(4,8) = 4.72873
```

More examples could be found in the [example](https://github.com/zhenghaoz/gorse/tree/master/example) folder.

## Benchmarks

All models are tested by 5-fold cross validation on a PC with Intel(R) Core(TM) i5-4590 CPU (3.30GHz) and 16.0GB RAM. All scores are the best scores achieved by `gorse` yet.

- Rating Prediction on MovieLens 1M ([source](https://github.com/zhenghaoz/gorse/blob/master/example/benchmark_rating_ml_1m/main.go))

|    Model     |       RMSE        |        MAE        |  Time   |
|--------------|-------------------|-------------------|---------|
| SlopeOne     | 0.90691 | 0.71541 | 0:00:35 |
| CoClustering | 0.90701 | 0.71212 | 0:00:11 |
| KNN          | 0.86462 | 0.67663 | 0:02:05 |
| SVD          | 0.84252 | 0.66189 | 0:02:37 |
| SVD++        | **0.84201** | **0.66161** | 0:04:43 |

- Item Ranking on MovieLens 100K ([source](https://github.com/zhenghaoz/gorse/blob/master/example/benchmark_ranking/main.go))

|  Model  |   PREC@10    |     RECALL@10     |      MAP@10       |      NDCG@10      |      MRR@10       |  Time   |
|---------|-------------------|-------------------|-------------------|-------------------|-------------------|---------|
| ItemPop | 0.19081 | 0.11584 | 0.05364 | 0.21785 | 0.40991 | 0:00:03 |
| SVD-BPR     | 0.32083 | 0.20906 | 0.11848 | 0.37643 | 0.59818 | 0:00:13 |
| WRMF    | **0.34727** | **0.23665** | **0.14550** | **0.41614** | **0.65439** | 0:00:14 |

See detail in [wiki](https://github.com/zhenghaoz/gorse/wiki/Benchmark).

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

7. Hu, Yifan, Yehuda Koren, and Chris Volinsky. "Collaborative filtering for implicit feedback datasets." Data Mining, 2008. ICDM'08. Eighth IEEE International Conference on. Ieee, 2008.

8. Massa, Paolo, and Paolo Avesani. "Trust-aware recommender systems." Proceedings of the 2007 ACM conference on Recommender systems. ACM, 2007.
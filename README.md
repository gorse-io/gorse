<img width=160 src="https://img.sine-x.com/gorse.png"/>

# gorse: Go Recommender System Engine

| Build | Build (AVX2) | Coverage | Document | Report |
|---|---|---|---|---|
| [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/1)](https://travis-ci.org/zhenghaoz/gorse) | [![Build Status](https://travis-matrix-badges.herokuapp.com/repos/zhenghaoz/gorse/branches/master/2)](https://travis-ci.org/zhenghaoz/gorse) | [![codecov](https://codecov.io/gh/zhenghaoz/gorse/branch/master/graph/badge.svg)](https://codecov.io/gh/zhenghaoz/gorse) | [![GoDoc](https://godoc.org/github.com/zhenghaoz/gorse?status.svg)](https://godoc.org/github.com/zhenghaoz/gorse) | [![Go Report Card](https://goreportcard.com/badge/github.com/zhenghaoz/gorse)](https://goreportcard.com/report/github.com/zhenghaoz/gorse) |

`gorse` is a a transparent recommender system engine over SQL database based on collaborative filtering written in Go.

- **Data**: Load data from built-in datasets or custom files.
- **Splitter**: Split dataset by [k-fold](https://godoc.org/github.com/zhenghaoz/gorse/core#NewKFoldSplitter), [ratio](https://godoc.org/github.com/zhenghaoz/gorse/core#NewRatioSplitter) or [leave-one-out](https://godoc.org/github.com/zhenghaoz/gorse/core#NewUserLOOSplitter).
- **Model**: [Recommendation models](https://godoc.org/github.com/zhenghaoz/gorse/model) based on collaborate filtering including matrix factorization, neighborhood-based method, Slope One and Co-Clustering.
- **Evaluator**: Implemented [RMSE](https://godoc.org/github.com/zhenghaoz/gorse/core#RMSE) and [MAE](https://godoc.org/github.com/zhenghaoz/gorse/core#MAE) for rating task. For ranking task, there are [Precision](https://godoc.org/github.com/zhenghaoz/gorse/core#NewPrecision), [Recall](https://godoc.org/github.com/zhenghaoz/gorse/core#NewRecall), [NDCG](https://godoc.org/github.com/zhenghaoz/gorse/core#NewNDCG), [MAP](https://godoc.org/github.com/zhenghaoz/gorse/core#NewMAP), [MRR](https://godoc.org/github.com/zhenghaoz/gorse/core#NewMRR)and [AUC](https://godoc.org/github.com/zhenghaoz/gorse/core#AUC).
- **Parameter Search**: Find best hyper-parameters using [grid search](https://godoc.org/github.com/zhenghaoz/gorse/core#GridSearchCV) or [random search](https://godoc.org/github.com/zhenghaoz/gorse/core#RandomSearchCV).
- **Persistence**: [Save](https://godoc.org/github.com/zhenghaoz/gorse/core#Save) a model or [load](https://godoc.org/github.com/zhenghaoz/gorse/core#Load) a model.
- **SIMD** (Optional): Vectors are computed by AVX2 instructions which are 4 times faster than single instructions in theory.

## Build

`gorse` could be built by

```go
go build github.com/zhenghaoz/gorse/cmd/gorse.go
```

If the CPU of your device supports AVX2 and FMA3 instructions, use the `avx2` build tag to enable AVX2 support.

```bash
go build -tags='avx2' github.com/zhenghaoz/gorse/cmd/gorse.go
```

## Setup

It's easy to setup a recomendation service with `gorse`. 

- **Step 1**: initialize the database.

```bash
./gorse init user:pass@host/database
```

It connects to a SQL database and creates several tables.  `user:pass@host/database` is the database used to store data of `gorse`.

- **Step 2**: Import ratings and Items.

```
./gorse data user:pass@host/database \
	--import-ratings-csv u.data \
	--import-items-csv u.item
```

It imports ratings and items from CSV files. `u.data` is the CSV file of ratings in MovieLens 100K dataset and `u.item` is the CSV file of items in MovieLens 100K dataset.

- **Step 3**: Start a server.

```bash
./gorse server -c config.toml
```

It loads configurations from `config.toml` and starts a recommendation server.

```toml
# This section declares settings for the server.
[server]
host = "127.0.0.1"      # server host
port = 8080             # server port

# This section declares setting for the database.
[database]
driver = "mysql"        # database driver
access = "gorse:password@/gorse"# database access

# This section declares settings for recommendation.
[recommend]
model = "svd"           # recommendation model
cache_size = 100        # the number of cached recommendations
update_threshold = 10   # update model when more than 10 ratings are added
check_period = 1        # check for update every one minute
similarity = "pearson"  # similarity metric for neighbors

# This section declares hyperparameters for the recommendation model.
[params]
optimizer = "bpr"       # the optimizer to oprimize matrix factorization model
n_factors = 10          # the number of latent factors
reg = 0.01              # regularization strength
lr = 0.05               # learning rate
n_epochs = 100          # the number of learning epochs
init_mean = 0.0         # the mean of initial latent factors initilaized by Gaussian distribution
init_std = 0.001        # the standard deviation of initial latent factors initilaized by Gaussian distribution
```

- **Step 4**: Send requests.

```bash
curl 127.0.0.1:8080/recommends/1?number=5
```



```json
{
  "Failed": false,
  "Items": [284, 448, 763, 276, 313]
}
```



## Document

- Visit [GoDoc](https://godoc.org/github.com/zhenghaoz/gorse) for detailed documentation of codes.
- Visit [Wiki](https://github.com/zhenghaoz/gorse/wiki) for tutorial, examples and high-level introduction.

## Contributors

[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/0)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/0)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/1)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/1)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/2)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/2)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/3)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/3)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/4)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/4)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/5)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/5)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/6)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/6)[![](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/images/7)](https://sourcerer.io/fame/zhenghaoz/zhenghaoz/gorse/links/7)

Any kind of contribution is expected: report a bug, give a advice or even create a pull request.

## Acknowledgments

`gorse` was inspired by following projects:

- [Guibing Guo's librec](https://github.com/guoguibing/librec)
- [Nicolas Hug's Surprise](https://github.com/NicolasHug/Surprise)
- [Golang Samples's gopher-vector](https://github.com/golang-samples/gopher-vector)

## Limitations

`gorse` has limitations and might not be applicable to some scenarios:

- **No Scalability**: Since `gorse` is a recommendation service on a single host, it's unable to handle large data. The bottleneck might be memory size or the performance of SQL database.
- **No Feature Engineering**:  `gorse` only uses interactions between items and users while features of items, users and contexts are ignored. This is not the best practice in real world.
- **Naive Policy**: There are lots of considerations on a recommender system such as the freshness of items, the variation of users' preferences ,etc. They are not included in `gorse`. 
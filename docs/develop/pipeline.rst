========
Pipeline
========

The `gorse` package tries to provide components for building a recommender system with Go. Let's get started with a simple example:

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

The simple example has almost included all key components in the `gorse` package. They are

Load Data
=========

.. code-block:: go

	data := core.LoadDataFromBuiltIn("ml-100k")


[LoadDataFromBuiltIn](https://godoc.org/github.com/zhenghaoz/gorse/core#LoadDataFromBuiltIn) loads the built-in MovieLens 100K dataset. There are several built-in datasets provided by `gorse` for testing purpose. We can load our own data from a CSV file by [LoadDataFromCSV](https://godoc.org/github.com/zhenghaoz/gorse/core#LoadDataFromCSV).

```
train, test := core.Split(data, 0.2)
```

[Split](https://godoc.org/github.com/zhenghaoz/gorse/core#Split) splits the dataset to a training dataset (80%) and a test dataset (20%), and the random seed is not set. A more complex split could be achieved by [Splitter](https://godoc.org/github.com/zhenghaoz/gorse/core#Splitter), which is used by cross validation.

Create Model
============

```
svd := model.NewSVD(base.Params{
	base.Lr:       0.007,
	base.NEpochs:  100,
	base.NFactors: 80,
	base.Reg:      0.1,
})
```

[NewSVD](https://godoc.org/github.com/zhenghaoz/gorse/model#NewSVD) creates an SVD model with hyper-parameters. All models could be found in the [model](https://godoc.org/github.com/zhenghaoz/gorse/model) package. All hyper-parameter names and enumerable values are predefined in [base](https://godoc.org/github.com/zhenghaoz/gorse/base) by [ParamName](https://godoc.org/github.com/zhenghaoz/gorse/base#ParamName) and [ParamString](https://godoc.org/github.com/zhenghaoz/gorse/base#ParamString).

Fit Model
=========

```
svd.Fit(train)
```

The model is trained by [Fit](https://godoc.org/github.com/zhenghaoz/gorse/model#SVD.Fit) method.

Evaluate Model
==============

```
core.RMSE(svd, test)
```



#### Predict

```
svd.Predict(4, 8)
```

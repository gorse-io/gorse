===============
Model Selection
===============

There are typically two evaluation approaches for recommender system: rating and ranking. 

- **[Rating](https://godoc.org/github.com/zhenghaoz/gorse/core#EvaluateRating)**: Evaluating the rating prediction task is simple. Given a test set and the well-fitted model, predictions are generated are compared to the ground truth.
- **[Ranking](https://godoc.org/github.com/zhenghaoz/gorse/core#EvaluateRank)**: Evaluating the ranking task is a little bit more complex. First, a recommendation list should be generated first for each user in the test set. **However, items in the interaction history (training data) of the user should be excluded in the recommendation list since there is no point to recommend items that users knew before.**

Cross-Validation
================

<img width=420 src="https://img.sine-x.com/cross-validation.png">

The performance of a model should be evaluated by cross-validation. There are three stages in cross-validation:

- **Split**: Dataset is divided by the [splitter](https://godoc.org/github.com/zhenghaoz/gorse/core#Splitter) to two parts: training dataset and test dataset. There are three splitters in `gorse`: k-fold splitter, ratio splitter, and user-leave-one-out splitter.
- **Fit**: Fit a recommendation [model](https://godoc.org/github.com/zhenghaoz/gorse/model) with the training dataset.
- **Evaluation**: The performance of the fitted model is evaluated by the [evaluator](https://godoc.org/github.com/zhenghaoz/gorse/core#CVEvaluator).

```go
data := core.LoadDataFromBuiltIn("ml-100k")
algo := model.NewSVDpp(base.Params{
	base.NEpochs:    100,
	base.Reg:        0.07,
	base.Lr:         0.005,
	base.NFactors:   50,
	base.InitMean:   0,
	base.InitStdDev: 0.001,
})
out := core.CrossValidate(algo, data, core.NewKFoldSplitter(5), 0,
	core.NewRatingEvaluator(core.RMSE, core.MAE))
fmt.Printf("RMSE = %.5f\n", stat.Mean(out[0].TestScore, nil))
fmt.Printf("MAE = %.5f\n", stat.Mean(out[1].TestScore, nil))
```

Parameter Search
================

*gorse* provides tools for searching best hyperparameters of models. Given a grid of candidate hyperparameters, there are two strategies to find the best combination:

- `Random Search https://godoc.org/github.com/zhenghaoz/gorse/core#RandomSearchCV)`_: :math:`n` random combinations of hyperparameters is picked and find the best combination among them.
- `Grid Search https://godoc.org/github.com/zhenghaoz/gorse/core#GridSearchCV`_: Depth-first search is performed in the hyperparameters space to find the best combination.

Apparently, random search is more efficient while grid search tends to find a better combination of hyperparameters. This is an example of searching hyperparameters for SVD++ model on MovieLens 100K:

```go
data := core.LoadDataFromBuiltIn("ml-100k")
cv := core.GridSearchCV(model.NewSVDpp(nil), data, core.ParameterGrid{
	base.NEpochs:    {100},
	base.Reg:        {0.05, 0.07},
	base.Lr:         {0.005},
	base.NFactors:   {50},
	base.InitMean:   {0},
	base.InitStdDev: {0.001},
}, core.NewKFoldSplitter(5), 0, core.NewRatingEvaluator(core.RMSE))
fmt.Printf("The best score is: %.5f\n", cv[0].BestScore)
fmt.Printf("The best params is: %v\n", cv[0].BestParams)
```

package algo

import "../data"

type Algorithm interface {
	Predict(userId int, itemId int) float64
	Fit(trainSet data.Set)
}
package core

type BPREstimator interface {
	Model
	FitBPR()
}

// BPR: Bayesian Personalized Ranking
//
// [1] Rendle, Steffen, et al. "BPR: Bayesian personalized
// ranking from implicit feedback." Proceedings of the
// twenty-fifth conference on uncertainty in artificial
// intelligence. AUAI Press, 2009.
type BPR struct {
	Model
}

// NewBPR creates a new BPR model.
func NewBPR(estimator Model) *BPR {
	bpr := new(BPR)
	bpr.Model = estimator
	return bpr
}

// Predict a rating for ranking.
func (bpr *BPR) Predict(userId, itemId int) float64 {
	return bpr.Model.Predict(userId, itemId)
}

func (bpr *BPR) Fit(set TrainSet) {

}

func (svd *SVD) FitBPR() {

}

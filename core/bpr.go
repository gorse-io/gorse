package core

type BPREstimator interface {
	Estimator
	FitBPR()
}

// BPR: Bayesian Personalized Ranking
//
// [1] Rendle, Steffen, et al. "BPR: Bayesian personalized
// ranking from implicit feedback." Proceedings of the
// twenty-fifth conference on uncertainty in artificial
// intelligence. AUAI Press, 2009.
type BPR struct {
	Estimator
}

// NewBPR creates a new BPR model.
func NewBPR(estimator Estimator) *BPR {
	bpr := new(BPR)
	bpr.Estimator = estimator
	return bpr
}

// Predict a rating for ranking.
func (bpr *BPR) Predict(userId, itemId int) float64 {
	return bpr.Estimator.Predict(userId, itemId)
}

func (bpr *BPR) Fit(set TrainSet) {

}

func (svd *SVD) FitBPR() {

}

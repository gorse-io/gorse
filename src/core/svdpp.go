// The SVD++ algorithm, an extension of SVD taking into account implicit
// ratings. The prediction \hat{r}_{ui} is set as:
//
// \hat{r}_{ui} = \mu + b_u + b_i + q_i^T\left(p_u + |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right)
//
// Where the y_j terms are a new set of item factors that capture implicit
// ratings. Here, an implicit rating describes the fact that a user u
// userHistory an item j, regardless of the rating value. If user u is unknown,
// then the bias b_u and the factors p_u are assumed to be zero. The same
// applies for item i with b_i, q_i and y_i.

package core

import (
	"github.com/gonum/floats"
	"math"
)

type SVDPP struct {
	userHistory map[int][]int     // I_u
	userFactor  map[int][]float64 // p_u
	itemFactor  map[int][]float64 // q_i
	implFactor  map[int][]float64 // y_i
	userBias    map[int]float64   // b_u
	itemBias    map[int]float64   // b_i
	globalBias  float64           // mu
	// Optimization with cache
	//cacheFailed bool
	//cacheFactor map[int][]float64 // |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right
}

func NewSVDPP() *SVDPP {
	return new(SVDPP)
}

func (pp *SVDPP) EnsembleImplFactors(userId int) ([]float64, bool) {
	history, exist := pp.userHistory[userId]
	emImpFactor := make([]float64, 0)
	// User history doesn't exist
	if !exist {
		return emImpFactor, false
	}
	// User history exists
	for _, itemId := range history {
		if len(emImpFactor) == 0 {
			// Create ensemble implicit factor
			emImpFactor = make([]float64, len(pp.implFactor[itemId]))
		}
		floats.Add(emImpFactor, pp.implFactor[itemId])
	}
	DivConst(math.Sqrt(float64(len(history))), emImpFactor)
	return emImpFactor, true
}

func (pp *SVDPP) InternalPredict(userId int, itemId int) (float64, []float64) {
	ret := .0
	userFactor, _ := pp.userFactor[userId]
	itemFactor, _ := pp.itemFactor[itemId]
	emImpFactor, _ := pp.EnsembleImplFactors(userId)
	if len(itemFactor) > 0 {
		temp := make([]float64, len(itemFactor))
		if len(userFactor) > 0 {
			floats.Add(temp, userFactor)
		}
		if len(emImpFactor) > 0 {
			floats.Add(temp, emImpFactor)
		}
		ret = floats.Dot(temp, itemFactor)
	}
	userBias, _ := pp.userBias[userId]
	itemBias, _ := pp.itemBias[itemId]
	ret += userBias + itemBias + pp.globalBias
	return ret, emImpFactor
}

func (pp *SVDPP) Predict(userId int, itemId int) float64 {
	ret, _ := pp.InternalPredict(userId, itemId)
	return ret
}

func (pp *SVDPP) Fit(trainSet TrainSet, options ...OptionSetter) {
	// Setup options
	option := Option{
		nFactors:   20,
		nEpochs:    20,
		lr:         0.007,
		reg:        0.02,
		initMean:   0,
		initStdDev: 0.1,
	}
	for _, editor := range options {
		editor(&option)
	}
	// Initialize parameters
	pp.userBias = make(map[int]float64)
	pp.itemBias = make(map[int]float64)
	pp.userFactor = make(map[int][]float64)
	pp.itemFactor = make(map[int][]float64)
	pp.implFactor = make(map[int][]float64)
	//pp.cacheFactor = make(map[int][]float64)
	for _, userId := range trainSet.Users() {
		pp.userBias[userId] = 0
		pp.userFactor[userId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	for _, itemId := range trainSet.Items() {
		pp.itemBias[itemId] = 0
		pp.itemFactor[itemId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
		pp.implFactor[itemId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	// Build user rating set
	pp.userHistory = make(map[int][]int)
	users, items, ratings := trainSet.Interactions()
	for i := 0; i < len(users); i++ {
		userId := users[i]
		itemId := items[i]
		// Create slice at first time
		if _, exist := pp.userHistory[userId]; !exist {
			pp.userHistory[userId] = make([]int, 0)
		}
		// Insert item
		pp.userHistory[userId] = append(pp.userHistory[userId], itemId)
	}
	// Create buffers
	a := make([]float64, option.nFactors)
	b := make([]float64, option.nFactors)
	// Stochastic Gradient Descent
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.Length(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := pp.userBias[userId]
			itemBias, _ := pp.itemBias[itemId]
			userFactor, _ := pp.userFactor[userId]
			itemFactor, _ := pp.itemFactor[itemId]
			// Compute error
			pred, emImpFactor := pp.InternalPredict(userId, itemId)
			diff := pred - rating
			// Update global bias
			gradGlobalBias := diff
			pp.globalBias -= option.lr * gradGlobalBias
			// Update user bias
			gradUserBias := diff + option.reg*userBias
			pp.userBias[userId] -= option.lr * gradUserBias
			// Update item bias
			gradItemBias := diff + option.reg*itemBias
			pp.itemBias[itemId] -= option.lr * gradItemBias
			// Update user latent factor
			copy(a, itemFactor)
			MulConst(diff, a)
			copy(b, userFactor)
			MulConst(option.reg, b)
			floats.Add(a, b)
			MulConst(option.lr, a)
			floats.Sub(pp.userFactor[userId], a)
			// Update item latent factor
			copy(a, userFactor)
			if len(emImpFactor) > 0 {
				floats.Add(a, emImpFactor)
			}
			MulConst(diff, a)
			copy(b, itemFactor)
			MulConst(option.reg, b)
			floats.Add(a, b)
			MulConst(option.lr, a)
			floats.Sub(pp.itemFactor[itemId], a)
			// Update implicit latent factor
			set, _ := pp.userHistory[userId]
			for _, itemId := range set {
				implFactor := pp.implFactor[itemId]
				copy(a, itemFactor)
				MulConst(diff, a)
				DivConst(math.Sqrt(float64(len(set))), a)
				copy(b, implFactor)
				MulConst(option.reg, b)
				floats.Add(a, b)
				MulConst(option.lr, a)
				floats.Sub(pp.implFactor[itemId], a)
			}
		}
	}
}

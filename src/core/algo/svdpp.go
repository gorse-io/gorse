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

package algo

import (
	"core/data"
	"github.com/gonum/floats"
	"math"
)

type SVDPP struct {
	userHistory map[int][]int     // I_u
	userFactor  map[int][]float64 // p_u
	itemFactor  map[int][]float64 // q_i
	implFactor  map[int][]float64 // y_i
	cacheFactor map[int][]float64 // |I_u|^{-\frac{1}{2}} \sum_{j \in I_u}y_j\right
	userBias    map[int]float64   // b_u
	itemBias    map[int]float64   // b_i
	globalBias  float64           // mu
}

func NewSVDPP() *SVDPP {
	return new(SVDPP)
}

func (pp *SVDPP) Predict(userId int, itemId int) float64 {
	ret := .0
	userFactor, _ := pp.userFactor[userId]
	itemFactor, _ := pp.itemFactor[itemId]
	if len(itemFactor) > 0 {
		temp := make([]float64, len(itemFactor))
		// + y_i
		history, historyAvailable := pp.userHistory[userId]
		cacheFactor, cacheAvailable := pp.cacheFactor[userId]
		if cacheAvailable {
			// Cache available
			floats.Add(temp, cacheFactor)
		} else if historyAvailable {
			// History available
			pp.cacheFactor[userId] = make([]float64, len(userFactor))
			for _, itemId := range history {
				floats.Add(pp.cacheFactor[userId], pp.implFactor[itemId])
			}
			DivConst(math.Sqrt(float64(len(history))), pp.cacheFactor[userId])
			floats.Add(temp, pp.cacheFactor[userId])
		}
		// + p_u
		if len(userFactor) > 0 {
			floats.Add(temp, userFactor)
		}
		ret = floats.Dot(itemFactor, temp)
	}
	// + b_u + b_i + mu
	userBias, _ := pp.userBias[userId]
	itemBias, _ := pp.itemBias[itemId]
	ret += userBias + itemBias + pp.globalBias
	return ret
}

func (pp *SVDPP) Fit(trainSet data.Set, options ...OptionSetter) {
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
	pp.cacheFactor = make(map[int][]float64)
	for _, userId := range trainSet.AllUsers() {
		pp.userBias[userId] = 0
		pp.userFactor[userId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	for _, itemId := range trainSet.AllItems() {
		pp.itemBias[itemId] = 0
		pp.itemFactor[itemId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
		pp.implFactor[itemId] = NewNormalVector(option.nFactors, option.initMean, option.initStdDev)
	}
	// Build user rating set
	pp.userHistory = make(map[int][]int)
	users, items, ratings := trainSet.AllInteraction()
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
	// Stochastic Gradient Descent
	buffer := make([]float64, option.nFactors)
	for epoch := 0; epoch < option.nEpochs; epoch++ {
		for i := 0; i < trainSet.NRow(); i++ {
			userId := users[i]
			itemId := items[i]
			rating := ratings[i]
			userBias, _ := pp.userBias[userId]
			itemBias, _ := pp.itemBias[itemId]
			userFactor, _ := pp.userFactor[userId]
			itemFactor, _ := pp.itemFactor[itemId]
			// Compute error
			diff := pp.Predict(userId, itemId) - rating
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
			gradUserFactor := Copy(buffer, itemFactor)
			floats.Add(MulConst(diff, gradUserFactor), MulConst(option.reg, userFactor))
			floats.Sub(pp.userFactor[userId], MulConst(option.lr, gradUserFactor))
			// Update item latent factor
			gradItemFactor := Copy(buffer, userFactor)
			floats.Add(MulConst(diff, gradItemFactor), MulConst(option.reg, itemFactor))
			floats.Sub(pp.itemFactor[itemId], MulConst(option.lr, gradItemFactor))
			// Update implicit latent factor
			set, _ := pp.userHistory[userId]
			for _, itemId := range set {
				implFactor := pp.implFactor[itemId]
				gradImplFactor := Copy(buffer, implFactor)
				MulConst(diff, gradItemFactor)
				DivConst(math.Sqrt(float64(len(set))), gradImplFactor)
				floats.Add(gradImplFactor, MulConst(option.reg, implFactor))
				floats.Sub(pp.implFactor[itemId], MulConst(option.lr, gradImplFactor))
			}
		}
	}
}

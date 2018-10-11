package core

// CLiMF: Collaborative Less-is-more Filtering
//
// [1] Shi, Yue, et al. "CLiMF: learning to maximize reciprocal
// rank with collaborative less-is-more filtering." Proceedings
// of the sixth ACM conference on Recommender systems. ACM, 2012.
type CLiMF struct {
	Base
}

func NewCLiMF() *CLiMF {
	climf := new(CLiMF)
	return climf
}

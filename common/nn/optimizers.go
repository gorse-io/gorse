package nn

type SGD struct {
	params []*Tensor
	lr     float32
}

func NewSGD(params []*Tensor, lr float32) *SGD {
	return &SGD{
		params: params,
		lr:     lr,
	}
}

func (s *SGD) Step() {
	for _, p := range s.params {
		for i := range p.data {
			p.data[i] -= s.lr * p.grad.data[i]
		}
	}
}

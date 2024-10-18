package nn

type op interface {
	forward(inputs ...*Tensor) *Tensor
	backward(dy *Tensor) []*Tensor
}

type add struct {
}

func (a *add) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.add(inputs[1])
	return y
}

func (a *add) backward(dy *Tensor) []*Tensor {
	gx0, gx1 := dy.clone(), dy.clone()
	return []*Tensor{gx0, gx1}
}

type sub struct {
}

func (s *sub) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.sub(inputs[1])
	return y
}

func (s *sub) backward(dy *Tensor) []*Tensor {
	panic("implement me")
}

type mul struct {
}

func (m *mul) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.mul(inputs[1])
	return y
}

func (m *mul) backward(dy *Tensor) []*Tensor {
	gx0, gx1 := dy.clone(), dy.clone()
	return []*Tensor{gx0, gx1}
}

type sin struct {
}

func (s *sin) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.sin()
	return y
}

func (s *sin) backward(dy *Tensor) []*Tensor {
	panic("implement me")
}

func Sin(x *Tensor) *Tensor {
	f := &sin{}
	return f.forward(x)
}

type pow struct {
	n float32
}

func (p *pow) forward(inputs ...*Tensor) *Tensor {
	y := inputs[0].clone()
	y.pow(p.n)
	return y
}

func (p *pow) backward(dy *Tensor) []*Tensor {
	panic("implement me")
}

type sum struct {
}

func (s *sum) forward(inputs ...*Tensor) *Tensor {
	x := inputs[0]
	y := NewTensor([]float32{0})
	for i := range x.data {
		y.data[0] += x.data[i]
	}
	return y
}

func (s *sum) backward(dy *Tensor) []*Tensor {
	panic("implement me")
}

// Add returns the element-wise sum of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Add(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	f := &add{}
	return f.forward(x0, x1)
}

// Sub returns the element-wise difference of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Sub(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	f := &sub{}
	return f.forward(x0, x1)
}

// Mul returns the element-wise product of two tensors. The shape of the second tensor must be a suffix sequence of the shape of the first tensor.
func Mul(x0, x1 *Tensor) *Tensor {
	if len(x0.shape) < len(x1.shape) {
		x0, x1 = x1, x0
	}
	for i := 0; i < len(x1.shape); i++ {
		if x0.shape[len(x0.shape)-len(x1.shape)+i] != x1.shape[i] {
			panic("the shape of the second tensor must be a suffix sequence of the shape of the first tensor")
		}
	}
	f := &mul{}
	return f.forward(x0, x1)
}

// Pow returns the element-wise power of a tensor.
func Pow(x *Tensor, n float32) *Tensor {
	f := &pow{n}
	return f.forward(x)
}

// Sum returns the sum of all elements in a tensor.
func Sum(x *Tensor) *Tensor {
	f := &sum{}
	return f.forward(x)
}

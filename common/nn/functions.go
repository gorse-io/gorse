package nn

type function interface {
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

func Add(x, y *Tensor) *Tensor {
	f := &add{}
	return f.forward(x, y)
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

func Mul(x0, x1 *Tensor) *Tensor {
	y := x0.clone()
	y.mul(x1)
	return y
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

func Pow(x *Tensor, n float32) *Tensor {
	y := x.clone()
	y.pow(n)
	return y
}

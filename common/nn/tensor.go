package nn

import (
	"fmt"
	"github.com/chewxy/math32"
	"math/rand"
	"strings"
)

type Tensor struct {
	data  []float32
	shape []int
	grad  *Tensor
	op    op
}

func NewTensor(data []float32, shape ...int) *Tensor {
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func NewScalar(data float32) *Tensor {
	return &Tensor{
		data:  []float32{data},
		shape: []int{},
	}
}

func LinSpace(start, end float32, shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	delta := (end - start) / float32(n-1)
	for i := range data {
		data[i] = start + delta*float32(i)
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

func RandN(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = rand.Float32()
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

// Ones creates a tensor filled with ones.
func Ones(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	for i := range data {
		data[i] = 1
	}
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

// Zeros creates a tensor filled with zeros.
func Zeros(shape ...int) *Tensor {
	n := 1
	for _, s := range shape {
		n *= s
	}
	data := make([]float32, n)
	return &Tensor{
		data:  data,
		shape: shape,
	}
}

// NoGrad creates a tensor does not require gradient.
func (t *Tensor) NoGrad() *Tensor {
	if t.op != nil {
		t.op = nil
	}
	return t
}

func (t *Tensor) String() string {
	// Print scalar value
	if len(t.shape) == 0 {
		return fmt.Sprint(t.data[0])
	}

	builder := strings.Builder{}
	builder.WriteString("[")
	if len(t.data) <= 10 {
		for i := 0; i < len(t.data); i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			if i != len(t.data)-1 {
				builder.WriteString(", ")
			}
		}
	} else {
		for i := 0; i < 5; i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			builder.WriteString(", ")
		}
		builder.WriteString("..., ")
		for i := len(t.data) - 5; i < len(t.data); i++ {
			builder.WriteString(fmt.Sprint(t.data[i]))
			if i != len(t.data)-1 {
				builder.WriteString(", ")
			}
		}
	}
	builder.WriteString("]")
	return builder.String()
}

func (t *Tensor) Backward() {
	t.grad = Ones(t.shape...)
	ops := []op{t.op}
	for len(ops) > 0 {
		op := ops[0]
		ops = ops[1:]
		inputs, output := op.inputsAndOutput()
		grads := op.backward(output.grad)
		for i := range grads {
			inputs[i].grad = grads[i]
			if inputs[i].op != nil {
				ops = append(ops, inputs[i].op)
			}
		}
	}
}

func (t *Tensor) Grad() *Tensor {
	return t.grad
}

func (t *Tensor) clone() *Tensor {
	newData := make([]float32, len(t.data))
	copy(newData, t.data)
	return &Tensor{
		data:  newData,
		shape: t.shape,
	}
}

func (t *Tensor) add(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] += other.data[i%wSize]
	}
	return t
}

func (t *Tensor) sub(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] -= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) mul(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] *= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) div(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] /= other.data[i%wSize]
	}
	return t
}

func (t *Tensor) square() *Tensor {
	for i := range t.data {
		t.data[i] = t.data[i] * t.data[i]
	}
	return t
}

func (t *Tensor) pow(other *Tensor) *Tensor {
	wSize := 1
	for i := range other.shape {
		wSize *= other.shape[i]
	}
	for i := range t.data {
		t.data[i] = math32.Pow(t.data[i], other.data[i%wSize])
	}
	return t
}

func (t *Tensor) sin() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Sin(t.data[i])
	}
	return t
}

func (t *Tensor) cos() *Tensor {
	for i := range t.data {
		t.data[i] = math32.Cos(t.data[i])
	}
	return t
}

func (t *Tensor) neg() *Tensor {
	for i := range t.data {
		t.data[i] = -t.data[i]
	}
	return t
}

func (t *Tensor) sum() float32 {
	sum := float32(0)
	for i := range t.data {
		sum += t.data[i]
	}
	return sum
}

package nn_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/nn"
	"math"
	"testing"
)

func testOptimizer(optimizerCreator func(params []*nn.Tensor, lr float32) nn.Optimizer, epochs int) (losses []float32) {
	// Create random input and output data
	x := nn.LinSpace(-math.Pi, math.Pi, 2000)
	y := nn.Sin(x)

	// Prepare the input tensor (x, x^2, x^3).
	p := nn.NewTensor([]float32{1, 2, 3}, 3)
	xx := nn.Pow(nn.Broadcast(x, 3), p)

	// Use the nn package to define our model and loss function.
	model := nn.NewSequential(
		nn.NewLinear(3, 1),
		nn.NewFlatten(),
	)

	// Use the optim package to define an Optimizer that will update the weights of
	// the model for us. Here we will use RMSprop; the optim package contains many other
	// optimization algorithms. The first argument to the RMSprop constructor tells the
	// optimizer which Tensors it should update.
	learningRate := 1e-3
	optimizer := optimizerCreator(model.Parameters(), float32(learningRate))
	for i := 0; i < epochs; i++ {
		// Forward pass: compute predicted y by passing x to the model.
		yPred := model.Forward(xx)

		// Compute and print loss
		loss := nn.MSE(yPred, y)
		losses = append(losses, loss.Data()[0])

		// Before the backward pass, use the optimizer object to zero all of the
		// gradients for the variables it will update (which are the learnable
		// weights of the model). This is because by default, gradients are
		// accumulated in buffers( i.e, not overwritten) whenever .backward()
		// is called. Checkout docs of torch.autograd.backward for more details.
		optimizer.ZeroGrad()

		// Backward pass: compute gradient of the loss with respect to model
		// parameters
		loss.Backward()

		// Calling the step function on an Optimizer makes an update to its
		// parameters
		optimizer.Step()
	}
	return
}

func TestSGD(t *testing.T) {
	losses := testOptimizer(nn.NewSGD, 1000)
	assert.IsDecreasing(t, losses)
	assert.Less(t, losses[len(losses)-1], float32(0.1))
}

func TestAdam(t *testing.T) {
	losses := testOptimizer(nn.NewAdam, 1000)
	assert.IsDecreasing(t, losses)
	assert.Less(t, losses[len(losses)-1], float32(0.1))
}

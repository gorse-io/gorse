package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/common/nn"
	"math"
)

func main() {
	// Create random input and output data
	x := nn.LinSpace(-math.Pi, math.Pi, 2000)
	y := nn.Sin(x)

	// Randomly initialize weights
	a := nn.RandN()
	b := nn.RandN()
	c := nn.RandN()
	d := nn.RandN()
	optimizer := nn.NewSGD([]*nn.Tensor{a, b, c, d}, 1e-6)

	for i := 0; i < 1000; i++ {
		// Forward pass: compute predicted y
		yPred := nn.Add(nn.Add(nn.Add(nn.Mul(a, x), nn.Mul(b, x)), nn.Mul(c, nn.Square(x))), nn.Mul(d, nn.Pow(x, nn.NewScalar(3))))

		// Compute and print loss
		loss := nn.Sum(nn.Square(nn.Sub(yPred, y)))
		if i%100 == 99 {
			fmt.Println(i, loss)
		}

		// Backward pass: compute gradient of the loss with respect to model parameters
		loss.Backward()

		// Calling the step function on an Optimizer makes an update to its parameters
		optimizer.Step()
	}

	fmt.Println("Result: y =", a, "+", b, "x +", c, "x^2 +", d, "x^3")
}

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

	for i := 0; i < 1000; i++ {
		// Forward pass: compute predicted y
		yPred := nn.Add(nn.Add(nn.Add(nn.Mul(a, x), nn.Mul(b, nn.Pow(x, 1))), nn.Mul(c, nn.Pow(x, 2))), nn.Mul(d, nn.Pow(x, 3)))

		// Compute and print loss
		loss := nn.Sum(nn.Pow(nn.Sub(yPred, y), 2))
		if i%100 == 99 {
			fmt.Println(i, loss)
		}

		loss.Backward()

		// Update weights using gradient descent
		learningRate := nn.NewTensor([]float32{1e-6})
		a = nn.Sub(a, nn.Mul(learningRate, a.Grad())).NoGrad()
		b = nn.Sub(b, nn.Mul(learningRate, b.Grad())).NoGrad()
		c = nn.Sub(c, nn.Mul(learningRate, c.Grad())).NoGrad()
		d = nn.Sub(d, nn.Mul(learningRate, d.Grad())).NoGrad()
	}

	fmt.Println("Result: y =", a, "+", b, "x +", c, "x^2 +", d, "x^3")
}

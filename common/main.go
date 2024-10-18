package main

import (
	"fmt"
	"github.com/zhenghaoz/gorse/common/nn"
	"math"
)

func main() {
	/*


	   learning_rate = 1e-6
	   for t in range(2000):
	       # Forward pass: compute predicted y
	       y_pred = a + b * x + c * x ** 2 + d * x ** 3

	       # Compute and print loss
	       loss = (y_pred - y).pow(2).sum().item()
	       if t % 100 == 99:
	           print(t, loss)

	       # Backprop to compute gradients of a, b, c, d with respect to loss
	       grad_y_pred = 2.0 * (y_pred - y)
	       grad_a = grad_y_pred.sum()
	       grad_b = (grad_y_pred * x).sum()
	       grad_c = (grad_y_pred * x ** 2).sum()
	       grad_d = (grad_y_pred * x ** 3).sum()

	       # Update weights using gradient descent
	       a -= learning_rate * grad_a
	       b -= learning_rate * grad_b
	       c -= learning_rate * grad_c
	       d -= learning_rate * grad_d


	   print(f'Result: y = {a.item()} + {b.item()} x + {c.item()} x^2 + {d.item()} x^3')
	*/

	// Create random input and output data
	x := nn.LinSpace(-math.Pi, math.Pi, 2000)
	y := nn.Sin(x)

	// Randomly initialize weights
	a := nn.RandN()
	b := nn.RandN()
	c := nn.RandN()
	d := nn.RandN()

	for i := 0; i < 2000; i++ {
		// Forward pass: compute predicted y
		yPred := nn.Add(nn.Add(nn.Add(nn.Mul(a, x), nn.Mul(b, nn.Pow(x, 1))), nn.Mul(c, nn.Pow(x, 2))), nn.Mul(d, nn.Pow(x, 3)))

		// Compute and print loss
		if i%100 == 99 {
			loss := nn.Sum(nn.Pow(nn.Sub(yPred, y), 2))
			fmt.Println(i, loss)
		}

		// Backprop to compute gradients of a, b, c, d with respect to loss
		gradYPred := nn.Mul(nn.NewTensor([]float32{2}), nn.Sub(yPred, y))
		gradA := nn.Sum(gradYPred)
		gradB := nn.Sum(nn.Mul(gradYPred, x))
		gradC := nn.Sum(nn.Mul(gradYPred, nn.Pow(x, 2)))
		gradD := nn.Sum(nn.Mul(gradYPred, nn.Pow(x, 3)))

		// Update weights using gradient descent
		learningRate := nn.NewTensor([]float32{1e-6})
		a = nn.Sub(a, nn.Mul(learningRate, gradA))
		b = nn.Sub(b, nn.Mul(learningRate, gradB))
		c = nn.Sub(c, nn.Mul(learningRate, gradC))
		d = nn.Sub(d, nn.Mul(learningRate, gradD))
	}

	fmt.Println("Result: y =", a, "+", b, "x +", c, "x^2 +", d, "x^3")
}

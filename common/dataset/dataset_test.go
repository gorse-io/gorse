package dataset

import (
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/zhenghaoz/gorse/common/nn"
	"testing"
)

func TestIris(t *testing.T) {
	data, target, err := LoadIris()
	assert.NoError(t, err)
	_ = data
	_ = target

	x := nn.NewTensor(lo.Flatten(data), len(data), 4)

	model := nn.NewSequential(
		nn.NewLinear(4, 100),
		nn.NewReLU(),
		nn.NewLinear(100, 100),
		nn.NewLinear(100, 3),
		nn.NewFlatten(),
	)
	_ = model
}

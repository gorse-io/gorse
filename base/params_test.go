package base

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParams_Copy(t *testing.T) {
	// Create parameters
	a := Params{
		NFactors:    1,
		Lr:          0.1,
		KNNType:     Baseline,
		RandomState: 0,
		UserBased:   true,
	}
	// Create copy
	b := a.Copy()
	b[NFactors] = 2
	b[Lr] = 0.2
	b[KNNType] = Basic
	b[RandomState] = 1
	b[UserBased] = false
	// Check original parameters
	assert.Equal(t, 1, a.GetInt(NFactors, -1))
	assert.Equal(t, 0.1, a.GetFloat64(Lr, -0.1))
	assert.Equal(t, Baseline, a.GetString(KNNType, ""))
	assert.Equal(t, int64(0), a.GetInt64(RandomState, -1))
	assert.Equal(t, true, a.GetBool(UserBased, false))
	// Check copy parameters
	assert.Equal(t, 2, b.GetInt(NFactors, -1))
	assert.Equal(t, 0.2, b.GetFloat64(Lr, -0.1))
	assert.Equal(t, Basic, b.GetString(KNNType, ""))
	assert.Equal(t, int64(1), b.GetInt64(RandomState, -1))
	assert.Equal(t, false, b.GetBool(UserBased, true))
}

func TestParams_Merge(t *testing.T) {
	// Create a group of parameters
	a := Params{
		Lr:  0.1,
		Reg: 0.2,
	}
	// Create another group of parameters
	b := Params{
		Reg:   0.3,
		Alpha: 0.4,
	}
	// Merge
	a.Merge(b)
	// Check
	assert.Equal(t, 0.1, a.GetFloat64(Lr, -1))
	assert.Equal(t, 0.3, a.GetFloat64(Reg, -1))
	assert.Equal(t, 0.4, a.GetFloat64(Alpha, -1))
}

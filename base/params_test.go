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
		Type:        Baseline,
		RandomState: 0,
		UserBased:   true,
	}
	// Create copy
	b := a.Copy()
	b[NFactors] = 2
	b[Lr] = 0.2
	b[Type] = Basic
	b[RandomState] = 1
	b[UserBased] = false
	// Check original parameters
	assert.Equal(t, 1, a.GetInt(NFactors, -1))
	assert.Equal(t, 0.1, a.GetFloat64(Lr, -0.1))
	assert.Equal(t, Baseline, a.GetString(Type, ""))
	assert.Equal(t, int64(0), a.GetInt64(RandomState, -1))
	assert.Equal(t, true, a.GetBool(UserBased, false))
	// Check copy parameters
	assert.Equal(t, 2, b.GetInt(NFactors, -1))
	assert.Equal(t, 0.2, b.GetFloat64(Lr, -0.1))
	assert.Equal(t, Basic, b.GetString(Type, ""))
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

func TestParams_GetBool(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, true, p.GetBool(UserBased, true))
	// Normal case
	p[UserBased] = false
	assert.Equal(t, false, p.GetBool(UserBased, true))
	// Wrong type case
	p[UserBased] = 1
	assert.Equal(t, true, p.GetBool(UserBased, true))
}

func TestParams_GetFloat64(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, 0.1, p.GetFloat64(Lr, 0.1))
	// Normal case
	p[Lr] = 1.0
	assert.Equal(t, 1.0, p.GetFloat64(Lr, 0.1))
	// Wrong type case
	p[Lr] = 1
	assert.Equal(t, 1.0, p.GetFloat64(Lr, 0.1))
	p[Lr] = "hello"
	assert.Equal(t, 0.1, p.GetFloat64(Lr, 0.1))
}

func TestParams_GetInt(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, -1, p.GetInt(NFactors, -1))
	// Normal case
	p[NFactors] = 0
	assert.Equal(t, 0, p.GetInt(NFactors, -1))
	// Wrong type case
	p[NFactors] = "hello"
	assert.Equal(t, -1, p.GetInt(NFactors, -1))
}

func TestParams_GetInt64(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, int64(-1), p.GetInt64(RandomState, -1))
	// Normal case
	p[RandomState] = int64(0)
	assert.Equal(t, int64(0), p.GetInt64(RandomState, -1))
	// Wrong type case
	p[RandomState] = 0
	assert.Equal(t, int64(0), p.GetInt64(RandomState, -1))
	p[RandomState] = "hello"
	assert.Equal(t, int64(-1), p.GetInt64(RandomState, -1))
}

func TestParams_GetString(t *testing.T) {
	p := Params{}
	// Empty case
	assert.Equal(t, Cosine, p.GetString(Similarity, Cosine))
	// Normal case
	p[Similarity] = MSD
	assert.Equal(t, MSD, p.GetString(Similarity, Cosine))
	// Wrong type case
	p[Similarity] = 1
	assert.Equal(t, Cosine, p.GetString(Similarity, Cosine))
}

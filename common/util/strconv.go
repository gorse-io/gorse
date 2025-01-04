package util

import (
	"golang.org/x/exp/constraints"
	"strconv"
)

func ParseFloat[T constraints.Float](s string) (T, error) {
	v, err := strconv.ParseFloat(s, 32)
	return T(v), err
}

func ParseUInt[T constraints.Unsigned](s string) (T, error) {
	v, err := strconv.ParseUint(s, 10, 8)
	return T(v), err
}

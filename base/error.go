package base

import "log"

func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func MustInt(val int, err error) int {
	if err != nil {
		log.Fatal(err)
	}
	return val
}

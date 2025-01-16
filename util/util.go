package util

import (
	"math/rand"
)

func ShuffleSlices(slices [][]interface{}) [][]interface{} {
	newSlices := make([][]interface{}, len(slices))
	l := len(slices[0])
	for i, s := range slices {
		if len(s) != l {
			panic("slices must all be the same length")
		}
		newSlices[i] = make([]interface{}, len(s))
	}
	for j := 0; len(slices[0]) > 0; j++ {
		randIdx := rand.Int() % len(slices[0])
		for i := range slices {
			s := &slices[i]
			newSlices[i][j] = (*s)[randIdx]
			Delete(s, randIdx)
		}
	}
	return newSlices
}

func ShuffleSlice(s []interface{}) []interface{} {
	newS := make([]interface{}, len(s))
	for i := 0; len(s) > 0; i++ {
		randIdx := rand.Int() % len(s)
		newS[i] =  s[randIdx]
		Delete(&s, randIdx)
	}
	return newS
}

func Delete(s *[]interface{}, i int) {
	(*s)[i] = (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
}

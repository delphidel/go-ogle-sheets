package util

import (
	"testing"
)

func TestShuffleSlices(t *testing.T) {
	count := 4
	sliceLen := 1000
	for try := range 100 {
		s := make([][]interface{}, count)
		for i := range count {
			s[i] = make([]interface{}, sliceLen)
			for j := range sliceLen {
				s[i][j] = j
			}
		}
		actual := ShuffleSlices(s)
		if len(actual) != count {
			t.Fatalf("Wrong number of slices; expected %v, got %v", count, len(actual))
		}
		allEqualOriginal := true
		for i := range count {
			if len(actual[i]) != sliceLen {
				t.Fatalf("At least one slice is the wrong length; expected %v, got %v", sliceLen, len(actual[i]))
			}
		}

		for j := range sliceLen {
			expected := actual[0][j]
			for i := range count {
				if actual[i][j] != expected {
					t.Fatalf("Slices were shuffled differently! slice 0 has %v at %d, but slice %d has %v", expected, j, i, actual[i][j])
				}
				if actual[i][j] != j {
					allEqualOriginal = false
				}
			}
		}
		if allEqualOriginal {
			t.Fatalf("On attempt %d, all values are equal to the original slide. This should be unlikely!", try)
		}
	}
}

func TestShuffle(t *testing.T) {
	s := make([]interface{}, 10000)
	for i := range 10000 {
		s[i] = i+1
	}
	for i := range 1000 {
		actual := ShuffleSlice(s)
		if len(actual) != len(s) {
			t.Fatalf("Wrong length; expected %v, got %v", len(s), len(actual))
		}
		var allEqual = true
		for i := range s {
			if s[i] != actual[i] {
				allEqual = false
			}
		}
		if allEqual {
			t.Fatalf("On attempt %d, all shuffled elements are equal to original! This should be unlikely.", i)
		}
	}
}


type deleteTestConf struct {
	given []interface{}
	arg int
	expected []interface{}
}
var deleteTests = []deleteTestConf{
	{
		[]interface{}{1, 2, 3, 4, 5, 6},
		2,
		[]interface{}{1, 2, 6, 4, 5},
	},
	{
		[]interface{}{1, 2, 3, 4, 5, 6},
		5,
		[]interface{}{1, 2, 3, 4, 5},
	},
	{
		[]interface{}{1},
		0,
		[]interface{}{},
	},
}
func TestDelete(t *testing.T) {
	for _, test := range deleteTests {
		// Love to modify the test config objects 😬
		s := test.given
		Delete(&s, test.arg)
		if len(s) != len(test.expected) {
			t.Fatalf("Wrong length; expected %v, got %v", len(test.expected), len(s))
		}
		for i := range test.expected {
			if s[i] != test.expected[i] {
				t.Fatalf("Wrong value at index %d; expected %v, got %v", i, test.expected[i], s[i])
			}
		}
	}
}

package routers

import (
	"reflect"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Check and ensure that all elements exists in another slice and
// check if the length of the slices are equal.
func isEqual(aa, bb []Part) bool {
	eqCtr := 0
	for _, a := range aa {
		for _, b := range bb {
			if reflect.DeepEqual(a, b) {
				eqCtr++
			}
		}
	}
	if eqCtr != len(bb) || len(aa) != len(bb) {
		return false
	}

	return true
}

func TestEqual(t *testing.T) {
	m1 := []Part{
		{
			Idx:  1,
			Etag: "c4bd2035a97cd20d62240cb5b729c24a",
		},
		{
			Idx:  2,
			Etag: "006d901fd8ddc366d25a02150b956993",
		},
		{
			Idx:  3,
			Etag: "4894ef833bbbabcfd174a8139a135c2c",
		},
	}
	m2 := []Part{
		{
			Idx:  2,
			Etag: "006d901fd8ddc366d25a02150b956993",
		},
		{
			Idx:  1,
			Etag: "c4bd2035a97cd20d62240cb5b729c24a",
		},

		{
			Idx:  3,
			Etag: "4894ef833bbbabcfd174a8139a135c2c",
		},
	}

	// This Transformer sorts a []int.
	trans := cmp.Transformer("Sort", func(in []Part) []Part {
		out := append([]Part(nil), in...) // Copy input to avoid mutating it
		// sort.Sort(data sort.Interface)
		sort.SliceStable(out, func(i, j int) bool { return out[i].Idx < out[j].Idx })

		return out
	})
	t.Log(reflect.DeepEqual(m1, m2))
	t.Log(isEqual(m1, m2))
	t.Log(cmp.Equal(m1, m2, trans))
}

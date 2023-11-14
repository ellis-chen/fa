package fsutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type dummyStruct struct {
	Elem1 string `json:"elem1"`
	Elem2 string `json:"elem2"`
	Elem3 string `json:"elem3"`
}

func TestSortRule(t *testing.T) {
	fileObj := dummyStruct{
		Elem1: "a",
		Elem2: "b",
		Elem3: "c",
	}
	err := ValidateSort(&fileObj, Sortable{Sort: "elem1", Asc: false})
	require.NoError(t, err)
	err = ValidateSort(&fileObj, Sortable{Sort: "faketag", Asc: false})
	require.Error(t, err)
	require.EqualError(t, err, ErrNoSuchSort.Error())
}

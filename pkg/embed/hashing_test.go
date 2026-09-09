package embed

import (
	"reflect"
	"testing"
)

func TestHashingSimilarWording(t *testing.T) {
	t.Parallel()

	vecs, err := Hashing{}.Embed([]string{
		"cats sleep",
		"cats eat",
		"quantum chromodynamics",
	})
	if err != nil {
		t.Fatal(err)
	}
	if Cosine(vecs[0], vecs[1]) < Cosine(vecs[0], vecs[2]) {
		t.Fatalf("overlapping words should be closer: %v %v %v",
			Cosine(vecs[0], vecs[1]), Cosine(vecs[0], vecs[2]), vecs)
	}
}

func TestHashingDeterministic(t *testing.T) {
	t.Parallel()

	a, err := Hashing{}.Embed([]string{"hello world"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hashing{}.Embed([]string{"hello world"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a[0], b[0]) {
		t.Fatalf("same input must be identical")
	}
}

func TestHashingBatchLen(t *testing.T) {
	t.Parallel()

	vecs, err := Hashing{}.Embed([]string{"a", "", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 3 || len(vecs[1]) != hashingDim {
		t.Fatalf("got %d vecs", len(vecs))
	}
}

package embed

import (
	"math"
	"reflect"
	"testing"
)

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

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
	if cosine(vecs[0], vecs[1]) < cosine(vecs[0], vecs[2]) {
		t.Fatalf("overlapping words should be closer: %v %v %v",
			cosine(vecs[0], vecs[1]), cosine(vecs[0], vecs[2]), vecs)
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

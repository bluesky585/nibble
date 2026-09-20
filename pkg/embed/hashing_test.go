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
	if Cosine32(vecs[0], vecs[1]) < Cosine32(vecs[0], vecs[2]) {
		t.Fatalf("overlapping words should be closer: %v %v %v",
			Cosine32(vecs[0], vecs[1]), Cosine32(vecs[0], vecs[2]), vecs)
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

// CJK text must embed by bigram, not by whole-sentence field: two texts
// sharing a phrase should land closer than two that share nothing,
// which a whitespace split cannot deliver (each sentence hashes to one
// slot, and unrelated sentences collide or separate at random).
func TestHashingCJKBigrams(t *testing.T) {
	t.Parallel()

	vecs, err := Hashing{}.Embed([]string{
		"大模型用于检索增强",
		"检索增强生成技术",
		"今天天气很好",
	})
	if err != nil {
		t.Fatal(err)
	}
	shared := Cosine32(vecs[0], vecs[1]) // shares 检索 增强
	apart := Cosine32(vecs[0], vecs[2])  // shares nothing
	if shared <= apart {
		t.Fatalf("shared bigrams should be closer: shared=%v apart=%v", shared, apart)
	}
	if shared == 0 {
		t.Fatalf("shared bigrams scored 0: bigram hashing is not reaching the same slots")
	}
}

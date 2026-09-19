package embed

import "math"

// Cosine returns the cosine similarity of a and b.
// A zero or mismatched vector yields 0.
func Cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
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

// Cosine32 is Cosine over float32 vectors. Embeddings are stored as
// float32 (the SQLite store quantizes on write), so scoring in float32
// is the same math over what the index actually kept — and it moves
// through half the bytes, which is most of a linear scan's cost. The
// accumulator stays float64: a float32 sum over a 1536-dimension dot
// product loses real precision, while the multiply itself does not.
// Ranking differences against Cosine sit far below the gaps between
// neighbors, as the float32 storage already accepted.
func Cosine32(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	// Four accumulators per quantity: a single accumulator serializes on
	// its own add (each step waits for the last), which is what bounds
	// the scan rather than memory bandwidth. Four lanes break that chain
	// and let the compiler keep multiplies of adjacent lanes in flight.
	// The f64 rescale every 64 elements bounds the drift a long float32
	// dot product would otherwise pick up.
	var dot, na, nb float64
	var d0, d1, d2, d3, a0, a1, a2, a3, b0, b1, b2, b3 float32
	n := len(a)
	i := 0
	for ; i+64 <= n; i += 64 {
		for j := i; j < i+64; j += 4 {
			d0 += a[j] * b[j]
			d1 += a[j+1] * b[j+1]
			d2 += a[j+2] * b[j+2]
			d3 += a[j+3] * b[j+3]
			a0 += a[j] * a[j]
			a1 += a[j+1] * a[j+1]
			a2 += a[j+2] * a[j+2]
			a3 += a[j+3] * a[j+3]
			b0 += b[j] * b[j]
			b1 += b[j+1] * b[j+1]
			b2 += b[j+2] * b[j+2]
			b3 += b[j+3] * b[j+3]
		}
		dot += float64(d0 + d1 + d2 + d3)
		na += float64(a0 + a1 + a2 + a3)
		nb += float64(b0 + b1 + b2 + b3)
		d0, d1, d2, d3 = 0, 0, 0, 0
		a0, a1, a2, a3 = 0, 0, 0, 0
		b0, b1, b2, b3 = 0, 0, 0, 0
	}
	// The tail holds the remainders of every lane, one element at a time.
	for ; i < n; i++ {
		d0 += a[i] * b[i]
		a0 += a[i] * a[i]
		b0 += b[i] * b[i]
	}
	dot += float64(d0)
	na += float64(a0)
	nb += float64(b0)
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

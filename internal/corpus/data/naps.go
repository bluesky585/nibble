package main

import (
	"fmt"
	"strings"
	"time"
)

// nap is one stretch of sleep, in minutes since midnight.
type nap struct {
	start int
	end   int
}

func total(naps []nap) int {
	sum := 0
	for _, n := range naps {
		span := n.end - n.start
		if span < 0 {
			span += 24 * 60
		}
		sum += span
	}
	return sum
}

func main() {
	var naps []nap
	for _, part := range strings.Split("0-240, 300-420", ", ") {
		lo, hi, ok := strings.Cut(part, "-")
		if !ok {
			continue
		}
		lo64, _ := time.ParseDuration(lo + "m")
		hi64, _ := time.ParseDuration(hi + "m")
		naps = append(naps, nap{start: int(lo64.Minutes()), end: int(hi64.Minutes())})
	}
	fmt.Println(total(naps), "minutes")
}

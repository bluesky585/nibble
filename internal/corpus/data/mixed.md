# Sleeping Habits of an <em>Ordinary</em> House Cat

Cats sleep for most of the day. A healthy adult cat sleeps between twelve and
sixteen hours in a twenty-four hour period, and the pattern is not one long
sleep but many short ones. The animal is crepuscular, which is a polite way of
saying it is most awake when you are not.

## The Nap Schedule

A nap is not a lesser sleep. Cats cycle through the same stages we do, and they
reach deep sleep faster, which is why a cat that has been asleep for ten
minutes can be harder to wake than a person who has been asleep for an hour.

Here is how the hours tend to fall for an indoor cat on a normal day:

| Hours | State        | Where            | Notes                       |
| ----- | ------------ | ---------------- | --------------------------- |
| 0-4   | Deep sleep   | Bed, under duvet | Twitching paws, no response |
| 4-9   | Light sleep  | Sofa arm         | Wakes at any door           |
| 9-13  | Dozing       | Any warm surface | Eyes closed, ears moving    |
| 13-17 | Alert rest   | Window sill      | Watching birds, tail flicks |
| 17-21 | Active       | Whole house      | Zooming, knocking things    |
| 21-24 | Wind-down    | Whoever is warm  | Purring, kneading           |

The totals matter less than the shape. A cat that sleeps sixteen hours in one
block is often unwell; a cat that sleeps sixteen hours in nine blocks is
ordinary.

## 為什麼睡那麼多

The energy story is the usual explanation, and it is only half right. A cat's
hunting style is a short burst followed by a long wait, so the body is built to
recover quickly from a sprint and to idle cheaply in between. Sleep is the idle.

捕猎需要爆发力。猫的肌肉适合短距离冲刺，而不是长跑。一次成功的捕猎往往只持续几秒，
但在此之前可能需要伏击很久。所以身体把大部分时间留给恢复，把少数时间留给爆发。

The other half is temperature. A cat's comfortable range is narrow and warm, and
sleeping curled is how it holds heat without spending energy on shivering.
🙂 That is also why the best sleeping spot in any house is the one a laptop has
just left.

## A Short Program

The numbers above are easy to check. This reads a day of nap records and prints
each cat's total, along with the longest stretch it slept without waking:

```python
from dataclasses import dataclass


@dataclass
class Nap:
    """One stretch of sleep, in minutes since midnight."""
    start: int
    end: int


def naps(records):
    # A record of "0-240, 300-420" becomes two Naps.
    out = []
    for part in records.split(","):
        lo, hi = part.split("-")
        out.append(Nap(int(lo), int(hi)))
    return out


def longest_stretch(spans):
    best = 0
    for n in spans:
        # end can be before start only across midnight.
        span = n.end - n.start
        if span < 0:
            span += 24 * 60
        best = max(best, span)
    return best


def report(name, records):
    spans = naps(records)
    total = sum(longest_stretch([n]) if n.end >= n.start else n.end + 24 * 60 - n.start
                for n in spans)
    print(f"{name}: {total} minutes asleep, longest stretch {longest_stretch(spans)}")


if __name__ == "__main__":
    report("Ada", "0-240, 300-420, 540-780, 1020-1260")
```

Go reads the same data with a parser instead of a scanner, so a declaration's
span is exact to the byte:

```go
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
```

## What to Watch For

A sudden change in sleep is the useful signal, not the baseline. A cat that
stops sleeping in its usual spots, or starts sleeping in a closet, is telling
you something before it shows any other sign.

If the change comes with a limp, a droop, or a refusal to eat, it is a
veterinary question. If it comes with a new piece of furniture, it is usually a
furniture question.

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

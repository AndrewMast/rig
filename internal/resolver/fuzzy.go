package resolver

import "strings"

// subseqScore scores how well token matches candidate as a case-insensitive
// subsequence. It returns 0 when token is not a subsequence at all. Ranking
// preference is prefix > contiguous substring > scattered, with a compactness
// tie-breaker so tighter matches win.
func subseqScore(token, candidate string) int {
	if token == "" {
		return 0
	}
	t := strings.ToLower(token)
	c := strings.ToLower(candidate)
	first, last, ok := matchSpan(t, c)
	if !ok {
		return 0
	}
	switch {
	case strings.HasPrefix(c, t):
		return 100 + lenBonus(t, c)
	case strings.Contains(c, t):
		return 50 + lenBonus(t, c)
	default:
		span := last - first + 1
		// Tighter spans (closer to len(t)) score higher.
		return 10 + (len(c) - span)
	}
}

// matchSpan greedily matches t as a subsequence of c, returning the index of the
// first and last matched runes. ok is false when t is not a subsequence.
func matchSpan(t, c string) (first, last int, ok bool) {
	first, last = -1, -1
	ti := 0
	for ci := 0; ci < len(c) && ti < len(t); ci++ {
		if c[ci] == t[ti] {
			if first < 0 {
				first = ci
			}
			last = ci
			ti++
		}
	}
	return first, last, ti == len(t)
}

// lenBonus rewards candidates whose length is close to the token's.
func lenBonus(t, c string) int {
	return 2*len(t) - len(c)
}

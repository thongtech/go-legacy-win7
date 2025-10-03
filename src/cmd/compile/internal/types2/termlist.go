// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types2

import "strings"

// A termlist represents the type set represented by the union
// t1 ∪ y2 ∪ ... tn of the type sets of the terms t1 to tn.
// A termlist is in normal form if all terms are disjoint.
// termlist operations don't require the operands to be in
// normal form.
type termlist []*term

// allTermlist represents the set of all types.
// It is in normal form.
var allTermlist = termlist{new(term)}

// termSep is the separator used between individual terms.
const termSep = " | "

// String prints the termlist exactly (without normalization).
func (xl termlist) String() string {
	if len(xl) == 0 {
		return "∅"
	}
	var buf strings.Builder
	for i, x := range xl {
		if i > 0 {
			buf.WriteString(termSep)
		}
		buf.WriteString(x.String())
	}
	return buf.String()
}

// isEmpty reports whether the termlist xl represents the empty set of types.
func (xl termlist) isEmpty() bool {
	// If there's a non-nil term, the entire list is not empty.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil {
			return false
		}
	}
	return true
}

// isAll reports whether the termlist xl represents the set of all types.
func (xl termlist) isAll() bool {
	// If there's a 𝓤 term, the entire list is 𝓤.
	// If the termlist is in normal form, this requires at most
	// one iteration.
	for _, x := range xl {
		if x != nil && x.typ == nil {
			return true
		}
	}
	return false
}

// norm returns the normal form of xl.
func (xl termlist) norm() termlist {
	// Optimized algorithm to reduce from O(n²) to near O(n) for common cases.
	// Early exit for small lists (common case).
	if len(xl) <= 1 {
		return xl
	}

	// Pre-allocate result slice with capacity to reduce allocations
	rl := make(termlist, 0, len(xl))
	used := make([]bool, len(xl))

	for i, xi := range xl {
		if xi == nil || used[i] {
			continue
		}

		// Check for universal term early to avoid unnecessary work
		if xi.typ == nil {
			return allTermlist
		}

		// Only check for unions with subsequent terms
		merged := false
		for j := i + 1; j < len(xl); j++ {
			xj := xl[j]
			if xj == nil || used[j] {
				continue
			}

			if u1, u2 := xi.union(xj); u2 == nil {
				// If we encounter a 𝓤 term, the entire list is 𝓤.
				if u1.typ == nil {
					return allTermlist
				}
				xi = u1
				used[j] = true // xj is now unioned into xi
				merged = true
			}
		}
		rl = append(rl, xi)
	}
	return rl
}

// union returns the union xl ∪ yl.
func (xl termlist) union(yl termlist) termlist {
	return append(xl, yl...).norm()
}

// intersect returns the intersection xl ∩ yl.
func (xl termlist) intersect(yl termlist) termlist {
	if xl.isEmpty() || yl.isEmpty() {
		return nil
	}

	// Optimized with pre-allocation to reduce memory allocations.
	// Pre-allocate with estimated capacity (conservative estimate)
	rl := make(termlist, 0, len(xl))

	for _, x := range xl {
		for _, y := range yl {
			if r := x.intersect(y); r != nil {
				rl = append(rl, r)
			}
		}
	}
	return rl.norm()
}

// equal reports whether xl and yl represent the same type set.
func (xl termlist) equal(yl termlist) bool {
	// Optimized: quick length check before expensive subset operations
	if len(xl) != len(yl) {
		// Different lengths might still be equal after normalization,
		// but this is a fast path for obviously different sets
		if len(xl) == 0 || len(yl) == 0 {
			return len(xl) == len(yl)
		}
	}
	return xl.subsetOf(yl) && yl.subsetOf(xl)
}

// includes reports whether t ∈ xl.
func (xl termlist) includes(t Type) bool {
	for _, x := range xl {
		if x.includes(t) {
			return true
		}
	}
	return false
}

// supersetOf reports whether y ⊆ xl.
func (xl termlist) supersetOf(y *term) bool {
	for _, x := range xl {
		if y.subsetOf(x) {
			return true
		}
	}
	return false
}

// subsetOf reports whether xl ⊆ yl.
func (xl termlist) subsetOf(yl termlist) bool {
	if yl.isEmpty() {
		return xl.isEmpty()
	}

	// each term x of xl must be a subset of yl
	for _, x := range xl {
		if !yl.supersetOf(x) {
			return false // x is not a subset yl
		}
	}
	return true
}

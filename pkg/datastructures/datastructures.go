package datastructures

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrNotFound = errors.New("not found in Index")

type Index[T any] map[string]T

func NewIndex[T any](items []T, getID func(T) string) Index[T] {
	idx := make(Index[T], len(items))
	for _, item := range items {
		idx[getID(item)] = item
	}
	return idx
}

func Get[T any](idx Index[T], key string) (T, error) {
	if found, ok := idx[key]; ok {
		return found, nil
	}
	var zero T
	return zero, ErrNotFound
}

func Keys[T any](idx Index[T]) []string {
	keys := make([]string, len(idx))
	i := 0
	for k := range idx {
		keys[i] = k
		i++
	}
	return keys
}

func AllExists[T any](idx Index[T], keys ...string) bool {
	for _, id := range keys {
		if _, ok := idx[id]; !ok {
			return false
		}
	}
	return true
}

func Has[T any](idx Index[T], key string) bool {
	return AllExists(idx, key)
}

func Filter[T any](idx Index[T], keys ...string) Index[T] {
	filterIdx := NewIndex[string](keys, func(s string) string {
		return s
	})
	newIdx := make(Index[T], 0)
	for k, v := range idx {
		if _, ok := filterIdx[k]; ok {
			newIdx[k] = v
		}
	}
	return newIdx
}

type DiffResult struct {
	Added   []string
	Deleted []string
	Rest    []string
}

func (d DiffResult) String() string {
	return fmt.Sprintf("{added: %v, deleted: %v, rest: %v}", d.Added, d.Deleted, d.Rest)
}

func Diff[C, P any](current Index[C], previous Index[P]) DiffResult {
	processed := make(map[string]struct{})
	var added, deleted, rest []string

	for k := range current {
		if _, ok := previous[k]; !ok {
			added = append(added, k)
		} else {
			processed[k] = struct{}{}
		}
	}
	for k := range previous {
		if _, ok := current[k]; !ok {
			deleted = append(deleted, k)
		} else {
			processed[k] = struct{}{}
		}
	}
	for k := range processed {
		rest = append(rest, k)
	}

	sort.Strings(added)
	sort.Strings(deleted)
	sort.Strings(rest)
	return DiffResult{
		Added:   added,
		Deleted: deleted,
		Rest:    rest,
	}
}

func Merge[T any](slices ...[]T) []T {
	var result []T
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

func Unique[T comparable](elements ...T) []T {
	index := make(map[T]struct{}, 0)
	var result []T

	for _, e := range elements {
		if _, found := index[e]; !found {
			index[e] = struct{}{}
			result = append(result, e)
		}
	}

	return result
}

// repeatableFlags are CLI flags whose mariadb-dump / mariadb-binlog
// semantics let the user pass the option multiple times, each value
// adding to the previous one rather than overriding it. UniqueArgs must
// keep every occurrence of these flags; dropping all but the last would
// silently lose user-supplied entries (see issue #1691).
var repeatableFlags = map[string]struct{}{
	"--ignore-table":      {},
	"--ignore-table-data": {},
	"--ignore-databases":  {},
	"--ignore-tables":     {},
}

// UniqueArgs returns unique CLI arguments with smart deduplication:
//   - For repeatable flags (e.g. --ignore-table) every occurrence is
//     kept regardless of value, only exact-string duplicates are folded.
//   - For other exact duplicates (same string), keeps the first
//     occurrence to preserve order.
//   - For flag name conflicts with different values (e.g., --flag vs
//     --flag=value), keeps the last occurrence so user-specified args
//     can override defaults.
func UniqueArgs(args ...string) []string {
	type occurrence struct {
		index int
		arg   string
	}

	// Group args by flag name
	flagOccurrences := make(map[string][]occurrence)
	for i, arg := range args {
		flagName := extractFlagName(arg)
		flagOccurrences[flagName] = append(flagOccurrences[flagName], occurrence{i, arg})
	}

	// Determine which index to keep for each flag
	keepIndex := make(map[int]bool)
	for flagName, occurrences := range flagOccurrences {
		if len(occurrences) == 1 {
			keepIndex[occurrences[0].index] = true
			continue
		}

		if _, repeatable := repeatableFlags[flagName]; repeatable {
			// Repeatable flags: keep every distinct full-string occurrence
			// (collapse exact duplicates only).
			seen := make(map[string]bool, len(occurrences))
			for _, occ := range occurrences {
				if !seen[occ.arg] {
					seen[occ.arg] = true
					keepIndex[occ.index] = true
				}
			}
			continue
		}

		// Check if all args are identical (exact duplicates)
		allSame := true
		first := occurrences[0].arg
		for _, occ := range occurrences[1:] {
			if occ.arg != first {
				allSame = false
				break
			}
		}
		if allSame {
			// Keep first for exact duplicates (preserve order)
			keepIndex[occurrences[0].index] = true
		} else {
			// Keep last for value overrides (user args win)
			keepIndex[occurrences[len(occurrences)-1].index] = true
		}
	}

	// Build result preserving original order
	var result []string
	for i, arg := range args {
		if keepIndex[i] {
			result = append(result, arg)
		}
	}

	return result
}

// extractFlagName extracts the flag name from a CLI argument.
// For "--flag=value", it returns "--flag".
// For "--flag value" style (where value is separate), it returns "--flag".
// For non-flag arguments, it returns the argument as-is.
func extractFlagName(arg string) string {
	// Handle --flag=value style
	if idx := strings.Index(arg, "="); idx != -1 {
		return arg[:idx]
	}
	// For --flag or -f style, or non-flag args, return as-is
	return arg
}

func Find[T any](elements []T, fn func(T) bool) *T {
	for _, el := range elements {
		if fn(el) {
			return &el
		}
	}
	return nil
}

func Any[T any](elements []T, fn func(T) bool) bool {
	return Find(elements, fn) != nil
}

func Remove[T any](elements []T, fn func(T) bool) []T {
	var result []T
	for _, elem := range elements {
		if !fn(elem) {
			result = append(result, elem)
		}
	}
	return result
}

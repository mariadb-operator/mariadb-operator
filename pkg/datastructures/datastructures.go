package datastructures

import (
	"errors"
	"fmt"
	"sort"
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

func Any[T any](elements []T, fn func(T) bool) bool {
	for _, el := range elements {
		if fn(el) {
			return true
		}
	}
	return false
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

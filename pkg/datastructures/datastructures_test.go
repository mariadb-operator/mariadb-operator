package datastructures

import (
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("Index", func() {
	It("removes duplicates and supports index operations", func() {
		idx := newIndex("a", "b", "b", "c", "d", "e")
		Expect(idx).To(Equal(newIndex("a", "b", "c", "d", "e")))

		item, err := Get(idx, "b")
		Expect(err).NotTo(HaveOccurred())
		Expect(item).To(Equal("b"))

		keys := Keys(idx)
		expectedKeys := []string{"a", "b", "c", "d", "e"}
		sort.Strings(keys)
		sort.Strings(expectedKeys)
		Expect(keys).To(Equal(expectedKeys))

		Expect(AllExists(idx, "a", "b", "c")).To(BeTrue())
		Expect(AllExists(idx, "a", "b", "c", "z")).To(BeFalse())

		Expect(Has(idx, "a")).To(BeTrue())
		Expect(Has(idx, "z")).To(BeFalse())

		filteredIdx := Filter(idx, "a", "b", "c")
		Expect(filteredIdx).To(Equal(newIndex("a", "b", "c")))
	})
})

var _ = Describe("Diff", func() {
	DescribeTable("computes the diff between current and previous index",
		func(current Index[string], previous Index[string], wantDiff DiffResult) {
			diff := Diff(current, previous)
			Expect(diff).To(Equal(wantDiff))
		},
		Entry("no diff",
			newIndex("a", "b"),
			newIndex("a", "b"),
			DiffResult{
				Rest: []string{"a", "b"},
			},
		),
		Entry("added",
			newIndex("a", "b", "c", "d"),
			newIndex("a", "b"),
			DiffResult{
				Added: []string{"c", "d"},
				Rest:  []string{"a", "b"},
			},
		),
		Entry("deleted",
			newIndex(),
			newIndex("a", "b"),
			DiffResult{
				Deleted: []string{"a", "b"},
			},
		),
		Entry("added and deleted",
			newIndex("b", "d", "e", "f"),
			newIndex("a", "b", "c"),
			DiffResult{
				Added:   []string{"d", "e", "f"},
				Deleted: []string{"a", "c"},
				Rest:    []string{"b"},
			},
		),
		Entry("no intersection",
			newIndex("d", "e", "f"),
			newIndex("a", "b", "c"),
			DiffResult{
				Added:   []string{"d", "e", "f"},
				Deleted: []string{"a", "b", "c"},
			},
		),
	)
})

var _ = Describe("Merge", func() {
	DescribeTable("merges multiple slices",
		func(slices [][]string, wantSlice []string) {
			Expect(Merge(slices...)).To(Equal(wantSlice))
		},
		Entry("empty",
			[][]string{
				{},
				nil,
			},
			nil,
		),
		Entry("half empty",
			[][]string{
				{"a", "b", "c"},
				{},
			},
			[]string{"a", "b", "c"},
		),
		Entry("full",
			[][]string{
				{"a", "b", "c"},
				{"d", "e"},
			},
			[]string{"a", "b", "c", "d", "e"},
		),
		Entry("multiple",
			[][]string{
				{"a", "b", "c"},
				{"d", "e"},
				{"f", "g", "h"},
				{"i"},
			},
			[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
		),
	)
})

var _ = Describe("Unique", func() {
	DescribeTable("removes duplicated elements",
		func(elements []string, wantElements []string) {
			got := Unique(wantElements...)
			Expect(got).To(Equal(wantElements))
		},
		Entry("empty", nil, nil),
		Entry("some repeated",
			[]string{"a", "b", "b", "c"},
			[]string{"a", "b", "c"},
		),
		Entry("multiple repeated",
			[]string{"a", "b", "b", "c", "d", "d", "d", "e"},
			[]string{"a", "b", "c", "d", "e"},
		),
		Entry("all different",
			[]string{"a", "b", "c", "d", "e"},
			[]string{"a", "b", "c", "d", "e"},
		),
	)
})

var _ = Describe("Find", func() {
	DescribeTable("finds an element matching the predicate",
		func(elements []string, fn func(string) bool, wantElement *string) {
			element := Find(elements, fn)
			Expect(element).To(Equal(wantElement))
		},
		Entry("empty", nil, nil, nil),
		Entry("not found",
			[]string{"a", "b", "c"},
			func(s string) bool { return s == "d" },
			nil,
		),
		Entry("found",
			[]string{"a", "b", "c"},
			func(s string) bool { return s == "b" },
			ptr.To("b"),
		),
	)
})

var _ = Describe("Any", func() {
	DescribeTable("reports whether any element matches the predicate",
		func(elements []string, fn func(string) bool, wantBool bool) {
			gotBool := Any(elements, fn)
			Expect(gotBool).To(Equal(wantBool))
		},
		Entry("empty", nil, nil, false),
		Entry("no match",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			false,
		),
		Entry("single match",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			true,
		),
		Entry("multiple match",
			[]string{
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
				"--databases foo",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			true,
		),
	)
})

var _ = Describe("UniqueArgs", func() {
	DescribeTable("removes duplicated args preserving overrides",
		func(args []string, wantElements []string) {
			elements := UniqueArgs(args...)
			Expect(elements).To(Equal(wantElements))
		},
		Entry("empty", nil, nil),
		Entry("no duplicates",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
		Entry("exact duplicates keep first",
			[]string{
				"--single-transaction",
				"--events",
				"--events",
			},
			[]string{
				"--single-transaction",
				"--events",
			},
		),
		Entry("flag with value override - user arg wins",
			[]string{
				"--ssl-verify-server-cert",
				"--events",
				"--ssl-verify-server-cert=0",
			},
			[]string{
				"--events",
				"--ssl-verify-server-cert=0",
			},
		),
		Entry("exact duplicates preserve order user override wins",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--ssl",
				"--ssl-verify-server-cert",
				// user args - exact duplicate of --events, override of --ssl-verify-server-cert
				"--ssl-verify-server-cert=0",
				"--events",
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--ssl",
				"--ssl-verify-server-cert=0",
			},
		),
		Entry("multiple value overrides keep last",
			[]string{
				"--timeout=30",
				"--retries=3",
				"--timeout=60",
			},
			[]string{
				"--retries=3",
				"--timeout=60",
			},
		),
		Entry("non-flag args preserved",
			[]string{
				"backup",
				"--verbose",
				"restore",
			},
			[]string{
				"backup",
				"--verbose",
				"restore",
			},
		),
		Entry("mixed exact and value duplicates",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
				"--events",
				"--all-databases",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
			},
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--all-databases",
				"--skip-add-locks",
				"--ignore-table=mysql.global_priv",
				"--verbose",
			},
		),
	)
})

var _ = Describe("Remove", func() {
	DescribeTable("removes elements matching the predicate",
		func(elements []string, fn func(string) bool, wantElements []string) {
			elements = Remove(elements, fn)
			Expect(elements).To(Equal(wantElements))
		},
		Entry("empty", nil, nil, nil),
		Entry("no match",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
		Entry("remove first",
			[]string{
				"--databases foo",
				"--single-transaction",
				"--events",
				"--routines",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
		Entry("remove middle",
			[]string{
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
		Entry("remove last",
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
				"--databases foo",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
		Entry("multiple match",
			[]string{
				"--databases foo",
				"--single-transaction",
				"--databases foo",
				"--events",
				"--routines",
				"--databases foo",
			},
			func(s string) bool { return strings.HasPrefix(s, "--databases") },
			[]string{
				"--single-transaction",
				"--events",
				"--routines",
			},
		),
	)
})

func newIndex(items ...string) Index[string] {
	return NewIndex[string](items, func(s string) string {
		return s
	})
}

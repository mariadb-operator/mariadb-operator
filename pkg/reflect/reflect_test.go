package reflect

import (
	"unsafe"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsNil", func() {
	DescribeTable("detects nil values correctly",
		func(input interface{}, want bool) {
			Expect(IsNil(input)).To(Equal(want))
		},
		Entry("nil channel", (chan int)(nil), true),
		Entry("nil func", (func())(nil), true),
		Entry("nil interface containing nil pointer", interface{}((*int)(nil)), true),
		Entry("nil interface", nil, true),
		Entry("nil map", (map[string]int)(nil), true),
		Entry("nil pointer", (*int)(nil), true),
		Entry("nil slice", ([]int)(nil), true),
		Entry("nil unsafe pointer", (unsafe.Pointer)(nil), true),
		Entry("non-nil channel", make(chan int), false),
		Entry("non-nil func", func() {}, false),
		Entry("non-nil int", 123, false),
		Entry("non-nil interface", interface{}(0), false),
		Entry("non-nil map", map[string]int{}, false),
		Entry("non-nil pointer", new(int), false),
		Entry("non-nil slice", []int{}, false),
		Entry("non-nil string", "hello", false),
		Entry("non-nil unsafe pointer", unsafe.Pointer(new(int)), false),
	)
})

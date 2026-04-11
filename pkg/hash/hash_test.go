package hash

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

var _ = Describe("HashJSON", func() {
	DescribeTable("hashes various inputs",
		func(input any, want string, wantErr bool) {
			got, err := HashJSON(input)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(want))
			}
		},
		Entry("Struct", TestStruct{Field1: "value1", Field2: 42}, Hash(`{"field1":"value1","field2":42}`), false),
		Entry("Slice of structs",
			[]TestStruct{{Field1: "value1", Field2: 42}, {Field1: "value2", Field2: 84}},
			Hash(`[{"field1":"value1","field2":42},{"field1":"value2","field2":84}]`),
			false,
		),
		Entry("Empty struct", TestStruct{}, Hash(`{"field1":"","field2":0}`), false),
		Entry("Map",
			map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"},
			Hash(`{"a":"1","b":"2","c":"3","d":"4","e":"5"}`),
			false,
		),
		Entry("Nil", nil, Hash("null"), false),
		Entry("Invalid input type", make(chan int), "", true),
	)
})

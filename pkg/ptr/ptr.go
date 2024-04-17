package ptr

// Deref attempts to dereference a list of pointers and returns the first non nil. If all of them are nil, returns the default value.
func Deref[T any](ptrs []*T, def T) T {
	for _, p := range ptrs {
		if p != nil {
			return *p
		}
	}
	return def
}

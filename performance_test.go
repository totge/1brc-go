package main

import "testing"

func TestBaseVersion(t *testing.T) {
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 4*1024*1024)
	// })
	Measure("sequential_idiomatic", *profile, func() {
		BaseExecute(resolveInput(*input), 8*1024*1024)
	})
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 12*1024*1024)
	// })
}

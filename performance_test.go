package main

import (
	"1brc-go/iterations/base"
	iter01 "1brc-go/iterations/iter_01"
	"testing"
)

func TestBaseVersion(t *testing.T) {
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 4*1024*1024)
	// })
	Measure("sequential_idiomatic", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		base.Execute(inputPath, outputPath, 8*1024*1024)
	})
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 12*1024*1024)
	// })
}

func TestIter01(t *testing.T) {
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 4*1024*1024)
	// })
	Measure("iter_01", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		iter01.Execute(inputPath, outputPath, 8*1024*1024)
	})
	// Measure("sequential_idiomatic", *profile, func() {
	// 	BaseExecute(resolveInput(*input), 12*1024*1024)
	// })
}

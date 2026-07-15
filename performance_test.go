package main

import (
	"1brc-go/iterations/base"
	iter01 "1brc-go/iterations/iter_01"
	iter02 "1brc-go/iterations/iter_02"
	iter03 "1brc-go/iterations/iter_03"
	iter04 "1brc-go/iterations/iter_04"
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
	Measure("iter_01", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		iter01.Execute(inputPath, outputPath, 8*1024*1024)
	})
}

func TestIter02(t *testing.T) {
	Measure("iter_02_gen", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		iter02.Execute(inputPath, outputPath, 8*1024*1024)
	})
}

func TestIter03(t *testing.T) {
	// Measure("iter_03_p30", *profile, func() {
	// 	inputPath, outputPath := resolveFileSize(*input)
	// 	iter03.Execute(inputPath, outputPath, 10*1024*1024, 30)
	// })
	// Measure("iter_03_p40", *profile, func() {
	// 	inputPath, outputPath := resolveFileSize(*input)
	// 	iter03.Execute(inputPath, outputPath, 10*1024*1024, 40)
	// })
	Measure("iter_03_p50", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		iter03.Execute(inputPath, outputPath, 10*1024*1024, 50)
	})
	// Measure("iter_03_p60", *profile, func() {
	// 	inputPath, outputPath := resolveFileSize(*input)
	// 	iter03.Execute(inputPath, outputPath, 10*1024*1024, 60)
	// })
}

func TestIter04(t *testing.T) {
	Measure("iter_04_p50_recgen", *profile, func() {
		inputPath, outputPath := resolveFileSize(*input)
		iter04.Execute(inputPath, outputPath, 16*1024*1024, 50)
	})
	// Measure("iter_04_p90", *profile, func() {
	// 	inputPath, outputPath := resolveFileSize(*input)
	// 	iter04.Execute(inputPath, outputPath, 10*1024*1024, 90)
	// })
	// Measure("iter_04_p65", *profile, func() {
	// 	inputPath, outputPath := resolveFileSize(*input)
	// 	iter04.Execute(inputPath, outputPath, 10*1024*1024, 65)
	// })
}

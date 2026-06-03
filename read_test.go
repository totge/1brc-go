package main

import (
	"flag"
	"os"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestMeasureRun(t *testing.T) {
	if *profile {
		os.MkdirAll("profiles", 0755)
	}
	Measure("runner", *profile, func() {
		Runner(resolveInput(*input))
	})
}

func TestBufioSequential(t *testing.T) {
	if *profile {
		os.MkdirAll("profiles", 0755)
	}
	Measure("bufio_4KB", *profile, func() {
		readBufioSize(resolveInput(*input), 4*1024)
	})
	Measure("bufio_500KB", *profile, func() {
		readBufioSize(resolveInput(*input), 512*1024)
	})
	Measure("bufio_1MB", *profile, func() {
		readBufioSize(resolveInput(*input), 1024*1024)
	})
	Measure("bufio_2MB", *profile, func() {
		readBufioSize(resolveInput(*input), 2*1024*1024)
	})
	Measure("bufio_16MB", *profile, func() {
		readBufioSize(resolveInput(*input), 16*1024*1024)
	})
	Measure("bufio_scanner", *profile, func() {
		readBufioScanner(resolveInput(*input))
	})
}
func TestReadSequeltial(t *testing.T) {
	Measure("read_4KB", *profile, func() {
		readChunked(resolveInput(*input), 4*1024)
	})
	Measure("read_500KB", *profile, func() {
		readChunked(resolveInput(*input), 512*1024)
	})
	Measure("read_1MB", *profile, func() {
		readChunked(resolveInput(*input), 1024*1024)
	})
	Measure("read_2MB", *profile, func() {
		readChunked(resolveInput(*input), 2*1024*1024)
	})
	Measure("read_16MB", *profile, func() {
		readChunked(resolveInput(*input), 16*1024*1024)
	})
}

func TestMMappedRead(t *testing.T) {
	Measure("mapped_4KB", *profile, func() {
		readMMapped(resolveInput(*input), 4*1024)
	})
	Measure("mapped_1MB", *profile, func() {
		readMMapped(resolveInput(*input), 1024*1024)
	})
	Measure("mapped_4MB", *profile, func() {
		readMMapped(resolveInput(*input), 4*1024*1024)
	})
	Measure("mapped_16MB", *profile, func() {
		readMMapped(resolveInput(*input), 16*1024*1024)
	})
}

func TestMMappedReadNoCopy(t *testing.T) {
	Measure("mapped_no_copy", *profile, func() {
		readMMappedNoCopy(resolveInput(*input))
	})

}

func TestChunkedReadConcurrent(t *testing.T) {
	Measure("chunked_14_6MB", *profile, func() {
		readChunkedConcurrent(resolveInput(*input), 14, 6*1024*1024)
	})
	Measure("chunked_16_6MB", *profile, func() {
		readChunkedConcurrent(resolveInput(*input), 16, 6*1024*1024)
	})
	Measure("chunked_14_4MB", *profile, func() {
		readChunkedConcurrent(resolveInput(*input), 14, 4*1024*1024)
	})
	Measure("chunked_16_4MB", *profile, func() {
		readChunkedConcurrent(resolveInput(*input), 16, 4*1024*1024)
	})
}

func TestChunkedWorkerPool(t *testing.T) {
	Measure("pool_128_auto_8MB", *profile, func() {
		readChunkedWorkerPool(resolveInput(*input), 128, runtime.NumCPU(), 8*1024*1024)
	})
	// Measure("pool_128_auto_4MB", *profile, func() {
	// 	readChunkedWorkerPool(resolveInput(*input), 128, runtime.NumCPU(), 4*1024*1024)
	// })
	// Measure("pool_128_auto_2MB", *profile, func() {
	// 	readChunkedWorkerPool(resolveInput(*input), 128, runtime.NumCPU(), 2*1024*1024)
	// })
	// Measure("pool_200_32_4MB", *profile, func() {
	// 	readChunkedWorkerPool(resolveInput(*input), 200, 32, 4*1024*1024)
	// })
	// Measure("pool_50_16_4MB", *profile, func() {
	// 	readChunkedWorkerPool(resolveInput(*input), 50, 16, 4*1024*1024)
	// })
	// Measure("pool_16_16_4MB", *profile, func() {
	// 	readChunkedWorkerPool(resolveInput(*input), 16, 16, 4*1024*1024)
	// })
}

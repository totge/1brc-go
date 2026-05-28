package main

import (
	"flag"
	"os"
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

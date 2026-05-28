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
	Measure("bufio_4KB", false, func() {
		readBufioSize(resolveInput(*input), 4*1024)
	})
	Measure("bufio_500KB", false, func() {
		readBufioSize(resolveInput(*input), 512*1024)
	})
	Measure("bufio_1MB", false, func() {
		readBufioSize(resolveInput(*input), 1024*1024)
	})
	Measure("bufio_2MB", false, func() {
		readBufioSize(resolveInput(*input), 2*1024*1024)
	})
	Measure("bufio_16MB", false, func() {
		readBufioSize(resolveInput(*input), 16*1024*1024)
	})
	Measure("bufio_scanner", false, func() {
		readBufioScanner(resolveInput(*input))
	})
}
func TestReadSequeltial(t *testing.T) {
	Measure("read_4KB", false, func() {
		readChunked(resolveInput(*input), 4*1024)
	})
	Measure("read_500KB", false, func() {
		readChunked(resolveInput(*input), 512*1024)
	})
	Measure("read_1MB", false, func() {
		readChunked(resolveInput(*input), 1024*1024)
	})
	Measure("read_2MB", false, func() {
		readChunked(resolveInput(*input), 2*1024*1024)
	})
	Measure("read_16MB", false, func() {
		readChunked(resolveInput(*input), 16*1024*1024)
	})
}

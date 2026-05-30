package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var input = flag.String("f", "small", "dataset: small, full")
var profile = flag.Bool("p", false, "save cpu and memory profiles")

func main() {
	flag.Parse()
	Runner(resolveInput(*input))
}

func resolveInput(name string) string {
	switch name {
	case "mid":
		return "data/measurements_mid.txt"
	case "full":
		return "data/measurements.txt"
	default:
		return "data/measurements_small.txt"
	}
}

// wrapper to run a function and add add time measurement and profiling
func Measure(name string, enableProfile bool, fn func()) {
	now := time.Now()
	timestamp := now.Format("20060102_150405")

	var cpuFile *os.File
	if enableProfile {
		cpuFile, _ = os.Create(fmt.Sprintf("profiles/cpu_%s_%s.prof", name, timestamp))
		pprof.StartCPUProfile(cpuFile)
	}

	runtime.GC()
	var mStart, mEnd runtime.MemStats
	runtime.ReadMemStats(&mStart)
	start := time.Now()

	fn()

	elapsed := time.Since(start)

	if enableProfile {
		pprof.StopCPUProfile()
		cpuFile.Close()
		memFile, _ := os.Create(fmt.Sprintf("profiles/mem_%s_%s.prof", name, timestamp))
		pprof.WriteHeapProfile(memFile)
		memFile.Close()
	}

	runtime.ReadMemStats(&mEnd)
	allocMB := float64(mEnd.TotalAlloc-mStart.TotalAlloc) / 1024 / 1024

	fmt.Printf("➜ [%-15s] Time: %-12s | Mem: %7.2f MB | Profiled: %v\n", name, elapsed, allocMB, enableProfile)
}

func Runner(filePath string) {
	readMMappedNoCopy(filePath)
}

package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)
var version string = "v0"

var dir = flag.String("dir", "", "sudirectory for the profile files in the `profiles` folder")
var postfix = flag.String("psf", "", "postfix for the profile files")
var input = flag.String("f", "", "input size, `smalll`/`mid`/`large`/`full`")
var loops = flag.Int("n", 1, "number of executions")

func saveStats(execTimes []float64, loopNums int, input string) {

	var sum float64
	var minExec float64
	var maxExec float64

	for i, dur := range execTimes {
		sum += dur
		maxExec = max(maxExec, dur)

		if i == 0 {
			minExec = dur
		} else {
			minExec = min(minExec, dur)
		}
	}
	avgExec := sum / float64(loopNums)

	fmt.Printf("Avg execution time was: %f.\n\tmax time: %f\n\tmin time: %f\n", avgExec, maxExec, minExec)

	file, err := os.OpenFile("./stat/timestats.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		fmt.Println(execTimes)
		return
	}
	defer file.Close()

	w := csv.NewWriter(file)

	record := []string{
		time.Now().Format(time.DateTime),
		version,
		input,
		fmt.Sprintf("%d", loopNums),
		strconv.FormatFloat(avgExec, 'f', 3, 64),
		strconv.FormatFloat(minExec, 'f', 3, 64),
		strconv.FormatFloat(maxExec, 'f', 3, 64),
		fmt.Sprintf("%v", execTimes),
	}

	w.Write(record)
	w.Flush()
}

func main() {
	flag.Parse()

	if *dir != "" {

		folder := "./profiles/" + *dir

		if err := os.Mkdir(folder, 0755); os.IsExist(err) {
			fmt.Println("The directory named", *dir, "exists")
		}

		execFile, err := os.Create(folder + "/exec_" + *postfix + ".prof")
		if err != nil {
			log.Fatal("could not create trace execution profile: ", err)
		}
		defer execFile.Close()
		trace.Start(execFile)
		defer trace.Stop()

		cpuFile, err := os.Create(folder + "/cpu_" + *postfix + ".prof")
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer cpuFile.Close()

		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()

		memFile, err := os.Create(folder + "/mem_" + *postfix + ".prof")
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer memFile.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(memFile); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}

	}

	var filePath string
	fmt.Printf("input: %s\n", *input)
	switch *input {
	case "small":
		filePath = "data/measurements_small.txt"
	case "mid":
		filePath = "data/measurements_mid.txt"
	case "full":
		filePath = "data/measurements.txt"
	case "large":
		filePath = "data/measurements_large.txt"
	default:
		filePath = "data/measurements_small.txt"
	}

	fmt.Printf("file path: %s\n", filePath)

	execTimes := make([]float64, 0, *loops)

	for i := 0; i < *loops; i++ {
		start := time.Now()
		process(filePath)
		dur := time.Since(start)

		execTimes = append(execTimes, dur.Seconds())
	}

	saveStats(execTimes, *loops, *input)

}

type measurement struct {
	location    string
	temperature float64
}

type metrics struct {
	minTemp float64
	maxTemp float64
	avgTemp float64
}

// orchestrates the entire process from reading the input till producing the output
func process(filePath string) {
	rawData, err := loadData(filePath)
	if err != nil {
		fmt.Println("Could not open file")
		os.Exit(1)
	}

	data := processData(rawData)

	grouped := make(map[string][]float64)

	// sorting each measurement by its location
	for _, m := range data {

		v, ok := grouped[m.location]

		if ok {
			grouped[m.location] = append(v, m.temperature)

		} else {
			v := make([]float64, 0, 100)
			grouped[m.location] = append(v, m.temperature)
		}

	}

	// calculating the metrics for each location
	results := make(map[string]metrics)
	for k, v := range grouped {

		var sumTemp float64
		for _, m := range v {
			sumTemp += m
		}

		maxTemp := slices.Max(v)
		minTemp := slices.Min(v)
		avgTemp := sumTemp / float64(len(v))

		results[k] = metrics{minTemp, maxTemp, avgTemp}
	}

	// test sort
	// for k, v := range grouped {
	// 	if len(v) >= 2 {
	// 		fmt.Printf("%s, metrics: %v\n", k, results[k])
	// 	}
	// }

	// sorting the output order
	locations := make([]string, 0, len(results))

	for k := range results {
		locations = append(locations, k)
	}

	sort.Strings(locations)

	// creating output

	csvFile, err := os.Create("output/result.csv")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer csvFile.Close()

	header := []string{"loaction", "min", "max", "avg"}
	csvWriter := csv.NewWriter(csvFile)

	csvWriter.Write(header)

	for _, l := range locations {
		currMetrics := results[l]
		minTempStr := strconv.FormatFloat(currMetrics.minTemp, 'f', 1, 64)
		maxTempStr := strconv.FormatFloat(currMetrics.maxTemp, 'f', 1, 64)
		avgTempStr := strconv.FormatFloat(currMetrics.avgTemp, 'f', 1, 64)

		row := []string{l, minTempStr, avgTempStr, maxTempStr}
		csvWriter.Write(row)
	}
	csvWriter.Flush()

}

func processData(rawData []string) []measurement {
	var processedData []measurement
	for i, line := range rawData {
		parts := strings.Split(line, ";")
		temperature, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			fmt.Printf("Error parsing line %d: %s could not be parsed\n", i, parts[1])
		}
		processedData = append(processedData, measurement{location: parts[0], temperature: temperature})
	}

	return processedData
}

// loads the input file data row by row into a slice of string values
func loadData(filePath string) (data []string, err error) {
	file, err := os.Open(filePath)

	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		data = append(data, scanner.Text())

	}

	return data, nil
}

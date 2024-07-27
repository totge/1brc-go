package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var version string = "v2"

var dir = flag.String("dir", "", "sudirectory for the profile files in the `profiles` folder")
var postfix = flag.String("psf", "", "postfix for the profile files")
var input = flag.String("f", "", "input size, `smalll`/`mid`/`large`/`full`")
var loops = flag.Int("n", 1, "number of executions")

var NUM_WORKER int = 5


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

type SafeReader struct {
	file os.File
	readNo int
	mut sync.Mutex
}

type partialRecord struct {
	byteContent []byte
	readNo int
}


type measurement struct {
	location    string
	temperature float64
}

type aggregate struct {
	sum     float64
	count   int
	minTemp float64
	maxTemp float64
}

func (a *aggregate) addMeasurement(temp float64) {
	if a.count == 0 {
		a.count = 1
		a.sum = temp
		a.minTemp = temp
		a.maxTemp = temp
	}
	a.count += 1
	a.sum += temp
	a.minTemp = min(temp, a.minTemp)
	a.maxTemp = max(temp, a.maxTemp)
}

func (a *aggregate) calcMetrics() (minTemp float64, maxTemp float64, avg float64) {
	avg = a.sum / float64(a.count)
	return a.minTemp, a.maxTemp, avg
}

func worker(safeReader *SafeReader, ch chan partialRecord, resCh chan map[string]*aggregate){
	buffer := make([]byte, 0, 100 * 1024 * 1024) //100 MB
	partialResults := make(map[string]*aggregate)
	endReached := false
	
	for !endReached {
		safeReader.mut.Unlock()
		numBytes, err := safeReader.file.Read(buffer)
		readNo := safeReader.readNo
		safeReader.readNo++
		safeReader.mut.Lock()

		if err != nil {
			if err == io.EOF {
				endReached = true
			} else {
				fmt.Println(err)
				os.Exit(1)
			}
		}
		if numBytes == 0 {
			break
		}
		
		// BUG: is it ok to reassign the buffer to a shorter slice?? -- no it's a problem
		if numBytes != len(buffer) {
			buffer = buffer[:numBytes]
		}
		// handling first - possibly not complete - record
		// TODO: handle if \n is not found in slice
		firstChunkIndex := bytes.IndexRune(buffer, '\n')
	
		ch <- partialRecord{bytes.Clone(buffer[:firstChunkIndex + 1]), 2*readNo}

		// handling last - possibly not complete - record
		// TODO: handle if last \n not found
		lastChunkIndex := bytes.LastIndexByte(buffer[firstChunkIndex+1:], '\n')


		ch <- partialRecord{bytes.Clone(buffer[lastChunkIndex + 1:]), 2*readNo + 1}

		completeRecords := buffer[firstChunkIndex+1:lastChunkIndex]

		recordEnd := bytes.IndexRune(completeRecords, '\n')
		for recordEnd > 0 {
			// processing record and adding it to the map
			loc, temp := processLine(completeRecords[:recordEnd])
			if partialResults[loc] == nil {
				partialResults[loc] = &aggregate{temp, 1, temp, temp}
			} else {
				partialResults[loc].addMeasurement(temp)
			}
		}

		resCh <- partialResults

	}
}

// össze kéne pározítani a random módon félbevágott recordokat - sorszámra hogy van a logika?
// az első az idekerül??
// lehet hogy egész record kerül be ide?
func chunkProcessor(ch chan partialRecord, resCh chan map[string]*aggregate){

	chunks
	for chunk := range ch {

	}

}
// orchestrates the entire process from reading the input till producing the output
func process(filePath string) {

	// open the input file
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
 
	var safeReader SafeReader
	safeReader.file = *file

	bufferSize := 100 * 1024 * 1024 // 100 MB


	buffer := make([]byte, 0, bufferSize)
	
	recordChunkChannel := make(chan partialRecord, 10000)
	partialResultsChannel := make(chan map[string]*aggregate, NUM_WORKER + 1)
	
	for i := 0; i < NUM_WORKER; i++ {
		go worker(&safeReader, recordChunkChannel, partialResultsChannel)
		go chunkProcessor(recordChunkChannel, partialResultsChannel)
	}

	// iterate over the input file, group measurements by location
	for i := 1; scanner.Scan(); i++ {
		line := scanner.Text()

		location, temperature := processData(line)

		if agg, ok := grouped[location]; ok {
			agg.addMeasurement(temperature)
		} else {
			agg = &aggregate{sum: temperature, count: 1, minTemp: temperature, maxTemp: temperature}
			grouped[location] = agg
		}
	}

	// create a sorted list of the locations
	locations := make([]string, 0, len(grouped))

	for l := range grouped {
		locations = append(locations, l)
	}
	sort.Strings(locations)

	// open file for output
	csvFile, err := os.Create("output/result.csv")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer csvFile.Close()

	header := []string{"loaction", "min", "max", "avg"}
	csvWriter := csv.NewWriter(csvFile)
	csvWriter.Write(header)

	// calculate the metrics for each location, write it to output
	for _, l := range locations {

		agg := grouped[l]

		minTemp, maxTemp, avgTemp := agg.calcMetrics()

		minTempStr := strconv.FormatFloat(minTemp, 'f', 1, 64)
		maxTempStr := strconv.FormatFloat(maxTemp, 'f', 1, 64)
		avgTempStr := strconv.FormatFloat(avgTemp, 'f', 1, 64)

		row := []string{l, minTempStr, maxTempStr, avgTempStr}
		csvWriter.Write(row)
	}

	csvWriter.Flush()

}

func processLine(line []byte) (location string, temperature float64) {
	// cut doesn't include the separator in either part
	locBytes, tempBytes, found := bytes.Cut(line, []byte(";"))

	if !found {
		fmt.Println("; not found in string:", string(line))
	}

	location = string(locBytes)
	temperature, err := strconv.ParseFloat(string(tempBytes), 64)

	if err != nil {
		fmt.Printf("%s could not be parsed\n", tempBytes)
	}

	return location, temperature
}

func processData(line string) (location string, temperature float64) {

	parts := strings.Split(line, ";")

	location = parts[0]
	temperature, err := strconv.ParseFloat(parts[1], 64)

	if err != nil {
		fmt.Printf("%s could not be parsed\n", parts[1])
	}

	return location, temperature
}


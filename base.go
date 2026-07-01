package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
)

type ChunkReader struct {
	reader     io.ReaderAt
	offset     int64
	bufferSize int
	hasNext    bool
	separator  byte
}

// bufferSize must be greater than record size
func NewChunkReader(reader io.ReaderAt, bufferSize int, separator byte) *ChunkReader {
	return &ChunkReader{
		reader:     reader,
		offset:     0,
		bufferSize: bufferSize,
		separator:  separator,
		hasNext:    true,
	}
}

func (chr *ChunkReader) HasNext() bool {
	return chr.hasNext
}

func (chr *ChunkReader) ReadNextChunk() ([]byte, error) {
	buffer := make([]byte, chr.bufferSize)

	n, err := chr.reader.ReadAt(buffer, chr.offset)

	// handle read errors
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read data chunk: %w", err)
	}
	// handle reaching end of file
	if err == io.EOF {
		chr.hasNext = false
	}

	dataRead := buffer[:n]

	// track the last recors separator in the buffer
	lastSeparator := -1

	// iterate backwards from the end of data read
	for i := len(dataRead) - 1; i >= 0; i-- {
		if dataRead[i] == chr.separator {
			lastSeparator = i
			break
		}
	}

	// no record separator found -> should never happen
	if lastSeparator == -1 {
		return nil, fmt.Errorf("no separator found in the data chunk.")
	}

	adjustedChunk := dataRead[:lastSeparator+1]
	chr.offset += int64(lastSeparator + 1)

	return adjustedChunk, nil
}

func ProduceRawRecords(chunk []byte, separator byte) []string {
	var records []string

	recordStart := 0

	for i, b := range chunk {
		if b == separator {
			records = append(records, string(chunk[recordStart:i+1]))
			recordStart = i + 1
		}
	}

	return records
}

type Record struct {
	station string
	temp    float64
}

func ParseRecord(rawRecord string) (Record, error) {
	var record Record

	separatorIdx := strings.Index(rawRecord, ";")
	if separatorIdx == -1 {
		return record, fmt.Errorf("separator ';' not found in record: %s", rawRecord)
	}

	record.station = rawRecord[:separatorIdx]

	temp, err := strconv.ParseFloat(rawRecord[separatorIdx+1:len(rawRecord)-1], 64)
	if err != nil {
		return record, fmt.Errorf("failed to convert temperature to float in record: %s", rawRecord)
	}
	record.temp = temp

	return record, nil
}

type Metrics struct {
	min float64
	avg float64
	max float64
}

type Aggregator struct {
	cityMeasurements map[string][]float64
}

func NewAggregator() Aggregator {
	cityMeasurements := make(map[string][]float64)

	return Aggregator{cityMeasurements: cityMeasurements}
}

func (a *Aggregator) AddRecord(record Record) {

	measurements := a.cityMeasurements[record.station]
	measurements = append(measurements, record.temp)
	a.cityMeasurements[record.station] = measurements

}

func (a *Aggregator) ListCities() []string {
	cities := make([]string, 0, len(a.cityMeasurements))

	for k := range a.cityMeasurements {
		cities = append(cities, k)
	}

	return cities
}

func (a *Aggregator) CalculateMetricsForCity(city string) (Metrics, error) {
	var metrics Metrics

	data, ok := a.cityMeasurements[city]
	if !ok {
		return metrics, fmt.Errorf("city not found: %s", city)
	}

	count := len(data)
	sum := 0.0
	for _, v := range data {
		sum += v
	}

	// TODO: do the rounding here
	metrics.max = slices.Max(data)
	metrics.min = slices.Min(data)
	metrics.avg = sum / float64(count)

	return metrics, nil
}

func RoundToOneDecimal(x float64) float64 {
	return math.Floor(x*10.0+0.5) / 10.0
}

func FormatMetrics(city string, metrics Metrics) string {
	min := RoundToOneDecimal(metrics.min)
	max := RoundToOneDecimal(metrics.max)
	avg := RoundToOneDecimal(metrics.avg)

	return fmt.Sprintf("%s=%.1f/%.1f/%.1f", city, min, avg, max)
}

func BaseExecute(inputPath string, bufferSize int) error {

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file at %s: %w", inputPath, err)
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile("results/results.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer outputFile.Close()

	reader := NewChunkReader(inputFile, bufferSize, '\n')
	aggregator := NewAggregator()

	// read and aggregate data
	for reader.HasNext() {
		chunk, err := reader.ReadNextChunk()
		if err != nil {
			return fmt.Errorf("failed reading chunk: %w", err)
		}

		rawRecords := ProduceRawRecords(chunk, '\n')

		for _, rawRec := range rawRecords {
			record, err := ParseRecord(rawRec)
			if err != nil {
				return fmt.Errorf("failed parsing record '%s': %w", rawRec, err)
			}

			aggregator.AddRecord(record)
		}
	}

	// write output

	cities := aggregator.ListCities()
	slices.Sort(cities)

	var sb strings.Builder

	sb.WriteString("{")

	for i, city := range cities {
		metrics, err := aggregator.CalculateMetricsForCity(city)
		if err != nil {
			return fmt.Errorf("failed to calculate metrics for city '%s': %w", city, err)
		}

		formattedOutput := FormatMetrics(city, metrics)
		sb.WriteString(formattedOutput)

		// don't add separator after last element
		if i+1 < len(cities) {
			sb.WriteString(", ")
		}
	}

	sb.WriteString("}")

	results := sb.String()

	_, err = outputFile.WriteString(results)
	if err != nil {
		panic(err)
	}
	return nil
}

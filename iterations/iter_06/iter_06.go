package iter06

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type Section struct {
	start  int64
	length int64
}

func CalculateSections(reader io.ReaderAt, dataSize int64, bufferSize int, separator byte, numSections int) ([]Section, error) {
	chunks := make([]Section, numSections)
	chunkSize := dataSize / int64(numSections)

	start := int64(0)
	for i := range numSections {
		end := dataSize
		if i < numSections-1 {
			var err error
			end, err = nextRecordBoundary(reader, start+chunkSize, bufferSize, separator)
			if err != nil {
				return nil, err
			}
		}

		chunks[i] = Section{start: start, length: end - start}
		start = end
	}

	return chunks, nil
}

// nextRecordBoundary returns the offset just past the first separator at or after targetOffset.
func nextRecordBoundary(reader io.ReaderAt, targetOffset int64, bufferSize int, separator byte) (int64, error) {
	peekBuf := make([]byte, bufferSize)
	n, err := reader.ReadAt(peekBuf, targetOffset)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("failed to read data: %w", err)
	}

	idx := bytes.IndexByte(peekBuf[:n], separator)
	if idx == -1 {
		return 0, fmt.Errorf("separator not found within %d bytes of offset %d", bufferSize, targetOffset)
	}

	return targetOffset + int64(idx) + 1, nil
}

type RecordGenerator struct {
	reader        *io.SectionReader
	sectionOffset int64
	buffer        []byte
	safeBuffer    []byte
	separator     byte
}

// bufferSize must be greater than record size
func NewRecordGenerator(reader io.ReaderAt, section Section, bufferSize int, separator byte) *RecordGenerator {
	sectionReader := io.NewSectionReader(reader, section.start, section.length)
	buffer := make([]byte, bufferSize)
	return &RecordGenerator{
		reader:        sectionReader,
		sectionOffset: 0,
		buffer:        buffer,
		separator:     separator,
	}
}

func (rg *RecordGenerator) readNextChunk() error {

	n, err := rg.reader.ReadAt(rg.buffer, rg.sectionOffset)

	// handle read errors
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read data chunk: %w", err)
	}

	if n == 0 {
		return io.EOF
	}

	dataRead := rg.buffer[:n]

	// adjust end to last record end
	lastSeparator := bytes.LastIndexByte(dataRead, rg.separator)

	// no record separator found -> should never happen
	if lastSeparator == -1 {
		return fmt.Errorf("no separator found in the data chunk.")
	}

	rg.safeBuffer = dataRead[:lastSeparator+1]
	rg.sectionOffset += int64(lastSeparator + 1)

	return nil
}

func (rg *RecordGenerator) ReadRecord() ([]byte, error) {

	// refill buffer if needed
	if len(rg.safeBuffer) == 0 || rg.safeBuffer == nil {
		err := rg.readNextChunk()
		if err != nil {
			return nil, err
		}
	}
	// find next record end
	idx := bytes.IndexByte(rg.safeBuffer, rg.separator)

	if idx == -1 {
		return nil, fmt.Errorf("no record separator found in buffer")
	}

	record := rg.safeBuffer[:idx]
	rg.safeBuffer = rg.safeBuffer[idx+1:]

	return record, nil
}

type Record struct {
	station []byte
	temp    float64
}

func ParseRecord(rawRecord []byte) (Record, error) {
	var record Record

	separatorIdx := bytes.IndexByte(rawRecord, ';')
	if separatorIdx == -1 {
		return record, fmt.Errorf("separator ';' not found in record: %s", rawRecord)
	}

	record.station = rawRecord[:separatorIdx]

	temp, err := strconv.ParseFloat(string(rawRecord[separatorIdx+1:]), 64)
	if err != nil {
		return record, fmt.Errorf("failed to convert temperature to float: %s in record: %s", rawRecord[separatorIdx+1:], rawRecord)
	}
	record.temp = temp

	return record, nil
}

type Metrics struct {
	min float64
	avg float64
	max float64
}

type AggregatedMeasurements struct {
	min   float64
	max   float64
	sum   float64
	count int
}

type MeasurementAggregator struct {
	cityMeasurements map[string]*AggregatedMeasurements
}

func NewMeasurementAggregator() MeasurementAggregator {
	cityMeasurements := make(map[string]*AggregatedMeasurements)

	return MeasurementAggregator{cityMeasurements: cityMeasurements}
}

func (a *MeasurementAggregator) AddRecord(record Record) {

	aggMeasurement, ok := a.cityMeasurements[string(record.station)]

	if !ok { // no previous measurement for the city
		aggMeasurement := AggregatedMeasurements{
			min:   record.temp,
			max:   record.temp,
			sum:   record.temp,
			count: 1,
		}

		a.cityMeasurements[string(record.station)] = &aggMeasurement

	} else { // there's already previous measurements, modify in place
		aggMeasurement.min = min(aggMeasurement.min, record.temp)
		aggMeasurement.max = max(aggMeasurement.max, record.temp)
		aggMeasurement.sum += record.temp
		aggMeasurement.count++
	}

}

type ResultAggregator struct {
	allResults map[string]*AggregatedMeasurements
}

func NewResultAggregator() ResultAggregator {
	allResults := make(map[string]*AggregatedMeasurements)

	return ResultAggregator{allResults: allResults}
}

func (ra *ResultAggregator) AddPartialResults(partialResults map[string]*AggregatedMeasurements) {
	for k, v := range partialResults {
		currentMeasurements, ok := ra.allResults[k]
		if !ok { // no previous measurement for the city, store the pointer directly
			ra.allResults[k] = v
		} else { // there's already previous measuremnts, modify in place
			currentMeasurements.min = min(currentMeasurements.min, v.min)
			currentMeasurements.max = max(currentMeasurements.max, v.max)
			currentMeasurements.sum += v.sum
			currentMeasurements.count += v.count
		}
	}
}

func (ra *ResultAggregator) ListCities() []string {
	cities := make([]string, 0, len(ra.allResults))

	for k := range ra.allResults {
		cities = append(cities, k)
	}

	return cities
}

func (ra *ResultAggregator) CalculateMetricsForCity(city string) (Metrics, error) {
	var metrics Metrics

	aggregatedData, ok := ra.allResults[city]
	if !ok {
		return metrics, fmt.Errorf("city not found: %s", city)
	}

	metrics.max = aggregatedData.max
	metrics.min = aggregatedData.min
	metrics.avg = aggregatedData.sum / float64(aggregatedData.count)

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

func ProcessSection(reader io.ReaderAt, chunk Section, bufferSize int) (*MeasurementAggregator, error) {
	recordGenerator := NewRecordGenerator(reader, chunk, bufferSize, '\n')
	aggregator := NewMeasurementAggregator()

	// read and aggregate data
	for {
		rawRec, err := recordGenerator.ReadRecord()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("failed reading record: %w", err)
		}

		record, err := ParseRecord(rawRec)
		if err != nil {
			return nil, fmt.Errorf("failed parsing record '%s': %w", rawRec, err)
		}

		aggregator.AddRecord(record)

	}

	return &aggregator, nil
}

func Execute(inputPath string, outputPath string, bufferSize int, numWorkers int) error {

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open file at %s: %w", inputPath, err)
	}
	defer inputFile.Close()

	info, err := inputFile.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := info.Size()

	chunks, err := CalculateSections(inputFile, fileSize, 128, '\n', numWorkers)
	if err != nil {
		return fmt.Errorf("failed to creat chunks from file: %w", err)
	}

	type partialResult struct {
		res *MeasurementAggregator
		err error
	}
	resultsChan := make(chan partialResult, len(chunks))
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)

		go func(c Section) {
			defer wg.Done()
			res, err := ProcessSection(inputFile, c, bufferSize)

			resultsChan <- partialResult{res: res, err: err}
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	errCount := 0
	resultAgg := NewResultAggregator()

	for msg := range resultsChan {
		// Unpack and check the error first
		if msg.err != nil {
			fmt.Printf("Worker error: %v\n", msg.err)
			errCount++
			continue // Skip aggregating this failed record
		}

		// Because we checked the error, it's now safe to use msg.res
		resultAgg.AddPartialResults(msg.res.cityMeasurements)
	}

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer outputFile.Close()

	// write output

	cities := resultAgg.ListCities()
	slices.Sort(cities)

	var sb strings.Builder

	sb.WriteString("{")

	for i, city := range cities {
		metrics, err := resultAgg.CalculateMetricsForCity(city)
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

	sb.WriteString("}\n")

	results := sb.String()

	_, err = outputFile.WriteString(results)
	if err != nil {
		panic(err)
	}
	return nil
}

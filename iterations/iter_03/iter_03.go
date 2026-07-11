package iter03

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

type Chunk struct {
	start int64
	end   int64
}

func CalculateChunkBoundaries(reader io.ReaderAt, dataSize int64, bufferSize int, separator byte, numChunks int) ([]Chunk, error) {
	chunks := make([]Chunk, numChunks)
	chunkSize := dataSize / int64(numChunks)

	start := int64(0)
	for i := range numChunks {
		end := dataSize
		if i < numChunks-1 {
			var err error
			end, err = nextRecordBoundary(reader, start+chunkSize, bufferSize, separator)
			if err != nil {
				return nil, err
			}
		}

		chunks[i] = Chunk{start: start, end: end}
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

type ChunkReader struct {
	reader     io.ReaderAt
	offset     int64
	chunkEnd   int64
	bufferSize int
	hasNext    bool
	separator  byte
}

// bufferSize must be greater than record size
func NewChunkReader(reader io.ReaderAt, chunk Chunk, bufferSize int, separator byte) *ChunkReader {
	return &ChunkReader{
		reader:     reader,
		offset:     chunk.start,
		chunkEnd:   chunk.end,
		bufferSize: bufferSize,
		separator:  separator,
		hasNext:    true,
	}
}

func (chr *ChunkReader) HasNext() bool {
	return chr.hasNext
}

func (chr *ChunkReader) ReadNextChunk() ([]byte, error) {
	readLimit := min(chr.bufferSize, int(chr.chunkEnd-chr.offset))
	buffer := make([]byte, readLimit)

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

	if chr.offset >= chr.chunkEnd {
		chr.hasNext = false
	}

	return adjustedChunk, nil
}

type RecordGenerator struct {
	chunk     []byte
	offset    int
	separator byte
	hasNext   bool
}

func NewRecordGenerator(chunk []byte, separator byte) *RecordGenerator {
	return &RecordGenerator{
		chunk:     chunk,
		offset:    0,
		separator: separator,
		hasNext:   true,
	}
}

func (rg *RecordGenerator) ReadNextRecord() ([]byte, error) {

	recordStart := rg.offset
	relativeEnd := bytes.Index(rg.chunk[rg.offset:], []byte{rg.separator})

	// handle if separator not found
	if relativeEnd == -1 {
		return nil, fmt.Errorf("failed to find separator in chunk")
	}

	// bytes.Index returns a position relative to the rg.chunk[rg.offset:] slice,
	// so it must be shifted back by rg.offset to index into rg.chunk itself
	recordEnd := rg.offset + relativeEnd
	// handle reaching end of chunk
	if recordEnd+1 == len(rg.chunk) {
		rg.hasNext = false
	}

	// updating offset
	rg.offset = recordEnd + 1

	return rg.chunk[recordStart:recordEnd], nil
}

func (rg *RecordGenerator) HasNext() bool {
	return rg.hasNext
}

type Record struct {
	station []byte
	temp    float64
}

func ParseRecord(rawRecord []byte) (Record, error) {
	var record Record

	separatorIdx := bytes.Index(rawRecord, []byte(";"))
	if separatorIdx == -1 {
		return record, fmt.Errorf("separator ';' not found in record: %s", rawRecord)
	}

	record.station = rawRecord[:separatorIdx]

	temp, err := strconv.ParseFloat(string(rawRecord[separatorIdx+1:]), 64)
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

type AggregatedMeasurements struct {
	min   float64
	max   float64
	sum   float64
	count int
}

type MeasurementAggregator struct {
	cityMeasurements map[string]AggregatedMeasurements
}

func NewMeasurementAggregator() MeasurementAggregator {
	cityMeasurements := make(map[string]AggregatedMeasurements)

	return MeasurementAggregator{cityMeasurements: cityMeasurements}
}

func (a *MeasurementAggregator) AddRecord(record Record) {

	aggMeasurement, ok := a.cityMeasurements[string(record.station)]

	if !ok { // no previous measurement for the city
		aggMeasurement.min = record.temp
		aggMeasurement.max = record.temp
		aggMeasurement.sum = record.temp
		aggMeasurement.count = 1
	} else { // there's already previous measuremnts
		aggMeasurement.min = min(aggMeasurement.min, record.temp)
		aggMeasurement.max = max(aggMeasurement.max, record.temp)
		aggMeasurement.sum += record.temp
		aggMeasurement.count++
	}

	a.cityMeasurements[string(record.station)] = aggMeasurement

}

func (a *MeasurementAggregator) ListCities() []string {
	cities := make([]string, 0, len(a.cityMeasurements))

	for k := range a.cityMeasurements {
		cities = append(cities, k)
	}

	return cities
}

func (a *MeasurementAggregator) CalculateMetricsForCity(city string) (Metrics, error) {
	var metrics Metrics

	aggregatedData, ok := a.cityMeasurements[city]
	if !ok {
		return metrics, fmt.Errorf("city not found: %s", city)
	}

	metrics.max = aggregatedData.max
	metrics.min = aggregatedData.min
	metrics.avg = aggregatedData.sum / float64(aggregatedData.count)

	return metrics, nil
}

type ResultAggregator struct {
	allResults map[string]AggregatedMeasurements
}

func NewResultAggregator() ResultAggregator {
	allResults := make(map[string]AggregatedMeasurements)

	return ResultAggregator{allResults: allResults}
}

func (ra *ResultAggregator) AddPartialResults(partialResults map[string]AggregatedMeasurements) {
	for k, v := range partialResults {
		currentMeasurements, ok := ra.allResults[k]
		if !ok { // no previous measurement for the city
			currentMeasurements = v
		} else { // there's already previous measuremnts
			currentMeasurements.min = min(currentMeasurements.min, v.min)
			currentMeasurements.max = max(currentMeasurements.max, v.max)
			currentMeasurements.sum += v.sum
			currentMeasurements.count += v.count
		}

		ra.allResults[k] = currentMeasurements
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

func ProcessChunk(reader io.ReaderAt, chunk Chunk, bufferSize int) (*MeasurementAggregator, error) {
	chunkReader := NewChunkReader(reader, chunk, bufferSize, '\n')
	aggregator := NewMeasurementAggregator()

	// read and aggregate data
	for chunkReader.HasNext() {
		chunk, err := chunkReader.ReadNextChunk()
		if err != nil {
			return nil, fmt.Errorf("failed reading chunk: %w", err)
		}

		recordGenerator := NewRecordGenerator(chunk, '\n')

		for recordGenerator.HasNext() {

			rawRec, err := recordGenerator.ReadNextRecord()
			if err != nil {
				return nil, fmt.Errorf("failed reading record from chunk: %w", err)
			}

			record, err := ParseRecord(rawRec)
			if err != nil {
				return nil, fmt.Errorf("failed parsing record '%s': %w", rawRec, err)
			}

			aggregator.AddRecord(record)
		}
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

	chunks, err := CalculateChunkBoundaries(inputFile, fileSize, 128, '\n', numWorkers)
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

		go func(c Chunk) {
			defer wg.Done()
			res, err := ProcessChunk(inputFile, c, bufferSize)

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

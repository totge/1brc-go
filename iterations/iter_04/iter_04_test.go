package iter04

import (
	"bytes"
	"strings"
	"testing"
)

func TestNextRecordBoundary(t *testing.T) {
	// indices:        0123 456789 01234
	//                 0  \n     \n      \n
	data := "012\n45678\n0123\n"
	reader := strings.NewReader(data)

	tests := []struct {
		name         string
		targetOffset int64
		want         int64
	}{
		{"separator sits at the target offset", 3, 4},
		{"separator later in the window", 0, 4},
		{"skips ahead to the next separator", 4, 10},
		{"last separator reached via EOF read", 14, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextRecordBoundary(reader, tt.targetOffset, len(data), '\n')
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
			// the byte right before the returned boundary must be the separator
			if got >= 1 && data[got-1] != '\n' {
				t.Errorf("boundary %d is not immediately after a separator", got)
			}
		})
	}
}

func TestNextRecordBoundary_SeparatorBeyondBuffer(t *testing.T) {
	// the only separator is at index 6, but the buffer only covers 3 bytes
	reader := strings.NewReader("abcdef\n")

	_, err := nextRecordBoundary(reader, 0, 3, '\n')
	if err == nil {
		t.Errorf("expected error when separator lies beyond the buffer window, got nil")
	}
}

func TestNextRecordBoundary_NoSeparator(t *testing.T) {
	reader := strings.NewReader("no-separators-here")

	_, err := nextRecordBoundary(reader, 0, 100, '\n')
	if err == nil {
		t.Errorf("expected error when no separator is present, got nil")
	}
}

func TestNextRecordBoundary_OffsetAtEOF(t *testing.T) {
	data := "abc\n"
	reader := strings.NewReader(data)

	// reading at the end of the file yields no data and therefore no separator
	_, err := nextRecordBoundary(reader, int64(len(data)), 10, '\n')
	if err == nil {
		t.Errorf("expected error when reading at EOF, got nil")
	}
}

func TestCalculateChunkBoundaries(t *testing.T) {
	// 30 bytes, a separator at every third byte (indices 2, 5, ... 29)
	data := strings.Repeat("ab\n", 10)
	reader := strings.NewReader(data)
	dataSize := int64(len(data))

	tests := []struct {
		name      string
		numChunks int
		want      []Chunk
	}{
		{"single chunk covers the whole file", 1, []Chunk{{0, 30}}},
		{"two chunks", 2, []Chunk{{0, 18}, {18, 30}}},
		{"three chunks", 3, []Chunk{{0, 12}, {12, 24}, {24, 30}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateChunkBoundaries(reader, dataSize, len(data), '\n', tt.numChunks)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d chunks, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("chunk %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}

			validateChunks(t, got, data, dataSize, '\n')
		})
	}
}

func TestCalculateChunkBoundaries_SeparatorNotFound(t *testing.T) {
	// no separators at all, so the boundary lookup for the first chunk must fail
	data := strings.Repeat("x", 100)
	reader := strings.NewReader(data)

	_, err := CalculateChunkBoundaries(reader, int64(len(data)), 10, '\n', 2)
	if err == nil {
		t.Errorf("expected error when a chunk boundary cannot be found, got nil")
	}
}

// validateChunks asserts the structural invariants every chunk set must satisfy:
// full coverage of the file, no gaps or overlaps, and every internal boundary
// landing immediately after a separator.
func validateChunks(t *testing.T, chunks []Chunk, data string, dataSize int64, separator byte) {
	t.Helper()

	if len(chunks) == 0 {
		t.Fatalf("expected at least one chunk")
	}
	if chunks[0].start != 0 {
		t.Errorf("first chunk must start at 0, got %d", chunks[0].start)
	}
	if chunks[len(chunks)-1].end != dataSize {
		t.Errorf("last chunk must end at dataSize %d, got %d", dataSize, chunks[len(chunks)-1].end)
	}

	for i, c := range chunks {
		if i > 0 && c.start != chunks[i-1].end {
			t.Errorf("chunk %d starts at %d but previous chunk ends at %d (gap or overlap)", i, c.start, chunks[i-1].end)
		}
		// every boundary except the final one must sit right after a separator
		if i < len(chunks)-1 {
			if c.end < 1 || data[c.end-1] != separator {
				t.Errorf("chunk %d end %d is not immediately after a separator", i, c.end)
			}
		}
	}
}

func TestChunkReader_ReadNextChunk(t *testing.T) {
	// indices: 012\n=0-3, 45678\n=4-9, 0123\n=10-14, 5\n=15-16, 78\n=17-19, 9\n=20-21
	reader := strings.NewReader("012\n45678\n0123\n5\n78\n9\n")

	// only read the middle of the file: the chunk covers "45678\n0123\n5\n"
	chunk := Chunk{start: 4, end: 17}
	chunkReader := NewChunkReader(reader, chunk, 6, '\n')

	tests := []struct {
		name          string
		expectData    string
		expectHasNext bool
	}{
		{"First", "45678\n", true},
		{"Second", "0123\n", true},
		{"Last", "5\n", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if !chunkReader.HasNext() {
				t.Fatalf("expected HasNext() to be true before reading %q", testCase.expectData)
			}
			got, err := chunkReader.ReadNextChunk()
			if err != nil {
				t.Fatalf("unxpected failure when reading data: %v", err)
			}
			if string(got) != testCase.expectData {
				t.Errorf("got %q, want %q", string(got), testCase.expectData)
			}
			if chunkReader.HasNext() != testCase.expectHasNext {
				t.Errorf("ChunkReader HasNext() got %t, want %t", chunkReader.HasNext(), testCase.expectHasNext)
			}
		})

	}

}

func TestRecordGenerator_ReadNextRecord(t *testing.T) {
	chunk := []byte("abc\ndefg\nhi\n")

	recordGenerator := NewRecordGenerator(chunk, '\n')

	tests := []struct {
		name          string
		expectRecord  string
		expectHasNext bool
	}{
		{"First", "abc", true},
		{"Second", "defg", true},
		{"Third", "hi", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := recordGenerator.ReadNextRecord()
			if err != nil {
				t.Fatalf("unexpected failure when reading record: %v", err)
			}
			if string(got) != testCase.expectRecord {
				t.Errorf("got %q, want %q", string(got), testCase.expectRecord)
			}
			if recordGenerator.HasNext() != testCase.expectHasNext {
				t.Errorf("RecordGenerator HasNext() got %t, want %t", recordGenerator.HasNext(), testCase.expectHasNext)
			}
		})
	}
}

func TestRecordGenerator_ReadNextRecord_SingleRecord(t *testing.T) {
	recordGenerator := NewRecordGenerator([]byte("solo\n"), '\n')

	got, err := recordGenerator.ReadNextRecord()
	if err != nil {
		t.Fatalf("unexpected failure when reading record: %v", err)
	}
	if string(got) != "solo" {
		t.Errorf("got %q, want %q", string(got), "solo")
	}
	if recordGenerator.HasNext() {
		t.Errorf("expected HasNext() to be false after the last record")
	}
}

func TestRecordGenerator_ReadNextRecord_NoSeparator(t *testing.T) {
	recordGenerator := NewRecordGenerator([]byte("noseparator"), '\n')

	_, err := recordGenerator.ReadNextRecord()
	if err == nil {
		t.Errorf("expected error when no separator is present, got nil")
	}
}

func TestParseRecord(t *testing.T) {
	// records arrive from RecordGenerator without the trailing separator, so
	// ParseRecord must not assume one is present.
	tests := []struct {
		name    string
		input   []byte
		want    Record
		wantErr bool
	}{
		{
			name:    "valid record",
			input:   []byte("Hamburg;12.3"),
			want:    Record{station: []byte("Hamburg"), temp: 12.3},
			wantErr: false,
		},
		{
			name:    "negative temperature",
			input:   []byte("Oslo;-5.5"),
			want:    Record{station: []byte("Oslo"), temp: -5.5},
			wantErr: false,
		},
		{
			name:    "single fractional digit is preserved",
			input:   []byte("Rome;9.9"),
			want:    Record{station: []byte("Rome"), temp: 9.9},
			wantErr: false,
		},
		{
			name:    "missing separator",
			input:   []byte("Hamburg12.3"),
			wantErr: true,
		},
		{
			name:    "invalid float",
			input:   []byte("Hamburg;notafloat"),
			wantErr: true,
		},
		{
			name:    "empty temperature",
			input:   []byte("Hamburg;"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRecord(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRecord() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if !bytes.Equal(got.station, tt.want.station) {
					t.Errorf("got station %q, want %q", got.station, tt.want.station)
				}
				if got.temp != tt.want.temp {
					t.Errorf("got temp %v, want %v", got.temp, tt.want.temp)
				}
			}
		})
	}
}

// TestRecordGenerator_ParseRecord_PreservesDecimals guards the real pipeline:
// records leave RecordGenerator without a trailing separator, and ParseRecord
// must parse the temperature in full. A regression here silently truncated the
// last fractional digit (e.g. 12.3 -> 12.0).
func TestRecordGenerator_ParseRecord_PreservesDecimals(t *testing.T) {
	chunk := []byte("Hamburg;12.3\nOslo;-5.5\nRome;9.9\n")

	want := []Record{
		{station: []byte("Hamburg"), temp: 12.3},
		{station: []byte("Oslo"), temp: -5.5},
		{station: []byte("Rome"), temp: 9.9},
	}

	rg := NewRecordGenerator(chunk, '\n')

	for i := 0; rg.HasNext(); i++ {
		rawRec, err := rg.ReadNextRecord()
		if err != nil {
			t.Fatalf("unexpected error reading record %d: %v", i, err)
		}

		got, err := ParseRecord(rawRec)
		if err != nil {
			t.Fatalf("unexpected error parsing record %q: %v", rawRec, err)
		}

		if !bytes.Equal(got.station, want[i].station) {
			t.Errorf("record %d: got station %q, want %q", i, got.station, want[i].station)
		}
		if got.temp != want[i].temp {
			t.Errorf("record %d: got temp %v, want %v (decimal part lost?)", i, got.temp, want[i].temp)
		}
	}
}

func TestNewAggregator(t *testing.T) {
	a := NewMeasurementAggregator()
	cities := a.ListCities()
	if len(cities) != 0 {
		t.Errorf("expected empty aggregator, got %d cities", len(cities))
	}
}

func TestAggregator_AddRecord(t *testing.T) {
	a := NewMeasurementAggregator()
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 12.3})
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 5.0})
	a.AddRecord(Record{station: []byte("Oslo"), temp: -3.0})

	keysCount := 0
	for range a.cityMeasurements {
		keysCount++
	}
	if keysCount != 2 {
		t.Errorf("expected 2 keys in map, got %d", keysCount)
	}

	if measurements, ok := a.cityMeasurements["Hamburg"]; !ok {
		t.Error("city not added to map")
		if measurements.count != 2 {
			t.Errorf("expected 2 measurements for Hamburg, got %d", measurements.count)
		}
	}
	if measurements, ok := a.cityMeasurements["Oslo"]; !ok {
		t.Error("city not added to map")
		if measurements.count != 1 {
			t.Errorf("expected 1 measurements for Oslo, got %d", measurements.count)
		}
	}
}

func TestAggregator_ListCities(t *testing.T) {
	a := NewMeasurementAggregator()
	//TODO: is it okay to use other mthods for these unitests?
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 10.0})
	a.AddRecord(Record{station: []byte("Oslo"), temp: -1.0})
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 5.0})

	cities := a.ListCities()
	if len(cities) != 2 {
		t.Errorf("expected 2 cities, got %d", len(cities))
	}

	citySet := make(map[string]bool)
	for _, c := range cities {
		citySet[c] = true
	}
	if !citySet["Hamburg"] {
		t.Errorf("expected Hamburg in cities")
	}
	if !citySet["Oslo"] {
		t.Errorf("expected Oslo in cities")
	}
}

func TestAggregator_CalculateMetricsForCity(t *testing.T) {
	a := NewMeasurementAggregator()
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 10.0})
	a.AddRecord(Record{station: []byte("Hamburg"), temp: -2.0})
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 6.0})

	metrics, err := a.CalculateMetricsForCity("Hamburg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.min != -2.0 {
		t.Errorf("got min %v, want -2.0", metrics.min)
	}
	if metrics.max != 10.0 {
		t.Errorf("got max %v, want 10.0", metrics.max)
	}
	expectedAvg := 14.0 / 3.0
	if metrics.avg != expectedAvg {
		t.Errorf("got avg %v, want %v", metrics.avg, expectedAvg)
	}
}

func TestReasultAggregator_CalculateMetricsForCity(t *testing.T) {
	a1 := NewMeasurementAggregator()
	a1.AddRecord(Record{station: []byte("Hamburg"), temp: 10.1})
	a1.AddRecord(Record{station: []byte("Hamburg"), temp: -2.4})
	a1.AddRecord(Record{station: []byte("Hamburg"), temp: 6.6})
	// count=3, sum=14.3, min=-2.4, max=10.1

	a2 := NewMeasurementAggregator()
	a2.AddRecord(Record{station: []byte("Hamburg"), temp: -3.0})
	a2.AddRecord(Record{station: []byte("Hamburg"), temp: 7.2})
	// count=2, sum=4.2, min=-3.0, max=7.2
	a2.AddRecord(Record{station: []byte("Oslo"), temp: 7.2})

	resAgg := NewResultAggregator()

	resAgg.AddPartialResults(a1.cityMeasurements)
	resAgg.AddPartialResults(a2.cityMeasurements)

	metrics, err := resAgg.CalculateMetricsForCity("Hamburg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.min != -3.0 {
		t.Errorf("got min %v, want -2.0", metrics.min)
	}
	if metrics.max != 10.1 {
		t.Errorf("got max %v, want 10.0", metrics.max)
	}
	expectedAvg := 18.5 / 5.0
	if metrics.avg != expectedAvg {
		t.Errorf("got avg %v, want %v", metrics.avg, expectedAvg)
	}
}

func TestAggregator_CalculateMetricsForCity_UnknownCity(t *testing.T) {
	a := NewMeasurementAggregator()

	_, err := a.CalculateMetricsForCity("NonExistent")
	if err == nil {
		t.Errorf("expected error for unknown city, got nil")
	}
}

func TestNewResultAggregator(t *testing.T) {
	ra := NewResultAggregator()
	if ra.allResults == nil {
		t.Fatal("expected initialized results map, got nil")
	}
	if len(ra.allResults) != 0 {
		t.Errorf("expected empty aggregator, got %d cities", len(ra.allResults))
	}
}

func TestResultAggregator_AddPartialResults_NewCities(t *testing.T) {
	ra := NewResultAggregator()

	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: 1.0, max: 5.0, sum: 12.0, count: 3},
		"Oslo":    {min: -4.0, max: 2.0, sum: -2.0, count: 2},
	})

	assertMeasurements(t, ra.allResults, "Hamburg", AggregatedMeasurements{min: 1.0, max: 5.0, sum: 12.0, count: 3})
	assertMeasurements(t, ra.allResults, "Oslo", AggregatedMeasurements{min: -4.0, max: 2.0, sum: -2.0, count: 2})

	if len(ra.allResults) != 2 {
		t.Errorf("expected 2 cities, got %d", len(ra.allResults))
	}
}

func TestResultAggregator_AddPartialResults_MergeOverlappingCity(t *testing.T) {
	ra := NewResultAggregator()

	// two partial results for the same city produced by different workers
	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: 3.0, max: 10.0, sum: 20.0, count: 4},
	})
	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: -1.0, max: 8.0, sum: 15.0, count: 3},
	})

	// min/max take the extremes, sum and count accumulate
	assertMeasurements(t, ra.allResults, "Hamburg", AggregatedMeasurements{min: -1.0, max: 10.0, sum: 35.0, count: 7})

	if len(ra.allResults) != 1 {
		t.Errorf("expected 1 city after merging, got %d", len(ra.allResults))
	}
}

func TestResultAggregator_AddPartialResults_MixedMerge(t *testing.T) {
	ra := NewResultAggregator()

	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: 3.0, max: 10.0, sum: 20.0, count: 4},
		"Oslo":    {min: -4.0, max: 2.0, sum: -2.0, count: 2},
	})
	// second batch overlaps on Hamburg and introduces a brand new city
	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: 5.0, max: 12.0, sum: 30.0, count: 3},
		"Rome":    {min: 15.0, max: 25.0, sum: 60.0, count: 3},
	})

	assertMeasurements(t, ra.allResults, "Hamburg", AggregatedMeasurements{min: 3.0, max: 12.0, sum: 50.0, count: 7})
	assertMeasurements(t, ra.allResults, "Oslo", AggregatedMeasurements{min: -4.0, max: 2.0, sum: -2.0, count: 2})
	assertMeasurements(t, ra.allResults, "Rome", AggregatedMeasurements{min: 15.0, max: 25.0, sum: 60.0, count: 3})

	if len(ra.allResults) != 3 {
		t.Errorf("expected 3 cities, got %d", len(ra.allResults))
	}
}

func TestResultAggregator_AddPartialResults_Empty(t *testing.T) {
	ra := NewResultAggregator()
	ra.AddPartialResults(map[string]AggregatedMeasurements{
		"Hamburg": {min: 1.0, max: 5.0, sum: 6.0, count: 2},
	})

	// merging an empty partial result must leave existing data untouched
	ra.AddPartialResults(map[string]AggregatedMeasurements{})

	assertMeasurements(t, ra.allResults, "Hamburg", AggregatedMeasurements{min: 1.0, max: 5.0, sum: 6.0, count: 2})
	if len(ra.allResults) != 1 {
		t.Errorf("expected 1 city, got %d", len(ra.allResults))
	}
}

// assertMeasurements checks that a city exists in the results map and its
// aggregated measurements match the expected values exactly.
func assertMeasurements(t *testing.T, results map[string]AggregatedMeasurements, city string, want AggregatedMeasurements) {
	t.Helper()

	got, ok := results[city]
	if !ok {
		t.Fatalf("expected city %q in results, but it is missing", city)
	}
	if got != want {
		t.Errorf("city %q: got %+v, want %+v", city, got, want)
	}
}

func TestFormatMetrics(t *testing.T) {
	expected := "Budapest=-13.2/21.4/41.0"
	got := FormatMetrics("Budapest", Metrics{min: -13.245, avg: 21.35, max: 41.0})
	if got != expected {
		t.Errorf("failed to format metrics properly, expected: %s, got: %s", expected, got)
	}
}

func TestRoundToOneDecimal(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{
			name:  "positive non-tie up",
			input: 2.98,
			want:  3.0,
		},
		{
			name:  "positive non-tie down",
			input: 2.33,
			want:  2.3,
		},
		{
			name:  "negative non-tie up",
			input: -1.33,
			want:  -1.3,
		},
		{
			name:  "negative non-tie down",
			input: -4.77,
			want:  -4.8,
		},
		{
			name:  "positive tie",
			input: 1.65,
			want:  1.7,
		},
		{
			name:  "negative tie",
			input: -3.35,
			want:  -3.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundToOneDecimal(tt.input)

			if got != tt.want {
				t.Errorf("got %f, want %f", got, tt.want)
			}

		})
	}
}

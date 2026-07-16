package iter06

import (
	"bytes"
	"errors"
	"io"
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
		want      []Section
	}{
		// Section values are {start, length}
		{"single chunk covers the whole file", 1, []Section{{0, 30}}},
		{"two chunks", 2, []Section{{0, 18}, {18, 12}}},
		{"three chunks", 3, []Section{{0, 12}, {12, 12}, {24, 6}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateSections(reader, dataSize, len(data), '\n', tt.numChunks)
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

			validateSections(t, got, data, dataSize, '\n')
		})
	}
}

func TestCalculateChunkBoundaries_SeparatorNotFound(t *testing.T) {
	// no separators at all, so the boundary lookup for the first chunk must fail
	data := strings.Repeat("x", 100)
	reader := strings.NewReader(data)

	_, err := CalculateSections(reader, int64(len(data)), 10, '\n', 2)
	if err == nil {
		t.Errorf("expected error when a chunk boundary cannot be found, got nil")
	}
}

// validateChunks asserts the structural invariants every chunk set must satisfy:
// full coverage of the file, no gaps or overlaps, and every internal boundary
// landing immediately after a separator.
func validateSections(t *testing.T, sections []Section, data string, dataSize int64, separator byte) {
	t.Helper()

	if len(sections) == 0 {
		t.Fatalf("expected at least one section")
	}
	if sections[0].start != 0 {
		t.Errorf("first chunk must start at 0, got %d", sections[0].start)
	}
	lastSectionEnd := sections[len(sections)-1].start + sections[len(sections)-1].length
	if lastSectionEnd != dataSize {
		t.Errorf("last chunk must end at dataSize %d, got %d", dataSize, lastSectionEnd)
	}

	for i, s := range sections {
		if i > 0 && s.start != sections[i-1].start+sections[i-1].length {
			t.Errorf("chunk %d starts at %d but previous chunk ends at %d (gap or overlap)", i, s.start, sections[i-1].start+sections[i-1].length)
		}
		// every boundary except the final one must sit right after a separator
		if i < len(sections) {
			if s.start+s.length < 1 || data[s.start+s.length-1] != separator {
				t.Errorf("chunk %d end %d is not immediately after a separator", i, s.start+s.length)
			}
		}
	}
}

// TestRecordGenerator_ReadRecord_AcrossBufferRefills reads only the middle
// section of the file with a buffer smaller than the section, forcing the
// generator to refill its buffer (readNextChunk) several times.
func TestRecordGenerator_ReadRecord_AcrossBufferRefills(t *testing.T) {
	// indices: 012\n=0-3, 45678\n=4-9, 0123\n=10-14, 5\n=15-16, 78\n=17-19, 9\n=20-21
	reader := strings.NewReader("012\n45678\n0123\n5\n78\n9\n")

	// the section covers "45678\n0123\n5\n" -> start 4, length 13
	section := Section{start: 4, length: 13}
	// a 6-byte buffer cannot hold the whole section, so it must refill
	rg := NewRecordGenerator(reader, section, 6, '\n')

	want := []string{"45678", "0123", "5"}
	for i, w := range want {
		got, err := rg.ReadRecord()
		if err != nil {
			t.Fatalf("record %d: unexpected error: %v", i, err)
		}
		if string(got) != w {
			t.Errorf("record %d: got %q, want %q", i, string(got), w)
		}
	}

	// the section is exhausted, so the next read must report EOF
	if _, err := rg.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF after the last record, got %v", err)
	}
}

func TestRecordGenerator_ReadRecord_WholeSection(t *testing.T) {
	data := "abc\ndefg\nhi\n"
	reader := strings.NewReader(data)
	section := Section{start: 0, length: int64(len(data))}
	rg := NewRecordGenerator(reader, section, 64, '\n')

	want := []string{"abc", "defg", "hi"}
	for i, w := range want {
		got, err := rg.ReadRecord()
		if err != nil {
			t.Fatalf("record %d: unexpected error: %v", i, err)
		}
		if string(got) != w {
			t.Errorf("record %d: got %q, want %q", i, string(got), w)
		}
	}

	if _, err := rg.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF after the last record, got %v", err)
	}
}

func TestRecordGenerator_ReadRecord_SingleRecord(t *testing.T) {
	data := "solo\n"
	reader := strings.NewReader(data)
	rg := NewRecordGenerator(reader, Section{start: 0, length: int64(len(data))}, 64, '\n')

	got, err := rg.ReadRecord()
	if err != nil {
		t.Fatalf("unexpected failure when reading record: %v", err)
	}
	if string(got) != "solo" {
		t.Errorf("got %q, want %q", string(got), "solo")
	}

	if _, err := rg.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF after the last record, got %v", err)
	}
}

func TestRecordGenerator_ReadRecord_NoSeparator(t *testing.T) {
	data := "noseparator"
	reader := strings.NewReader(data)
	rg := NewRecordGenerator(reader, Section{start: 0, length: int64(len(data))}, 64, '\n')

	if _, err := rg.ReadRecord(); err == nil {
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

func TestParseTemperature(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    float64
		wantErr bool
	}{
		{
			name:    "4 digit positive",
			input:   []byte("12.3"),
			want:    12.3,
			wantErr: false,
		},
		{
			name:    "4 digit negative",
			input:   []byte("-54.5"),
			want:    -54.5,
			wantErr: false,
		},
		{
			name:    "3 digit positive",
			input:   []byte("7.9"),
			want:    7.9,
			wantErr: false,
		},
		{
			name:    "3 digit negative",
			input:   []byte("-3.4"),
			want:    -3.4,
			wantErr: false,
		},
		{
			name:    "invalid length",
			input:   []byte("-100.5"),
			wantErr: true,
		},
		{
			name:    "empty temperature",
			input:   []byte(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTemperature(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTemperature() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if got != tt.want {
					t.Errorf("got %f, want %f", got, tt.want)
				}
			}
		})
	}
}

// TestRecordGenerator_ParseRecord_PreservesDecimals guards the real pipeline:
// records leave RecordGenerator without a trailing separator, and ParseRecord
// must parse the temperature in full. A regression here silently truncated the
// last fractional digit (e.g. 12.3 -> 12.0).
// func TestRecordGenerator_ParseRecord_PreservesDecimals(t *testing.T) {
// 	chunk := []byte("Hamburg;12.3\nOslo;-5.5\nRome;9.9\n")

// 	want := []Record{
// 		{station: []byte("Hamburg"), temp: 12.3},
// 		{station: []byte("Oslo"), temp: -5.5},
// 		{station: []byte("Rome"), temp: 9.9},
// 	}

// 	rg := NewRecordGenerator(chunk, '\n')

// 	for i := 0; rg.HasNext(); i++ {
// 		rawRec, err := rg.ReadNextRecord()
// 		if err != nil {
// 			t.Fatalf("unexpected error reading record %d: %v", i, err)
// 		}

// 		got, err := ParseRecord(rawRec)
// 		if err != nil {
// 			t.Fatalf("unexpected error parsing record %q: %v", rawRec, err)
// 		}

// 		if !bytes.Equal(got.station, want[i].station) {
// 			t.Errorf("record %d: got station %q, want %q", i, got.station, want[i].station)
// 		}
// 		if got.temp != want[i].temp {
// 			t.Errorf("record %d: got temp %v, want %v (decimal part lost?)", i, got.temp, want[i].temp)
// 		}
// 	}
// }

func TestNewAggregator(t *testing.T) {
	a := NewMeasurementAggregator()

	if len(a.cityMeasurements) != 0 {
		t.Errorf("expected empty aggregator, got %d cities", len(a.cityMeasurements))
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

	a.AddRecord(Record{station: []byte("Hamburg"), temp: 10.0})
	a.AddRecord(Record{station: []byte("Oslo"), temp: -1.0})
	a.AddRecord(Record{station: []byte("Hamburg"), temp: 5.0})

	if len(a.cityMeasurements) != 2 {
		t.Errorf("expected 2 cities, got %d", len(a.cityMeasurements))
	}

	citySet := make(map[string]bool)
	for c := range a.cityMeasurements {
		citySet[c] = true
	}
	if !citySet["Hamburg"] {
		t.Errorf("expected Hamburg in cities")
	}
	if !citySet["Oslo"] {
		t.Errorf("expected Oslo in cities")
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

	ra.AddPartialResults(map[string]*AggregatedMeasurements{
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
	ra.AddPartialResults(map[string]*AggregatedMeasurements{
		"Hamburg": {min: 3.0, max: 10.0, sum: 20.0, count: 4},
	})
	ra.AddPartialResults(map[string]*AggregatedMeasurements{
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

	ra.AddPartialResults(map[string]*AggregatedMeasurements{
		"Hamburg": {min: 3.0, max: 10.0, sum: 20.0, count: 4},
		"Oslo":    {min: -4.0, max: 2.0, sum: -2.0, count: 2},
	})
	// second batch overlaps on Hamburg and introduces a brand new city
	ra.AddPartialResults(map[string]*AggregatedMeasurements{
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
	ra.AddPartialResults(map[string]*AggregatedMeasurements{
		"Hamburg": {min: 1.0, max: 5.0, sum: 6.0, count: 2},
	})

	// merging an empty partial result must leave existing data untouched
	ra.AddPartialResults(map[string]*AggregatedMeasurements{})

	assertMeasurements(t, ra.allResults, "Hamburg", AggregatedMeasurements{min: 1.0, max: 5.0, sum: 6.0, count: 2})
	if len(ra.allResults) != 1 {
		t.Errorf("expected 1 city, got %d", len(ra.allResults))
	}
}

// assertMeasurements checks that a city exists in the results map and its
// aggregated measurements match the expected values exactly.
func assertMeasurements(t *testing.T, results map[string]*AggregatedMeasurements, city string, want AggregatedMeasurements) {
	t.Helper()

	got, ok := results[city]
	if !ok {
		t.Fatalf("expected city %q in results, but it is missing", city)
	}
	if *got != want {
		t.Errorf("city %q: got %+v, want %+v", city, *got, want)
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

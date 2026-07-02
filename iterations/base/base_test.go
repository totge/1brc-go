package base

import (
	"strings"
	"testing"
)

func TestChunkReader_ReadNextChunk(t *testing.T) {
	reader := strings.NewReader("012\n45678\n0123\n")

	chunkReader := NewChunkReader(reader, 6, '\n')

	tests := []struct {
		name          string
		expectData    string
		expectHasNext bool
	}{
		{"First", "012\n", true},
		{"Second", "45678\n", true},
		{"Last", "0123\n", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
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

func TestProduceRawRecords(t *testing.T) {
	input := "abc\ndefg\nhi\n"

	records := ProduceRawRecords([]byte(input), '\n')

	if len(records) != 3 {
		t.Errorf("got %d recods, want %d", len(records), 3)
	}
	if records[0] != "abc\n" {
		t.Errorf("for first record got %s , want %s", records[0], "abc\n")
	}
	if records[1] != "defg\n" {
		t.Errorf("for second record got %s , want %s", records[1], "defg\n")
	}
	if records[2] != "hi\n" {
		t.Errorf("for first record got %s , want %s", records[2], "hi\n")
	}
}

func TestParseRecord(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Record
		wantErr bool
	}{
		{
			name:    "valid record",
			input:   "Hamburg;12.3\n",
			want:    Record{station: "Hamburg", temp: 12.3},
			wantErr: false,
		},
		{
			name:    "negative temperature",
			input:   "Oslo;-5.5\n",
			want:    Record{station: "Oslo", temp: -5.5},
			wantErr: false,
		},
		{
			name:    "missing separator",
			input:   "Hamburg12.3\n",
			wantErr: true,
		},
		{
			name:    "invalid float",
			input:   "Hamburg;notafloat\n",
			wantErr: true,
		},
		{
			name:    "empty temperature",
			input:   "Hamburg;\n",
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
				if got.station != tt.want.station {
					t.Errorf("got station %q, want %q", got.station, tt.want.station)
				}
				if got.temp != tt.want.temp {
					t.Errorf("got temp %v, want %v", got.temp, tt.want.temp)
				}
			}
		})
	}
}

func TestNewAggregator(t *testing.T) {
	a := NewAggregator()
	cities := a.ListCities()
	if len(cities) != 0 {
		t.Errorf("expected empty aggregator, got %d cities", len(cities))
	}
}

func TestAggregator_AddRecord(t *testing.T) {
	a := NewAggregator()
	a.AddRecord(Record{station: "Hamburg", temp: 12.3})
	a.AddRecord(Record{station: "Hamburg", temp: 5.0})
	a.AddRecord(Record{station: "Oslo", temp: -3.0})

	keysCount := 0
	for range a.cityMeasurements {
		keysCount++
	}
	if keysCount != 2 {
		t.Errorf("expected 2 keys in map, got %d", keysCount)
	}

	if measurements, ok := a.cityMeasurements["Hamburg"]; !ok {
		t.Error("city not added to map")
		if len(measurements) != 2 {
			t.Errorf("expected 2 measurements for Hamburg, got %d", len(measurements))
		}
	}
	if measurements, ok := a.cityMeasurements["Oslo"]; !ok {
		t.Error("city not added to map")
		if len(measurements) != 1 {
			t.Errorf("expected 1 measurements for Oslo, got %d", len(measurements))
		}
	}
}

func TestAggregator_ListCities(t *testing.T) {
	a := NewAggregator()
	//TODO: is it okay to use other mthods for these unitests?
	a.AddRecord(Record{station: "Hamburg", temp: 10.0})
	a.AddRecord(Record{station: "Oslo", temp: -1.0})
	a.AddRecord(Record{station: "Hamburg", temp: 5.0})

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
	a := NewAggregator()
	a.AddRecord(Record{station: "Hamburg", temp: 10.0})
	a.AddRecord(Record{station: "Hamburg", temp: -2.0})
	a.AddRecord(Record{station: "Hamburg", temp: 6.0})

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

func TestAggregator_CalculateMetricsForCity_UnknownCity(t *testing.T) {
	a := NewAggregator()

	_, err := a.CalculateMetricsForCity("NonExistent")
	if err == nil {
		t.Errorf("expected error for unknown city, got nil")
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

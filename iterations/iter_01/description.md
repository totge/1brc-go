## Iteration 01
Most time is spent executing `Aggregator.AddRecord` and since it tored all the data points individually it took a lot of memory to do its job.
In this iteration the goal was to speed this function up and lower the memory needs by only storing aggregated metrics.

### Implementation changes

- **Single-threaded, sequential processing.** The whole file is read and aggregated on one goroutine, start to finish.
- **Chunked reading via `io.ReaderAt`.** `ChunkReader` pulls fixed-size buffers (`bufferSize` bytes) directly from the file using `ReadAt` at an increasing offset, rather than streaming through `bufio.Reader`.
- **Record-boundary alignment.** After each read, the reader scans backward from the end of the buffer for the last `\n` and trims the chunk there, so no record is ever split across chunk boundaries. The leftover bytes are re-read as part of the next chunk (the offset only advances past the last full record).
- **String allocation per record.** `ProduceRawRecords` splits a chunk into individual `"station;temp\n"` strings, and `ParseRecord` further slices/parses each one (`strings.Index` for the separator, `strconv.ParseFloat` for the temperature). Every record incurs its own string/slice allocations.
- **Unbounded per-station accumulation.** `Aggregator` stores every parsed temperature in a `map[string][]float64`, keyed by station — it keeps the raw measurements rather than folding them into a running min/max/sum as records arrive.
- **Metrics computed on demand.** `CalculateMetricsForCity` does a full pass over a station's slice at the end (via `slices.Min`/`slices.Max` and a manual sum loop) to derive min/avg/max, rather than tracking these incrementally during ingestion.
- **Output.** Stations are sorted alphabetically, formatted as `Station=min/avg/max` (rounded to one decimal with round-half-up via `RoundToOneDecimal`), joined into a single `{...}` line, and written to the output file.

This version favors clarity over performance: no concurrency, no incremental aggregation, and heavy use of intermediate string/slice allocations — it exists as a correctness baseline to compare later optimizations against.

## Iteration 03
The goal in this iteration is to keep relying on general, robust built-ins (standard go hash map, `strconv.ParseFloat`) even though they are clear bottlenecks now, and instead improve performance through parallelization.

Iteration 02 processed the whole file on a single goroutine: one `ChunkReader` walked the file from start to end, feeding records into one `Aggregator`. This iteration splits the file into independent regions that are read, parsed and aggregated concurrently, then merges the partial results into a single final map.

### Implementation changes
- **File splitting into chunks.** A new `Chunk` type describes a region of the file as a `start`/`end` offset pair. `CalculateChunkBoundaries` divides the file into `numWorkers` roughly equal chunks. Because a naive byte split would cut a record in half, `nextRecordBoundary` snaps every internal boundary forward to the offset just past the next separator, so each chunk begins and ends exactly on a record boundary. Chunks are contiguous and non-overlapping, together covering the whole file.

- **Chunk-bounded reading.** `ChunkReader` gained a `chunkEnd` field and now takes a `Chunk` in its constructor, so it iterates over the content of a single chunk rather than the entire file. `ReadNextChunk` caps each read with `min(bufferSize, chunkEnd - offset)` so it never reads past the chunk, and reports `HasNext() == false` once `offset` reaches `chunkEnd` (in addition to the existing EOF handling). This makes it safe for multiple readers to work on the same file concurrently, each confined to its own region.

- **Per-worker aggregation.** `MeasurementAggregator` is now created per worker. `ProcessChunk` bundles the read → generate → parse → aggregate pipeline for one chunk and returns that worker's partial `MeasurementAggregator`, keeping each worker's state fully independent so no locking is needed on the hot path.

- **Result aggregation layer.** A new `ResultAggregator` merges the partial per-worker maps into one. `AddPartialResults` folds each worker's city map into the combined map, taking the extremes for `min`/`max` and accumulating `sum` and `count`, so a city split across several chunks ends up with correct combined statistics. Final metrics and output formatting are computed from this merged map.

- **Concurrent orchestration.** `Execute` now takes a `numWorkers` parameter. It computes the chunk boundaries, launches one goroutine per chunk (each running `ProcessChunk`), and collects the partial results over a buffered channel guarded by a `sync.WaitGroup`. The main goroutine drains the channel, merges every partial result through the `ResultAggregator`, then sorts the cities and writes the formatted output as before.

The overall shape is now three layers: **split** the file into record-aligned chunks, **process** each chunk concurrently into a partial result, and **aggregate** the partial results into the final output.

## Iteration 04
This iteration doesn't chase performance; it is a cleanup pass. Iteration 03 split the reading path across two hand-rolled types — a `ChunkReader` that pulled raw byte blocks from the file and a `RecordGenerator` that carved those blocks into records — each tracking its own offsets and end conditions. The goal here is to collapse that machinery into a single, simpler type that leans on standard-library utilities instead of manual bookkeeping.

### Implementation changes
- **`Chunk` → `Section`.** The region type is renamed `Section` and now describes a region as a `start`/`length` pair instead of `start`/`end`. `CalculateChunkBoundaries` becomes `CalculateSections` and stores `length = end - start` for each region. The `length`-based shape maps directly onto `io.SectionReader`.

- **One reader type instead of two.** `ChunkReader` and the old byte-slice `RecordGenerator` are merged into a single `RecordGenerator`. It is constructed from an `io.ReaderAt` and a `Section`, and immediately wraps them in an `io.NewSectionReader`, which confines every read to that region for free — replacing the manual `min(bufferSize, chunkEnd - offset)` clamping and the `chunkEnd` field that the old reader carried around.

- **Single reusable buffer.** The buffer is allocated once in `NewRecordGenerator` and reused on every refill, instead of the old `ChunkReader` allocating a fresh `make([]byte, readLimit)` on each `ReadNextChunk` call.

- **Idiomatic separator scanning.** The manual backward loop that hunted for the last separator in a block is replaced by `bytes.LastIndexByte`, and record splitting uses `bytes.IndexByte`.

- **Record-at-a-time iteration via EOF.** The public surface is now a single `ReadRecord` method that returns the next record and transparently refills the buffer (`readNextChunk`) when the current one is drained. Completion is signalled with the standard `io.EOF` sentinel rather than a bespoke `HasNext()` flag, so callers loop with the usual `for { ... if err == io.EOF { break } }` pattern. `ProcessChunk` is renamed `ProcessSection` and drives this loop.

The reading path is now a single type that borrows sectioning, buffering-boundary handling and end-of-data signalling from the standard library, rather than reimplementing each of them by hand.

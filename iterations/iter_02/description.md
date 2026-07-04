## Iteration 02
Half of the execution time is the go runtime managing the program execution. A good amount of that belongs to memory allocation and reclaiming, so the aim in this iteration is to reduce those actions.

Just changing all strings to byte slices wasn't enough, since `ProduceRawRecords` was still building a huge `[][]byte` slice to hold every record produced from a chunk, causing a large number of allocations (and `madvise` calls as that slice grew).

### Implementation changes
- **No conversion to string.** `ParseRecord`/`Record` and downstream consumers were changed to use `[]byte` for representing records (and their parts) to prevent unnecessary allocations.
- **Single record instead of batch.** In the previous version, after reading a chunk of raw data, `ProduceRawRecords` eagerly converted the whole chunk into a list of records before any of them were parsed. This iteration replaces that call site with `RecordGenerator`, which yields records one by one, avoiding the big intermediate `[][]byte` allocation entirely.

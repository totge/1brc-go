## Iteration 05
Based on the preview iterations frame graph `AddRecord` was the biggest bottleneck in the program. So far aggregator maps stored the `AggregatedMeasurements` struct by value, so every update had to read the struct out of the map, modify a local copy, and write the whole struct back into the map — a second hash-and-store on every single record. This iteration stores a pointer to the struct instead, so an existing city can be updated in place with no write-back.

### Implementation changes
- **Maps hold pointers.** `MeasurementAggregator.cityMeasurements` and `ResultAggregator.allResults` change from `map[string]AggregatedMeasurements` to `map[string]*AggregatedMeasurements`. The map now stores the address of each city's accumulator rather than a copy of it.

- **In-place updates on the hot path.** `AddRecord` now looks the city up once; on a hit it mutates the struct through the pointer (`agg.min = min(...)`, `agg.sum += ...`, `agg.count++`) with no follow-up map assignment. Only the first time a city is seen does it allocate a new `AggregatedMeasurements` and store its pointer. This removes the per-record map write-back that the value-based version paid on every row.

- **Pointer-based merge.** `ResultAggregator.AddPartialResults` follows the same shape: for a city already present it accumulates into the existing struct in place through its pointer; for a new city it adopts the worker's pointer directly. Because each worker's partial map is discarded right after it is merged, adopting the pointer is safe and avoids copying the accumulator.

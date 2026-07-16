## Iteration 07
The previous iteration replaced `strconv.ParseFloat` with a specialised byte parser. Since every temperature has exactly one decimal digit, a float is unnecessary: the value can be represented exactly as an integer number of tenths. This iteration keeps everything as `int` from parsing through aggregation and only converts back to `float64` at the very end, when the final metrics are computed. Integer arithmetic is cheaper than floating point and avoids per-record float divisions.

### Implementation changes
- **`parseTemperature` returns `int` (tenths).** Instead of building a `float64` and dividing the decimal digit by 10, it composes the digits into a single scaled integer — e.g. `12.3` → `123`, `-3.4` → `-34` — reading each ASCII digit directly (`100*d0 + 10*d1 + d2`). This removes the floating-point division that ran on every one of the billion rows.

- **`Record.temp` and the accumulator are `int`.** `AggregatedMeasurements`'s `min`, `max` and `sum` change from `float64` to `int`, so `AddRecord` and the `AddPartialResults` merge do all their comparisons and accumulation in integer arithmetic.

- **Convert to float only at the end.** `CalculateMetricsForCity` scales back down when producing the final `Metrics`: `min`/`max` are divided by 10, and the average is `float64(sum) / float64(count*10)`. This is the only place floating point is used, once per city rather than once per record.

### Results
➜ [iter_07_p50    ] Time: 4.7506315s   | Mem:  505.21 MB | Profiled: true

Small improvement of 1–2 tenths of a second over the previous iteration, with memory unchanged at ~505 MB. Float parsing was already cheap after iteration 06, so moving to integers trimmed only the remaining per-record float work rather than a dominant cost.

### Conclusions
Switching to fixed-point integers shaved a little more off the parsing and aggregation cost but was not a step change — parsing is no longer where the time goes. Based on the CPU profile the map operations (hashing, mapaccess) remain the largest share of the runtime, but before addressing that, the goal in the next iteration will be to optimize the byte indexing during the record parsing and the record generation.

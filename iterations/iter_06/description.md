## Iteration 06
After the previous the CPU profile showed float parsing (`strconv.ParseFloat`) as the new biggest bottleneck. So far the program used the general-purpose `strconv.ParseFloat` parser. It took a significant share of the execution time and also forced a `[]byte`→`string` conversion on every one of the billion rows. This iteration replaces it with a custom parser that assumes the narrow, fixed shape the 1BRC data actually has.

### Implementation changes
- **Custom `parseTemperature([]byte) (float64, error)`.** The 1BRC format guarantees each temperature is in the range `-99.9 < t < 99.9`, always has exactly one decimal digit, a single `.` separator, and an optional leading `-`. That means the input is only ever 3 to 5 bytes long (`1.1` up to `-23.5`). The parser validates the length, strips an optional `-`, reads one or two integer digits, then the single decimal digit, computing the value directly from the ASCII bytes (`int(b - '0')`) instead of going through the general float machinery.

- **No `[]byte`→`string` allocation.** The parser works straight off the record's byte slice, so the per-record string conversion is required anymore.


### Results
➜ [iter_06_p50    ] Time: 4.872334708s | Mem:  505.21 MB | Profiled: true

Execution time dropped from ~7s to ~4.9s while allocated memory stayed flat at ~505 MB.

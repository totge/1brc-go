## Findings
### File reading



## Iterations
### 1. Naive idiomatic implementation
#### Description
No code optimization, nicely structured code 
#### Results
➜ [sequential_idiomatic] Time: 4m3.524779166s | Mem: 157997.18 MB
#### Conclusions
`Aggregator.AddRecord` takes the most cpu time and also allocates the most memory.

When is on the cpu profile data, out of the four minutes execution time only around 114 seconds were spent executing the actual program. This suggest thatsignificant amount of time was spent for waiting for the operating system probably because the problem uses too much memory.

-> It should only store aggregated numbers, not all data point which should lower the memory usage and also the time it takes to add each record to the hash map.

### 2. Aggregated data
#### Description
Instead of storing each data point indivually, in this version, we only store aggregated data, minimum, maximum, sum and count of the processed measurements for each city. 

#### Results
➜ [iter_01        ] Time: 1m20.463489541s | Mem: 117579.30 MB | Profiled: false

Significant improvement the program took less than half the time it took for the previous version to execute.

#### Conclusions

Based on the CPU profile around half of the execution time is spent executing the actual code and the other half is spent on the go run time managing the program. 
On the program's side, it's the map access and assignment and the float parsing that seems to take the most time. On the runtime's side it is memory related management that seems to take to most time.

The goal for the next iteration is to lower the number of steps that require memory allocation and to try release the garbage collection pressure.

### 4. Concurrent processing
#### Description


#### Results
➜ [iter_03_p50    ] Time: 11.515542542s | Mem: 22535.62 MB | Profiled: true

Significant improvement from ~80s going down to the 11-12s range consistently.

#### Conclusions

Based on the CPU profile the main proportions did not change (which was expected as there was no major change in the implementation). The biggest bottnecks seem to be map operations (byte slice to string conversion, mapaccess, mapassign), record parsing (float conversion, byte indexing), record reading (byte indexing).

The next iteration will involve a refactor to simplify the added complexity by concurrency, and use the standard library instead of handrolled solutions.

### 5. Refactor single buffer per goroutine
#### Description 
The main goal in this iteraton was not to address the biggest bottlenecks but rather to simplify the code and rely on standard library utilities like `io.SectionReader` that can reduce the handrolled lower level code while keeping the same performance as before.

The only major change in terms or the program logic is that now the `RecordGenerator` type does the the entire buffered data reading and returning the measurement records one by one. And this type has a single buffer that is used throughout its lifetime so no reallocation is needs.

#### Results
➜ [iter_04_p65    ] Time: 11.131470667s | Mem: 10026.23 MB | Profiled: false

Surprisingly to somne extent the only allocating one buffer per go routine did not have a significant effect on the performance.

#### Conclusions
Based on the profile file, the most significant bottleneck are the map operations, so the next iteration should target optimizing that part.

### 6. Store pointers in maps instead of structs
#### Description 
In the previous iteration the map operations were the biggest bottleneck, so this iteration targets them directly. Until now the aggregator maps stored the `AggregatedMeasurements` struct by value, which meant every record on the hot `AddRecord` path had to read the struct out of the map, modify a copy, and write the whole struct back — a second hash-and-store on every one of the billion rows.

This iteration changes both `MeasurementAggregator.cityMeasurements` and `ResultAggregator.allResults` to `map[string]*AggregatedMeasurements`. Now a city that already exists is updated in place through its pointer, with no write-back; only the first time a city is seen does it allocate and store a pointer. The merge step in `AddPartialResults` works the same way, accumulating into the existing struct through its pointer.

#### Results
➜ [iter_05_p50    ] Time: 7.04232575s  | Mem:  505.31 MB | Profiled: true

Significant jump in performance.

#### Conclusions
Both time and memory improved sharply: execution dropped from ~11s to ~7s, and allocated memory fell roughly 20x (from ~10 GB to ~505 MB). Removing the per-record write-back not only cut map-store work on the hot path but also drastically reduced the amount of struct copying, which shows up in the much lower memory figure.

The next iteration should target float parsing which became the new bottleneck.

### 7. Custom temperature parser for `[]byte` to `float64`
#### Description 
In the previous iteration float parsing became the biggest bottleneck, so this iteration targets it. Until now temperatures were parsed with `strconv.ParseFloat`, a general-purpose parser that handles arbitrary lengths, exponents, `NaN`/`Inf` and every valid float representation — and takes a `string`, forcing a `[]byte`→`string` conversion on every one of the billion rows.

This iteration replaces it with a custom `parseTemperature([]byte)` that assumes the fixed shape the 1BRC data actually has: a value in `-99.9 < t < 99.9`, always exactly one decimal digit, a single `.` separator and an optional leading `-`, so the input is only ever 3–5 bytes. It reads the digits directly from the ASCII bytes (`int(b - '0')`) and works straight off the record's byte slice, removing both the general float machinery and the per-record string allocation. The now-unused `strconv` import is dropped.

#### Results
➜ [iter_06_p50    ] Time: 4.872334708s | Mem:  505.21 MB | Profiled: true

Significant jump in performance — execution dropped from ~7s to ~4.9s (~30%), while memory stayed flat at ~505 MB.

#### Conclusions
Specialising the parser to the known input shape lowered its compute needs and improved the performance. Based on the cpu profile map operations (hashing, byte-slice-to-string key conversion, mapaccess) take up the biggest portion (~44%) of the runtime again, but record parsing (with temperature parsing and byte indexing) and record reading (byte indexing) are also significant (with ~27% and ~21%). Next iteration will target to switching `float64` to `int` in the parsing and aggregation steps.

### 8. Use `int` instead of `float64`
#### Description 
The previous iteration introduced a specialized float parser that was much faster than the general `strconv.ParseFloat`, but it still produced and aggregated `float64` values. Since every temperature has exactly one decimal digit, the value can be held exactly as an integer number of tenths. This iteration keeps everything as `int` from parsing through aggregation, only converting back to `float64` at the very end when the final metrics are computed.

`parseTemperature` now returns the scaled integer directly (`12.3` → `123`, `-3.4` → `-34`), dropping the per-record float division. `Record.temp` and the `min`/`max`/`sum` fields of `AggregatedMeasurements` become `int`, so `AddRecord` and the merge step do integer arithmetic. `CalculateMetricsForCity` scales back down once per city — `min`/`max` divided by 10 and the average as `sum / (count*10)` — which is the only place floating point remains.

#### Results
➜ [iter_07_p50    ] Time: 4.7506315s   | Mem:  505.21 MB | Profiled: true

Small improvement of 1-2 tenths of a second, with memory unchanged at ~505 MB.

#### Conclusions
Float parsing was already cheap after iteration 7, so moving to integers trimmed only the remaining per-record float work rather than a dominant cost — hence the modest gain. Based on the CPU profile the map operations (hashing, mapaccess) remain the largest share of the runtime, but before addressing that, the goal in the next iteration will be to optimize the byte indexing during the record parsing and the record generation.

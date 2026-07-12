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

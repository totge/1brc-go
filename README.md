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

-> It should only store aggregated numbers, not all data point which should lower the memory usage and also the time it takes to add each record to the hash map.

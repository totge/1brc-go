## Iteration 03
The goal in this iteration is to keep relying on general, robust built-ins (standard go hash map, `strconv.ParseFloat`) even though they are clear bottlenecks now and try to improve the performance by parallelization.

### Implementation changes

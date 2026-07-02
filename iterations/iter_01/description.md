## Iteration 01
Most time is spent executing `Aggregator.AddRecord` and since it tored all the data points individually it took a lot of memory to do its job.
In this iteration the goal was to speed this function up and lower the memory needs by only storing aggregated metrics.

### Implementation changes
- **Aggegated per-station accumulation.** `Aggregator` now only stores `min`, `max`, `sum` and `count` instead of storing each measurements individually.

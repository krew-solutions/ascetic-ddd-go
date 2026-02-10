# Saga Pattern

A Go implementation of the Saga pattern using the **Routing Slip** approach.

For detailed information, see the [documentation of Python version](https://krew-solutions.github.io/ascetic-ddd-python/modules/saga/index.html).

## Key Differences from Python Version

- No async/await - uses regular Go functions with `context.Context`
- Goroutines and channels for parallel execution (ParallelActivity)
- Error handling via Go's error return pattern
- ActivityType as a function that creates activity instances

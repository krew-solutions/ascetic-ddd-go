# Saga Pattern

A Go implementation of the Saga pattern using the **Routing Slip** approach.

For detailed documentation, see the Python version:
https://github.com/krew-solutions/ascetic-ddd-python/blob/main/ascetic_ddd/saga/README.md

## Key Differences from Python Version

- No async/await - uses regular Go functions with `context.Context`
- Goroutines and channels for parallel execution (ParallelActivity)
- Error handling via Go's error return pattern
- ActivityType as a function that creates activity instances

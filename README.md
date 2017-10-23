# web-utils 

[![Travis-CI](https://api.travis-ci.org/ShevaXu/web-utils.svg)](https://api.travis-ci.org/ShevaXu/web-utils)
[![GoDoc](https://godoc.org/github.com/ShevaXu/web-utils?status.svg)](https://godoc.org/github.com/ShevaXu/web-utils)
[![Go Report Card](https://goreportcard.com/badge/github.com/ShevaXu/web-utils)](https://goreportcard.com/report/github.com/ShevaXu/web-utils)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://choosealicense.com/licenses/mit/)

web-utils provides useful web-dev features following some best practices for production, including:

### HTTP Utils

A **SafeClient** with

* `timeout` setting for underlying http.Client;
* request retries (can be timeout only);
* [exponential backoff](https://en.wikipedia.org/wiki/Exponential_backoff).

Just `utils.StdClient()` to get a preset-client or `cl := utils.SafeClient{...}` for a custom one.

### Semaphore

For [Bounding resource use](https://github.com/golang/go/wiki/BoundingResourceUse).

```go
s := NewSemaphore(10)

go func() {
    ctx := context.Background() // for cancellation
    if s.Obtain(ctx) {
        defer s.Release()
        // do whatever 
    }
}()
```

For *weighted* semaphore, see []this implementation](https://github.com/golang/sync/blob/master/semaphore/semaphore.go).

### Asserting

Tiny functions for testing.

```go
a := assert.NewAssert()

a.True(...)
a.Equal(...)
a.NotEqual(...)
a.Nil(...)
a.NotNil(...)
a.NoError(...)
```

## Install

```
go get github.com/ShevaXu/web-utils
```

## TODO

* `context` support for client;
* rate-limit ([leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket));
* [circuit-breaker](https://martinfowler.com/bliki/CircuitBreaker.html).
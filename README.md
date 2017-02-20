# web-utils 

[![Travis-CI](https://api.travis-ci.org/ShevaXu/web-utils.svg)](https://api.travis-ci.org/ShevaXu/web-utils)
[![GoDoc](https://godoc.org/github.com/ShevaXu/web-utils?status.svg)](https://godoc.org/github.com/ShevaXu/web-utils)
[![Go Report Card](https://goreportcard.com/badge/github.com/ShevaXu/web-utils)](https://goreportcard.com/report/github.com/ShevaXu/web-utils)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://choosealicense.com/licenses/mit/)

web-utils provides useful network features following some best practices for production, including:

A **SafeClient** with

* `timeout` setting for underlying http.Client;
* request retries (can be timeout only);
* [exponential backoff](https://en.wikipedia.org/wiki/Exponential_backoff).

and more ...

## Usage

```
go get github.com/ShevaXu/web-utils
```

Just `utils.StdClient()` to get a preset-client or `cl := utils.SafeClient{...}` for a custom one.

## TODO

* `context` support for client;
* rate-limit ([leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket));
* [circuit-breaker](https://martinfowler.com/bliki/CircuitBreaker.html).
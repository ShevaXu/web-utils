# web-utils [![Travis-CI](https://api.travis-ci.org/ShevaXu/web-utils.svg)](https://api.travis-ci.org/ShevaXu/web-utils) (https://godoc.org/github.com/ShevaXu/web-utils?status.svg)](http://godoc.org/github.com/ShevaXu/web-utils)

web-utils provides useful features to Go's http.Client following some best practices for production, including:

* `timeout` setting for underlying http.Client;
* request retries (can be timeout only);
* [exponential backoff](https://en.wikipedia.org/wiki/Exponential_backoff).

## Usage

`utils.StdClient()` to get a ready-to-use client or `cl := utils.SafeClient{...}` for a custom one.

## License

MIT

package utils

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
)

// NewJsonPost returns a Request with json encoded and header set.
func NewJsonPost(url string, v interface{}) (*http.Request, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	return req, nil
}

// ShouldRetry determines if the client should repeat the request
// without modifications at any later time;
// returns true for http 408 and 5xx status.
// TODO: should exclude 501, 505 and 511?
func ShouldRetry(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout || (statusCode >= 500 && statusCode <= 599)
}

// IsTimeoutErr checks if an error is timeout by cast it to net.Error.
func IsTimeoutErr(e error) bool {
	if err, ok := e.(net.Error); ok {
		return err.Timeout()
	}
	return false
}

// Backoff implements the exponential backoff algorithm with jitter for client sending remote calls.
// It use an alternative method described in https://www.awsarchitectureblog.com/2015/03/backoff.html:
type Backoff struct {
	BaseSleep, MaxSleep int
}

// Next returns the next sleep time computed by the previous one;
// the Decorrelated Jitter is:
// sleep = min(cap, random_between(base, sleep * 3)).
func (b *Backoff) Next(previous int) int {
	if previous <= b.BaseSleep {
		previous = b.BaseSleep
	}
	// Intn will panic if arg <= 0
	sleep := rand.Intn(previous*3-b.BaseSleep) + b.BaseSleep
	if sleep > b.MaxSleep {
		return b.MaxSleep
	}
	return sleep
}

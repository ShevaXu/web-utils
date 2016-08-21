package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"time"
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

// SafeClient provides additional io Reader handle and request retry
// by wrapping a http.Client (safe for concurrent use by multiple goroutines).
type SafeClient struct {
	TimeoutOnly bool
	http.Client // embedded
	Backoff
}

// RequestWithClose sends the request and returns statusCode and raw body.
// It reads and closes Response.Body, return any error occurs.
func (c *SafeClient) RequestWithClose(req *http.Request) (status int, body []byte, err error) {
	var resp *http.Response

	resp, err = c.Do(req)
	if err != nil {
		return
	}

	// Close() iff resp did return
	defer resp.Body.Close()

	status = resp.StatusCode

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}

// RequestWithRetry wraps RequestWithClose and exponential-backoff retries in following conditions:
// 1. timeout error occurs (mostly client-side);
// 2. server-side should-retry statusCode returned.
// It returns the last response if tries run out.
// NOTICE: retry works for request with no body only.
func (c *SafeClient) RequestWithRetry(req *http.Request, maxTries int) (tries, status int, body []byte, err error) {
	// 0 will trigger setting wait to base
	wait := 0

	for ; tries < maxTries; tries++ {
		// update next sleep time
		wait = c.Next(wait)
		// do request
		status, body, err = c.RequestWithClose(req)
		if err != nil {
			if !c.TimeoutOnly || IsTimeoutErr(err) {
				time.Sleep(time.Duration(wait) * time.Millisecond)
				continue
			}
			return
		}
		// no error, check status
		if ShouldRetry(status) {
			time.Sleep(time.Duration(wait) * time.Millisecond)
			continue
		}
		// succeed or should not repeat
		return
	}

	// return the last request's response, succeed or not
	tries--
	return
}

// PostJsonWithRetry is the special case of RequestWithRetry that initialize a Request each time to ensure Body get consumed.
// Using other methods, forms or custom headers can do similarly.
func (c *SafeClient) PostJsonWithRetry(url string, v interface{}, maxTries int) (tries, status int, body []byte, err error) {
	var req *http.Request
	wait := 0

	for ; tries < maxTries; tries++ {
		// make post request each time
		req, err = NewJsonPost(url, v)
		if err != nil {
			return
		}

		wait = c.Next(wait)
		status, body, err = c.RequestWithClose(req)
		if err != nil {
			if !c.TimeoutOnly || IsTimeoutErr(err) {
				time.Sleep(time.Duration(wait) * time.Millisecond)
				continue
			}
			return
		}
		if ShouldRetry(status) {
			time.Sleep(time.Duration(wait) * time.Millisecond)
			continue
		}
		return
	}

	tries--
	return
}

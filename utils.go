// Package utils provides useful features to Go's http.Client following
// some best practices for production, such as timeout, retries and backoff.
package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"
)

// RequestHook can modify the Request anyway it wants.
type RequestHook func(req *http.Request)

// NewJSONPost returns a Request with json encoded and header set;
// additional headers or cookies can be set through the RequestHook.
func NewJSONPost(url string, v interface{}, f RequestHook) (*http.Request, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	if f != nil {
		f(req)
	}

	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	return req, nil
}

// NewFormPost returns a Request with default "Content-type: text/plain".
func NewFormPost(url string, v url.Values, f RequestHook) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(v.Encode())))
	if err != nil {
		return nil, err
	}

	if f != nil {
		f(req)
	}

	return req, nil
}

// ShouldRetry determines if the client should repeat the request
// without modifications at any later time;
// returns true for http 408 and 5xx status.
func ShouldRetry(statusCode int) bool {
	// TODO: should exclude 501, 505 and 511?
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

// HTTPClient provides additional features upon http.Client,
// e.g., io Reader handle and request retry;
// it also normalize the HTTP response.
type HTTPClient interface {
	RequestWithRetry(req *http.Request, maxTries int) (tries, status int, body []byte, err error)
	DoRequest(method, url string, content []byte, maxTries int, f RequestHook) (tries, status int, body []byte, err error)
}

// SafeClient implements HTTPClient; it wraps a http.Client
// underneath (safe for concurrent use by multiple goroutines).
type SafeClient struct {
	TimeoutOnly bool
	http.Client // embedded
	Backoff
}

// RequestWithClose sends the request and returns statusCode and raw body.
// It reads and closes Response.Body, return any error occurs.
func (c *SafeClient) RequestWithClose(req *http.Request) (status int, body []byte, err error) {
	var resp *http.Response

	// Close() iff resp did return
	if resp != nil {
		defer resp.Body.Close()
	}

	resp, err = c.Do(req)
	if err != nil {
		return
	}

	status = resp.StatusCode

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}

// RequestWithRetry wraps RequestWithClose and exponential-backoff
// retries in following conditions:
// 1. timeout error occurs (mostly client-side);
// 2. server-side should-retry statusCode returned.
// It returns the last response if tries run out.
// NOTICE: retry works for request with no body only before go1.9.
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

// DoRequest is the generalized version of RequestWithRetry that
// initialize a Request each time to ensure Body get consumed.
// Additional headers or cookies can be set through the RequestHook.
func (c *SafeClient) DoRequest(method, url string, content []byte, maxTries int, f RequestHook) (tries, status int, body []byte, err error) {
	var req *http.Request
	wait := 0

	for ; tries < maxTries; tries++ {
		// make a new request each time
		if len(content) > 0 {
			req, err = http.NewRequest(method, url, bytes.NewBuffer(content))
		} else {
			req, err = http.NewRequest(method, url, nil)
		}
		if err != nil {
			return
		}

		if f != nil {
			f(req)
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

// PostJSONWithRetry is a convenient method for JSON POST requests.
func (c *SafeClient) PostJSONWithRetry(url string, v interface{}, maxTries int, f RequestHook) (tries, status int, body []byte, err error) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	return c.DoRequest("POST", url, data, maxTries, func(req *http.Request) {
		req.Header.Add("Content-Type", "application/json; charset=utf-8")
		if f != nil {
			f(req)
		}
	})
}

// PostFormWithRetry is a convenient method for form POST requests.
func (c *SafeClient) PostFormWithRetry(url string, v url.Values, maxTries int, f RequestHook) (tries, status int, body []byte, err error) {
	return c.DoRequest("POST", url, []byte(v.Encode()), maxTries, f)
}

// StdClient gives a ready-to-use SafeClient instance.
func StdClient() *SafeClient {
	return &SafeClient{
		true,
		http.Client{Timeout: 5 * time.Second},
		Backoff{100, 5000},
	}
}

package utils_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/ShevaXu/web-utils"
)

type testContent struct {
	Data string `json:"data"`
}

func TestNewJsonPost(t *testing.T) {
	req, err := NewJsonPost("/", testContent{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "application/json; charset=utf-8", req.Header.Get("Content-Type"), "Proper header")
	decoder := json.NewDecoder(req.Body)
	var c testContent
	err = decoder.Decode(&c)
	if err != nil {
		t.Fatalf("Error decoding the body: %s", err)
	}
	assert.Equal(t, c.Data, "hello", `Should respond with "hello"`)
}

type RetryTest struct {
	code   int
	should bool
}

func TestShouldRetry(t *testing.T) {
	tests := []RetryTest{
		{200, false},
		{400, false},
		{408, true},
		{500, true},
		{501, true},
		{502, true},
		{505, true},
		{511, true},
	}
	for _, test := range tests {
		assert.Equal(t, test.should, ShouldRetry(test.code), "Retry test fails")
	}
}

func TestIsTimeoutErr(t *testing.T) {
	assert.Equal(t, false, IsTimeoutErr(errors.New("not timeout")), "Normal error is not")
	assert.Equal(t, false, IsTimeoutErr(&net.AddrError{"error", "addr"}), "AddrError is not")
	assert.Equal(t, true, IsTimeoutErr(&net.DNSError{"error", "name", "server", true, false}), "Should be")
}

var OkHandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
})

var (
	minTimeout        = 10
	maxTimeout        = 50
	testBackoff       = Backoff{minTimeout, maxTimeout}
	testTimeoutClient = SafeClient{
		true,
		http.Client{Timeout: time.Duration(minTimeout) * time.Millisecond},
		testBackoff,
	}
)

var TimeoutHandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	time.Sleep(20 * time.Millisecond)
})

func TestBackoff_Next(t *testing.T) {
	sleep0 := testBackoff.Next(0)
	assert.True(t, sleep0 >= minTimeout && sleep0 <= minTimeout*3, "First sleep is bounded")
	sleep1 := testBackoff.Next(sleep0)
	sleep2 := testBackoff.Next(sleep1)
	sleep3 := testBackoff.Next(sleep2)
	assert.True(t, sleep1 >= minTimeout && sleep2 >= minTimeout, "Each sleep > base")
	assert.True(t, sleep2 <= maxTimeout && sleep3 <= maxTimeout, "Each sleep < max")
}

type closeTest struct {
	h             http.Handler
	expectedCode  int
	expectedBody  []byte
	expectTimeout bool
}

// TODO: how to test if Close() works
func TestSafeClient_RequestWithClose(t *testing.T) {
	tests := []closeTest{
		{
			OkHandlerFunc,
			http.StatusOK,
			[]byte("OK"),
			false,
		},
		{
			TimeoutHandlerFunc,
			http.StatusOK,
			nil,
			true,
		},
	}

	for _, test := range tests {
		server := httptest.NewServer(test.h)

		req, err := http.NewRequest("GET", server.URL, nil)
		if err != nil {
			t.Errorf("Error new request: %s", err)
			continue
		}
		status, body, err := testTimeoutClient.RequestWithClose(req)
		if test.expectTimeout {
			if err != nil {
				assert.Equal(t, true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			assert.Equal(t, test.expectedCode, status, "Return code")
			assert.Equal(t, test.expectedBody, body, "Return body")
		}

		server.Close()
	}
}

const internalErr = "Internal error"

var Status5xxHandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(internalErr))
})

type retryTest struct {
	closeTest
	tries         int
	expectedTries int
}

// TODO: test retry wait/interval setting
func TestSafeClient_RequestWithRetry(t *testing.T) {
	tests := []retryTest{
		{
			closeTest{
				OkHandlerFunc,
				http.StatusOK,
				[]byte("OK"),
				false,
			},
			3,
			0,
		},
		{
			closeTest{
				TimeoutHandlerFunc,
				http.StatusOK,
				nil,
				true,
			},
			3,
			2,
		},
		{
			closeTest{
				Status5xxHandlerFunc,
				http.StatusInternalServerError,
				[]byte(internalErr),
				false,
			},
			5,
			4,
		},
	}

	for _, test := range tests {
		server := httptest.NewServer(test.h)

		req, err := http.NewRequest("GET", server.URL, nil)
		if err != nil {
			t.Errorf("Error new request: %s", err)
			continue
		}
		n, status, body, err := testTimeoutClient.RequestWithRetry(req, test.tries)
		if test.expectTimeout {
			if err != nil {
				assert.Equal(t, true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			assert.Equal(t, test.expectedCode, status, "Returns code")
			assert.Equal(t, test.expectedBody, body, "Returns body")
		}
		assert.Equal(t, test.expectedTries, n, "Report retried times")

		server.Close()
	}
}

func TestSafeClient_RequestWithRetry_Bug(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// TimeoutHandlerFunc causes client-side timeout, thus not drill out the request body
	server := httptest.NewServer(Status5xxHandlerFunc) // this handler will read the request body
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer([]byte("foo")))
	if err != nil {
		t.Fatalf("Error new request: %s", err)
	}

	_, _, _, err = testTimeoutClient.RequestWithRetry(req, 3)

	assert.True(t, err != nil, "Should have error")
}

func TestSafeClient_PostJsonWithRetry(t *testing.T) {
	tests := []retryTest{
		{
			closeTest{
				OkHandlerFunc,
				http.StatusOK,
				[]byte("OK"),
				false,
			},
			3,
			0,
		},
		{
			closeTest{
				TimeoutHandlerFunc,
				http.StatusOK,
				nil,
				true,
			},
			3,
			2,
		},
		{
			closeTest{
				Status5xxHandlerFunc,
				http.StatusInternalServerError,
				[]byte(internalErr),
				false,
			},
			5,
			4,
		},
	}

	for _, test := range tests {
		server := httptest.NewServer(test.h)
		n, status, body, err := testTimeoutClient.PostJsonWithRetry(server.URL, testContent{"foo"}, test.tries)
		if test.expectTimeout {
			if err != nil {
				assert.Equal(t, true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			assert.Equal(t, test.expectedCode, status, "Returns code")
			assert.Equal(t, test.expectedBody, body, "Returns body")
		}
		assert.Equal(t, test.expectedTries, n, "Report retried times")

		server.Close()
	}
}

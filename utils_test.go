package utils_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	. "github.com/ShevaXu/web-utils"
	"github.com/ShevaXu/web-utils/assert"
)

type testContent struct {
	Data string `json:"data"`
}

func addTestHeader(req *http.Request) {
	req.Header.Add("x-test", "test")
}

func TestNewJsonPost(t *testing.T) {
	a := assert.NewAssert(t)

	req, err := NewJsonPost("/", testContent{"hello"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.Equal("application/json; charset=utf-8", req.Header.Get("Content-Type"), "Proper header")

	decoder := json.NewDecoder(req.Body)
	var c testContent
	err = decoder.Decode(&c)
	if err != nil {
		t.Fatalf("Error decoding the body: %s", err)
	}
	a.Equal(c.Data, "hello", `Should respond with "hello"`)

	req, err = NewJsonPost("/", testContent{"hello"}, addTestHeader)
	if err != nil {
		t.Fatal(err)
	}
	a.Equal("test", req.Header.Get("x-test"), "Hooked header")
}

func TestNewJsonForm(t *testing.T) {
	a := assert.NewAssert(t)

	v := url.Values{}
	req, err := NewJsonForm("/", v, nil)
	if err != nil {
		t.Fatal(err)
	}
	a.Equal("", req.Header.Get("Content-Type"), "Proper header")

	body, _ := ioutil.ReadAll(req.Body)
	a.Equal(v.Encode(), string(body), "Body encoded")

	req, err = NewJsonPost("/", v, addTestHeader)
	if err != nil {
		t.Fatal(err)
	}
	a.Equal("test", req.Header.Get("x-test"), "Hooked header")
}

type RetryTest struct {
	code   int
	should bool
}

func TestShouldRetry(t *testing.T) {
	a := assert.NewAssert(t)

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
		a.Equal(test.should, ShouldRetry(test.code), "Retry test fails")
	}
}

func TestIsTimeoutErr(t *testing.T) {
	a := assert.NewAssert(t)

	a.Equal(false, IsTimeoutErr(errors.New("not timeout")), "Normal error is not")
	a.Equal(false, IsTimeoutErr(&net.AddrError{}), "AddrError is not")
	a.Equal(true, IsTimeoutErr(&net.DNSError{IsTimeout: true}), "Should be")
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
	a := assert.NewAssert(t)

	sleep0 := testBackoff.Next(0)
	a.True(sleep0 >= minTimeout && sleep0 <= minTimeout*3, "First sleep is bounded")
	sleep1 := testBackoff.Next(sleep0)
	sleep2 := testBackoff.Next(sleep1)
	sleep3 := testBackoff.Next(sleep2)
	a.True(sleep1 >= minTimeout && sleep2 >= minTimeout, "Each sleep > base")
	a.True(sleep2 <= maxTimeout && sleep3 <= maxTimeout, "Each sleep < max")
}

type closeTest struct {
	h             http.Handler
	expectedCode  int
	expectedBody  []byte
	expectTimeout bool
}

// TODO: how to test if Close() works
func TestSafeClient_RequestWithClose(t *testing.T) {
	a := assert.NewAssert(t)

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
				a.Equal(true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			a.Equal(test.expectedCode, status, "Return code")
			a.Equal(test.expectedBody, body, "Return body")
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
	tries          int
	expectedReries int
}

// TODO: test retry wait/interval setting
func TestSafeClient_RequestWithRetry(t *testing.T) {
	a := assert.NewAssert(t)

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
				a.Equal(true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			a.Equal(test.expectedCode, status, "Returns code")
			a.Equal(test.expectedBody, body, "Returns body")
		}
		a.Equal(test.expectedReries, n, "Report retried times")

		server.Close()
	}
}

func TestSafeClient_RequestWithRetry_Bug(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	a := assert.NewAssert(t)

	// TimeoutHandlerFunc causes client-side timeout, thus not drill out the request body
	server := httptest.NewServer(Status5xxHandlerFunc) // this handler will read the request body
	defer server.Close()

	req, err := http.NewRequest("POST", server.URL, bytes.NewBuffer([]byte("foo")))
	if err != nil {
		t.Fatalf("Error new request: %s", err)
	}

	_, _, _, err = testTimeoutClient.RequestWithRetry(req, 3)

	//fmt.Println(err) // Post http://127.0.0.1:49833: http: ContentLength=3 with Body length 0
	a.True(err != nil, "Should have error")
}

func CheckHeaderHandler(header, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get(header), value) { // approximate
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Wrong header"))
		}
	})
}

func TestSafeClient_PostJsonWithRetry(t *testing.T) {
	a := assert.NewAssert(t)

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
		{
			closeTest{
				CheckHeaderHandler("x-test", "test"),
				http.StatusForbidden,
				[]byte("Wrong header"),
				false,
			},
			5,
			0, // no retry if forbidden
		},
		{
			closeTest{
				CheckHeaderHandler("Content-Type", "application/json"),
				http.StatusOK,
				[]byte("OK"),
				false,
			},
			5,
			0, // no retry if forbidden
		},
	}

	for _, test := range tests {
		server := httptest.NewServer(test.h)
		n, status, body, err := testTimeoutClient.PostJsonWithRetry(server.URL, testContent{"foo"}, test.tries, nil)
		if test.expectTimeout {
			if err != nil {
				a.Equal(true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			a.Equal(test.expectedCode, status, "Returns code")
			a.Equal(test.expectedBody, body, "Returns body")
		}
		a.Equal(test.expectedReries, n, "Report retried times")

		server.Close()
	}

	// hooked case
	server := httptest.NewServer(CheckHeaderHandler("x-test", "test"))
	n, status, _, err := testTimeoutClient.PostJsonWithRetry(server.URL, testContent{"foo"}, 3, addTestHeader)
	if err != nil {
		t.Errorf("Error request: %s", err)
	}
	a.Equal(http.StatusOK, status, "Returns code")
	a.Equal(0, n, "Report retried times")

	server.Close()
}

func TestSafeClient_PostFormWithRetry(t *testing.T) {
	a := assert.NewAssert(t)

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
		{
			closeTest{
				CheckHeaderHandler("x-test", "test"),
				http.StatusForbidden,
				[]byte("Wrong header"),
				false,
			},
			5,
			0, // no retry if forbidden
		},
	}

	for _, test := range tests {
		server := httptest.NewServer(test.h)
		n, status, body, err := testTimeoutClient.PostFormWithRetry(server.URL, url.Values{}, test.tries, nil)
		if test.expectTimeout {
			if err != nil {
				a.Equal(true, IsTimeoutErr(err), "Should be")
			} else {
				t.Error("Should return timeout error")
			}
		} else {
			if err != nil {
				t.Errorf("Error request: %s", err)
			}
			a.Equal(test.expectedCode, status, "Returns code")
			a.Equal(test.expectedBody, body, "Returns body")
		}
		a.Equal(test.expectedReries, n, "Report retried times")

		server.Close()
	}

	// hooked case
	server := httptest.NewServer(CheckHeaderHandler("x-test", "test"))
	n, status, _, err := testTimeoutClient.PostFormWithRetry(server.URL, url.Values{}, 3, addTestHeader)
	if err != nil {
		t.Errorf("Error request: %s", err)
	}
	a.Equal(http.StatusOK, status, "Returns code")
	a.Equal(0, n, "Report retried times")

	server.Close()
}

func TestStdClient(t *testing.T) {
	a := assert.NewAssert(t)

	cl := StdClient()
	a.NotNil(cl, "StdClient not nil")
	addr := fmt.Sprintf("%p", cl)

	cl2 := StdClient()
	addr2 := fmt.Sprintf("%p", cl2)

	a.Equal(cl, cl2, "Every call returns a value-equal client")
	//fmt.Println(addr, addr2)
	a.NotEqual(addr, addr2, "Every call returns a defferent client")
}

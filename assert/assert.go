// Package assert provides tiny asserting functions for testing.
// Inspired by github.com/stretchr/testify/assert
// and https://github.com/benbjohnson/testing
package assert

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// Assert wraps a testing.TB for convenient asserting calls.
type Assert struct {
	t testing.TB
}

// ObjectsAreEqual checks two interfaces with reflect.DeepEqual.
func ObjectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	return reflect.DeepEqual(expected, actual)
}

// IsNil checks an interface{} with the reflect package.
func IsNil(object interface{}) bool {
	if object == nil {
		return true
	}

	value := reflect.ValueOf(object)
	kind := value.Kind()
	if kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil() {
		return true
	}

	return false
}

// errorSingle fails and prints the single object
// along with the message.
func errorSingle(t testing.TB, msg string, obj interface{}) {
	//t.Errorf("%s: %v", msg, obj)
	_, file, line, _ := runtime.Caller(2)
	fmt.Printf("\033[31m\t%s:%d: %s\n\n\t\t%#v\033[39m\n\n", filepath.Base(file), line, msg, obj)
	t.Fail()
}

// errorCompare fails and prints both the compared objects
// along with the message.
func errorCompare(t testing.TB, msg string, expected, actual interface{}) {
	_, file, line, _ := runtime.Caller(2)
	fmt.Printf("\033[31m\t%s:%d: %s\n\n\t\tgot: %#v\n\033[32m\t\texp: %#v\033[39m\n\n", filepath.Base(file), line, msg, actual, expected)
	t.Fail()
}

func (a *Assert) True(cond bool, msg string) {
	if !cond {
		errorSingle(a.t, msg, cond)
	}
}

func (a *Assert) Equal(expected, actual interface{}, msg string) {
	if !ObjectsAreEqual(expected, actual) {
		errorCompare(a.t, msg, expected, actual)
	}
}

func (a *Assert) NotEqual(expected, actual interface{}, msg string) {
	if ObjectsAreEqual(expected, actual) {
		errorCompare(a.t, msg, expected, actual)
	}
}

func (a *Assert) NoError(err error, msg string) {
	if err != nil {
		errorSingle(a.t, msg, err)
	}
}

func (a *Assert) Nil(obj interface{}, msg string) {
	if !IsNil(obj) {
		errorSingle(a.t, msg, obj)
	}
}

func (a *Assert) NotNil(obj interface{}, msg string) {
	if IsNil(obj) {
		errorSingle(a.t, msg, obj)
	}
}

// NewAssert provides an Assert instance.
func NewAssert(t testing.TB) *Assert {
	return &Assert{t}
}

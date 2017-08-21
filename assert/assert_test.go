package assert_test

import (
	"testing"

	. "github.com/ShevaXu/web-utils/assert"
)

func TestObjectsAreEqual(t *testing.T) {
	v := struct{}{}
	if !ObjectsAreEqual(v, v) {
		t.Error("Same object should be equal")
	}

	var v2 interface{} = nil
	if !ObjectsAreEqual(v2, nil) {
		t.Error("Nils should be equal")
	}
}

func TestIsNil(t *testing.T) {
	v := struct{}{}
	if IsNil(v) {
		t.Error("Zero struct{} should not be nil")
	}

	var vi interface{} = nil
	if !IsNil(vi) {
		t.Error("Nil interface should be nil")
	}

	var vc chan struct{}
	if !IsNil(vc) {
		t.Error("Empty chan should be nil")
	}

	vc0 := make(chan struct{}, 0)
	if IsNil(vc0) {
		t.Error("Zero chan should not be nil")
	}

	var vs []struct{}
	if !IsNil(vs) {
		t.Error("Empty slice should be nil")
	}

	vs0 := make([]struct{}, 0)
	if IsNil(vs0) {
		t.Error("Zero slice should not be nil")
	}
}

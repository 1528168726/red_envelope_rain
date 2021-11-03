package main

import (
	"testing"
)

func TestGetUserCurCount(t *testing.T) {
	a := getUserCurCount(1)
	if a == 0 {
		t.Error(a)
	} else {
		t.Log(a)
	}
}

func TestGlobalInfo(t *testing.T) {
	t.Log(globalInfo)
}

func TestInsertEnvelopes(t *testing.T) {
	id, err := insertEnvelopes(1, 2, 2)
	if err != nil {
		t.Error(err)
	} else {
		t.Log(id)
	}
}

func TestGetEnvelopes(t *testing.T) {
	env, err := getEnvelopes(1)
	if err != nil {
		t.Error(err)
	} else {
		t.Log(env)
	}
}

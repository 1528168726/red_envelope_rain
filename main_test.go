package main

import (
	"testing"
)

func TestConnectToMySql(t *testing.T) {
	err := connectToMySql()
	if err != nil {
		t.Error(err)
	} else {
		t.Log("connect to mysql server succeed")
	}
}

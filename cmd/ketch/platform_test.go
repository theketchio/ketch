package main

import (
	"io/ioutil"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClientCtor struct {
	called bool
}

func (mc *mockClientCtor) Client() client.Client {
	mc.called = true
	return nil
}

func TestJustInTimeClient(t *testing.T) {
	var cli mockClientCtor
	_ = newPlatformAddCmd(jit(&cli), ioutil.Discard)
	if cli.called {
		t.Fatal("client should not get called")
	}
}

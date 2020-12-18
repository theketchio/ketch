package main

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

/*
 Test fixtures for resource interfaces, these are used to mock out interactions with the cluster.  Provide your
 own implementation of f for tests. See platform_add_test.go for an example of usage.
*/

type resourceCreatorMock struct {
	f      func(o runtime.Object) error
	called bool
}

func (m *resourceCreatorMock) Create(_ context.Context, object runtime.Object, _ ...client.CreateOption) error {
	m.called = true
	return m.f(object)
}

type resourceGetterMock struct {
	f      func(name types.NamespacedName, obj runtime.Object) error
	called bool
}

func (m *resourceGetterMock) Get(_ context.Context, name types.NamespacedName, object runtime.Object) error {
	m.called = true
	return m.f(name, object)
}

type resourceDeleterMock struct {
	f      func(obj runtime.Object) error
	called bool
}

func (m *resourceDeleterMock) Delete(_ context.Context, object runtime.Object, _ ...client.DeleteOption) error {
	m.called = true
	return m.f(object)
}

type resourceListerMock struct {
	f      func(o runtime.Object) error
	called bool
}

func (m *resourceListerMock) List(_ context.Context, object runtime.Object, _ ...client.ListOption) error {
	m.called = true
	return m.f(object)
}

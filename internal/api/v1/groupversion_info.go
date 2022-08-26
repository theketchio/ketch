/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1 contains API Schema definitions for the resources v1 API group
// +kubebuilder:object:generate=true
// +groupName=theketch.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	TheKetchGroup = "theketch.io"
)

// Group is the exported variable indicating group, defaults to theketch.io
var Group = TheKetchGroup

type SchemeOptions struct {
	group             string
	registerFramework bool
}

type schemeOption func(opts *SchemeOptions)

func WithGroup(group string) schemeOption {
	return func(opts *SchemeOptions) { opts.group = group }
}

// WithoutFramework means ketch won't register its Framework and FrameworkList when executing AddToScheme().
func WithoutFramework() schemeOption {
	return func(opts *SchemeOptions) {
		opts.registerFramework = false
	}
}
func defaultSchemeOptions() SchemeOptions {
	return SchemeOptions{
		group:             TheKetchGroup,
		registerFramework: true,
	}
}

// AddToScheme can be easily consumed by ketch-cli and by default it uses `theketch.io` // and it can be consumed by shipa:
// AddToScheme(WithGroup("shipa.io"))
func AddToScheme(opts ...schemeOption) func(s *runtime.Scheme) error {
	options := defaultSchemeOptions()
	for _, o := range opts {
		o(&options)
	}
	groupVersion := schema.GroupVersion{Group: options.group, Version: "v1"}
	builder := &scheme.Builder{GroupVersion: groupVersion}
	builder.Register(&App{}, &AppList{})
	builder.Register(&Job{}, &JobList{})
	Group = options.group
	return builder.AddToScheme
}

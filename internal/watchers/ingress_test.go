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

package watchers

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type testingContext struct {
	env *envtest.Environment
	context.Context
	k8sClient client.Client
	cancel    context.CancelFunc
}

func setup(objects []client.Object) (*testingContext, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	ctx := &testingContext{
		Context: cancelCtx,
		env: &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
		},
		cancel: cancel,
	}
	cfg, err := ctx.env.Start()
	if err != nil {
		return nil, err
	}
	if err = ketchv1.AddToScheme()(scheme.Scheme); err != nil {
		return nil, err
	}
	ctx.k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	go func() {
		_ = k8sManager.Start(ctx)
	}()

	for _, obj := range objects {
		if err = ctx.k8sClient.Create(context.TODO(), obj); err != nil {
			return nil, err
		}
	}
	return ctx, nil
}

func teardown(ctx *testingContext) {
	if ctx == nil {
		return
	}
	ctx.cancel()
	err := ctx.env.Stop()
	if err != nil {
		panic(err)
	}
}

func TestInform(t *testing.T) {
	app := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-app",
			Namespace: "namespace",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{},
			Namespace:   "namespace",
			Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.NginxIngressControllerType}},
		},
	}

	tests := []struct {
		description string
		obj         []client.Object
		expectedErr string
	}{
		{
			description: "success",
			obj: []client.Object{app, &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: ketchv1.IngressConfigmapName, Namespace: ketchv1.IngressConfigmapNamespace},
				Data: map[string]string{
					"className":       "nginx",
					"serviceEndpoint": "127.0.0.1",
					"clusterIssuer":   "letsencrypt",
					"ingressType":     "nginx",
				},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			testctx, err := setup(tc.obj)
			require.Nil(t, err)
			require.NotNil(t, testctx)
			defer teardown(testctx)

			i := NewIngressWatcher(fake.NewSimpleClientset(), testctx.k8sClient, ctrl.Log)
			i.retryDelay = time.Millisecond * 100
			i.retries = 1

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			err = i.Inform(ctx)
			if tc.expectedErr == "" {
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func Test_handleAddUpdateIngressConfigmap(t *testing.T) {
	defaultObjects := []client.Object{
		&ketchv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-app",
			},
			Spec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{},
				Namespace:   "namespace",
				Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.NginxIngressControllerType}},
			},
		},
	}

	testctx, err := setup(defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, testctx)
	defer teardown(testctx)
	var buf bytes.Buffer
	opts := zap.Options{
		DestWriter: &buf,
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	i := IngressWatcher{
		client:     testctx.k8sClient,
		logger:     ctrl.Log,
		retryDelay: time.Millisecond * 100,
		retries:    1,
	}

	tests := []struct {
		description string
		obj         interface{}
		expected    string
	}{
		{
			description: "success",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: ketchv1.IngressConfigmapName},
				Data: map[string]string{
					"className":       "nginx",
					"serviceEndpoint": "127.0.0.1",
					"clusterIssuer":   "letsencrypt",
					"ingressType":     "nginx",
				},
			},
			expected: `"msg":"updating app ingress controller","app":"default-app","ingress controller":{"className":"nginx","serviceEndpoint":"127.0.0.1","type":"nginx","clusterIssuer":"letsencrypt"}`,
		},
		{
			description: "ok - no object",
			obj:         nil,
		},
		{
			description: "logs error",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: ketchv1.IngressConfigmapName},
				Data:       map[string]string{},
			},
			expected: "error updating ingress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			i.handleAddUpdateIngressConfigmap(context.Background(), tc.obj)
			require.Contains(t, buf.String(), tc.expected)
			buf.Truncate(0)
		})
	}
}

func TestUpdateAppIngress(t *testing.T) {
	defaultObjects := []client.Object{
		&ketchv1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default-app",
			},
			Spec: ketchv1.AppSpec{
				Deployments: []ketchv1.AppDeploymentSpec{},
				Namespace:   "namespace",
				Ingress:     ketchv1.IngressSpec{Controller: ketchv1.IngressControllerSpec{IngressType: ketchv1.NginxIngressControllerType}},
			},
		},
	}

	testctx, err := setup(defaultObjects)
	require.Nil(t, err)
	require.NotNil(t, testctx)
	defer teardown(testctx)

	i := IngressWatcher{
		client: testctx.k8sClient,
		logger: ctrl.Log,
	}

	tests := []struct {
		description           string
		ingressControllerSpec ketchv1.IngressControllerSpec
		expectedErr           string
	}{
		{
			description:           "fail at missing ingress controller fields",
			ingressControllerSpec: ketchv1.IngressControllerSpec{},
			expectedErr:           "App.theketch.io \"default-app\" is invalid: spec.ingress.controller.type: Unsupported value: \"\": supported values: \"traefik\", \"istio\", \"nginx\"",
		},
		{
			description: "success",
			ingressControllerSpec: ketchv1.IngressControllerSpec{
				ClassName:       "nginx",
				ServiceEndpoint: "127.0.0.1",
				ClusterIssuer:   "letsencrypt",
				IngressType:     "nginx",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			err = i.updateAppsIngress(context.Background(), tc.ingressControllerSpec)
			if tc.expectedErr == "" {
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

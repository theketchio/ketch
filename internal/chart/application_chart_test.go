package chart

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"io/ioutil"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
	"github.com/shipa-corp/ketch/internal/templates"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNew(t *testing.T) {

	const chartDirectory = "./testdata/charts/"

	poolWithClusterIssuer := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "ingress-class",
				ServiceEndpoint: "10.10.10.10",
				ClusterIssuer:   "letsencrypt-production",
			},
		},
	}
	poolWithoutClusterIssuer := &ketchv1.Pool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pool",
		},
		Spec: ketchv1.PoolSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "gke",
				ServiceEndpoint: "20.20.20.20",
			},
		},
	}
	exportedPorts := map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort{
		3: {{Port: 9090, Protocol: "TCP"}},
	}
	dashboard := &ketchv1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dashboard",
		},
		Spec: ketchv1.AppSpec{
			Deployments: []ketchv1.AppDeploymentSpec{
				{
					Image:   "shipasoftware/go-app:v1",
					Version: 3,
					Processes: []ketchv1.ProcessSpec{
						{Name: "web", Units: intRef(3), Cmd: []string{"python"}},
						{Name: "worker", Units: intRef(1), Cmd: []string{"celery"}},
					},
					RoutingSettings: ketchv1.RoutingSettings{
						Weight: 100,
					},
				},
			},
			Env: []ketchv1.Env{
				{Name: "VAR", Value: "VALUE"},
			},
			Pool: "pool",
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
				Cnames:               []string{"theketch.io", "app.theketch.io"},
			},
		},
	}

	tests := []struct {
		name        string
		application *ketchv1.App
		pool        *ketchv1.Pool
		opts        []Option

		wantYamlsFilename string
		wantErr           bool
	}{
		{
			name: "istio templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			pool:              poolWithClusterIssuer,
			wantYamlsFilename: "dashboard-istio-cluster-issuer",
		},
		{
			name: "istio templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			pool:              poolWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-istio",
		},
		{
			name: "traefik templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			pool:              poolWithClusterIssuer,
			wantYamlsFilename: "dashboard-traefik-cluster-issuer",
		},
		{
			name: "traefik templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			pool:              poolWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-traefik",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.application, tt.pool, tt.opts...)
			if tt.wantErr {
				require.Nil(t, err, "New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			expectedFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s.yaml", tt.wantYamlsFilename))
			actualFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s-output.yaml", tt.wantYamlsFilename))

			chartConfig := ChartConfig{
				Version: "0.0.1",
				AppName: tt.application.Name,
			}
			client := HelmClient{cfg: &action.Configuration{KubeClient: &fake.PrintingKubeClient{}, Releases: storage.Init(driver.NewMemory())}, namespace: tt.pool.Spec.NamespaceName}

			release, err := client.UpdateChart(*got, chartConfig, func(install *action.Install) {
				install.DryRun = true
				install.ClientOnly = true
			})
			require.Nil(t, err)

			actualManifests := strings.TrimSpace(release.Manifest)
			err = ioutil.WriteFile(actualFilename, []byte(actualManifests), 0755)
			require.Nil(t, err)
			expected, err := ioutil.ReadFile(expectedFilename)
			require.Nil(t, err)
			require.Equal(t, string(expected), actualManifests)
		})
	}
}

func TestNewAppChart(t *testing.T) {
	app := &ketchv1.App{
		Spec: ketchv1.AppSpec{
			Components: []ketchv1.ComponentLink{{
				Name: "Frontend",
				Type: "webserver",
				Properties: map[string]runtime.RawExtension{
					"image": runtime.RawExtension{Raw: []byte("image: me-my-frontend:1.2.3")},
				},
			}},
		},
	}
	app.SetName("test-app")

	tests := []struct {
		name        string
		application *ketchv1.App
		components  map[ketchv1.ComponentType]ketchv1.ComponentSpec
		traits      map[ketchv1.TraitType]ketchv1.TraitSpec

		wantYamlsFilename string
		wantErr           bool
	}{
		{
			name:        "success",
			application: app,
			components: map[ketchv1.ComponentType]ketchv1.ComponentSpec{
				"webserver": ketchv1.ComponentSpec{
					Schematic: ketchv1.Schematic{
						Kube: &ketchv1.Kube{
							Templates: []ketchv1.KubeTemplate{{
								Template: runtime.RawExtension{Raw: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  selector:
    matchLabels:
      app.ketch.io/component: frontend
  template:
    metadata:
      labels:
        app.ketch.io/component: frontend
    spec:
      containers:
      - name: frontend
        image: some/image:latest
        ports:
        - containerPort: 80
        livenessProbe:
          httpGet:
            path: /
            port: 80
          readinessProbe:
          httpGet:
            path: /
            port: 80
`)},
								Parameters: []ketchv1.Parameter{{
									Name:     "image",
									Required: true,
									Type:     "string",
									FieldPaths: []string{
										"spec.template.spec.containers[0].image",
									},
								}},
							}},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAppChart(tt.application, tt.components, tt.traits)
			if tt.wantErr {
				require.Nil(t, err, "New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Nil(t, err)

			t.Log(got)

			//chartConfig := ChartConfig{
			//	Version: "0.0.1",
			//	AppName: tt.application.Name,
			//}
			//client := HelmClient{cfg: &action.Configuration{KubeClient: &fake.PrintingKubeClient{}, Releases: storage.Init(driver.NewMemory())}, namespace: "mock-namespace"}
			//release, err := client.UpdateChart(*got, chartConfig, func(install *action.Install) {
			//	install.DryRun = true
			//	install.ClientOnly = true
			//})
			//require.Nil(t, err)
			//
			//fmt.Println(release)
		})
	}
}

func TestRenderComponentTemplates(t *testing.T) {
	componentSpec := &ketchv1.ComponentSpec{
		Schematic: ketchv1.Schematic{
			Kube: &ketchv1.Kube{
				Templates: []ketchv1.KubeTemplate{{
					Template: runtime.RawExtension{Raw: []byte(`
apiVersion: apps/v1
kind: Deployment
spec:
  selector:
    matchLabels:
      app.ketch.io/component: frontend
  template:
    metadata:
      labels:
        app.ketch.io/component: frontend
    spec:
      containers:
        - name: frontend
          image: some/image:latest
          ports:
            - containerPort: 80
          livenessProbe:
            httpGet:
              path: /
              port: 80
            readinessProbe:
              httpGet:
                path: /
                port: 80
`)},
					Parameters: []ketchv1.Parameter{{
						Name:     "image",
						Required: true,
						Type:     "string",
						FieldPaths: []string{
							"spec.template.spec.containers[0].image",
						},
					}},
				}},
			},
		},
	}
	tests := []struct {
		componentSpec *ketchv1.ComponentSpec
		componentLink *ketchv1.ComponentLink
		details       string
		err           error
	}{
		{
			componentSpec: componentSpec,
			componentLink: &ketchv1.ComponentLink{},
			details:       "happy",
		},
	}
	for _, test := range tests {
		res, err := RenderComponentTemplates(test.componentSpec, "test-component")
		require.Nil(t, err)
		t.Log(res)
	}
}

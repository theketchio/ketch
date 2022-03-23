package chart

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/templates"
	"github.com/theketchio/ketch/internal/utils/conversions"
)

func TestNewApplicationChart(t *testing.T) {

	const chartDirectory = "./testdata/charts/"

	frameworkWithClusterIssuer := &ketchv1.Framework{
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "ingress-class",
				ServiceEndpoint: "10.10.10.10",
				ClusterIssuer:   "letsencrypt-production",
			},
		},
	}
	frameworkWithoutClusterIssuer := &ketchv1.Framework{
		ObjectMeta: metav1.ObjectMeta{
			Name: "framework",
		},
		Spec: ketchv1.FrameworkSpec{
			NamespaceName: "ketch-gke",
			IngressController: ketchv1.IngressControllerSpec{
				ClassName:       "gke",
				ServiceEndpoint: "20.20.20.20",
			},
		},
	}
	exportedPorts := map[ketchv1.DeploymentVersion][]ketchv1.ExposedPort{
		3: {{Port: 9090, Protocol: "TCP"}},
		4: {{Port: 9091, Protocol: "TCP"}},
	}
	memorySize := resource.NewQuantity(5*1024*1024*1024, resource.BinarySI)
	cores := resource.NewMilliQuantity(5300, resource.DecimalSI)
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
						{
							Name:  "web",
							Units: conversions.IntPtr(3),
							Cmd:   []string{"python"},
							Env: []ketchv1.Env{
								{Name: "TEST_API_KEY", Value: "SECRET"},
								{Name: "TEST_API_URL", Value: "example.com"},
							},
							Resources: &v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    *memorySize,
									v1.ResourceMemory: *cores,
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    *memorySize,
									v1.ResourceMemory: *cores,
								},
							},
							Volumes: []v1.Volume{{
								Name: "test-volume",
								VolumeSource: v1.VolumeSource{
									AWSElasticBlockStore: &v1.AWSElasticBlockStoreVolumeSource{
										VolumeID: "volume-id",
										FSType:   "ext4",
									},
								},
							}},
							VolumeMounts: []v1.VolumeMount{{
								MountPath: "/test-ebs",
								Name:      "test-volume",
							}},
						},
						{Name: "worker", Units: conversions.IntPtr(1), Cmd: []string{"celery"}},
					},
					RoutingSettings: ketchv1.RoutingSettings{
						Weight: 30,
					},
					ImagePullSecrets: []v1.LocalObjectReference{
						{Name: "registry-secret"},
						{Name: "private-registry-secret"},
					},
					KetchYaml: &ketchv1.KetchYamlData{
						Healthcheck: &ketchv1.KetchYamlHealthcheck{
							Path:            "/actuator/health/liveness",
							Method:          "GET",
							Scheme:          "http",
							Match:           ".*UP.*",
							UseInRouter:     true,
							ForceRestart:    false,
							AllowedFailures: 0,
							IntervalSeconds: 0,
							TimeoutSeconds:  0,
						},
					},
				},
				{
					Image:   "shipasoftware/go-app:v2",
					Version: 4,
					Processes: []ketchv1.ProcessSpec{
						{
							Name:  "web",
							Units: conversions.IntPtr(3),
							Cmd:   []string{"python"},
						},
						{Name: "worker", Units: conversions.IntPtr(1), Cmd: []string{"celery"}},
					},
					RoutingSettings: ketchv1.RoutingSettings{
						Weight: 70,
					},
				},
			},
			Env: []ketchv1.Env{
				{Name: "VAR", Value: "VALUE"},
			},
			Framework: "framework",
			DockerRegistry: ketchv1.DockerRegistrySpec{
				SecretName: "default-image-pull-secret",
			},
			Ingress: ketchv1.IngressSpec{
				GenerateDefaultCname: true,
				Cnames: []ketchv1.Cname{
					{Name: "theketch.io", Secure: true},
					{Name: "app.theketch.io", Secure: true},
					{Name: "darkweb.theketch.io", Secure: true, SecretName: "darkweb-ssl"},
				},
			},
			Labels: []ketchv1.MetadataItem{
				{
					Apply:             map[string]string{"pod.io/label": "pod-label"},
					DeploymentVersion: 3,
					ProcessName:       "web",
					Target:            ketchv1.Target{APIVersion: "v1", Kind: "Pod"},
				},
				{
					Apply:             map[string]string{"theketch.io/test-label": "test-label-value"},
					DeploymentVersion: 3,
					ProcessName:       "web",
					Target:            ketchv1.Target{APIVersion: "apps/v1", Kind: "Deployment"},
				},
				{
					Apply:  map[string]string{"theketch.io/test-label-all": "test-label-value-all"},
					Target: ketchv1.Target{APIVersion: "apps/v1", Kind: "Deployment"},
				},
			},
			Annotations: []ketchv1.MetadataItem{
				{
					Apply:             map[string]string{"pod.io/annotation": "pod-annotation"},
					DeploymentVersion: 3,
					ProcessName:       "web",
					Target:            ketchv1.Target{APIVersion: "v1", Kind: "Pod"},
				},
				{
					Apply:             map[string]string{"theketch.io/test-annotation": "test-annotation-value"},
					DeploymentVersion: 4,
					ProcessName:       "web",
					Target: ketchv1.Target{
						APIVersion: "v1",
						Kind:       "Service",
					},
				},
				{
					Apply: map[string]string{"theketch.io/gateway-annotation": "test-gateway"},
					Target: ketchv1.Target{
						APIVersion: "networking.istio.io/v1alpha3",
						Kind:       "Gateway",
					},
				},
				{
					Apply: map[string]string{"theketch.io/ingress-annotation": "test-ingress"},
					Target: ketchv1.Target{
						APIVersion: "networking.k8s.io/v1",
						Kind:       "Ingress",
					},
				},
				{
					Apply: map[string]string{"theketch.io/ingress-route-annotation": "test-ingress"},
					Target: ketchv1.Target{
						APIVersion: "traefik.containo.us/v1alpha1",
						Kind:       "IngressRoute",
					},
				},
			},
		},
	}

	setServiceAccount := func(app *ketchv1.App) *ketchv1.App {
		out := *app
		out.Spec.ServiceAccountName = "custom-service-account"
		return &out
	}
	// convertSecureEndpoints returns a copy of app with Cnames made not secure
	convertSecureEndpoints := func(app *ketchv1.App) *ketchv1.App {
		out := *app
		out.Spec.Ingress.Cnames = []ketchv1.Cname{}
		for _, cname := range app.Spec.Ingress.Cnames {
			out.Spec.Ingress.Cnames = append(out.Spec.Ingress.Cnames, ketchv1.Cname{Name: cname.Name, Secure: false})
		}
		return &out
	}
	setPodSecurityContext := func(app *ketchv1.App) *ketchv1.App {
		out := *app
		fsGroup := int64(2000)
		runAsUser := int64(3000)
		out.Spec.SecurityContext = &v1.PodSecurityContext{
			FSGroup:   &fsGroup,
			RunAsUser: &runAsUser,
		}
		return &out
	}
	setVolumeClaimTemplates := func(app *ketchv1.App) *ketchv1.App {
		out := *app
		storageClass := "standard"
		out.Spec.VolumeClaimTemplates = []ketchv1.PersistentVolumeClaim{
			{
				Name:             "v1-shipa",
				AccessModes:      []v1.PersistentVolumeAccessMode{"ReadWriteMany"},
				StorageClassName: &storageClass,
				Storage:          "1Gi",
			},
			{
				Name:             "v2-shipa",
				AccessModes:      []v1.PersistentVolumeAccessMode{"ReadWriteOnce"},
				StorageClassName: &storageClass,
				Storage:          "1Gi",
			},
		}
		return &out
	}
	setStatefulSet := func(app *ketchv1.App) *ketchv1.App {
		out := *app
		appType := ketchv1.StatefulSetAppType
		out.Spec.Type = &appType
		return &out
	}

	tests := []struct {
		name        string
		application *ketchv1.App
		framework   *ketchv1.Framework
		opts        []Option
		group       string

		wantYamlsFilename string
		wantErr           bool
	}{
		{
			name: "nginx templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.NginxDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-nginx-cluster-issuer",
		},
		{
			name: "nginx templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.NginxDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       setServiceAccount(convertSecureEndpoints(dashboard)),
			framework:         frameworkWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-nginx",
		},
		{
			name: "istio templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-istio-cluster-issuer",
		},
		{
			name: "istio templates with cluster issuer and pod security context",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       setPodSecurityContext(dashboard),
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-istio-cluster-issuer-pod-security-context",
		},
		{
			name: "istio templates with cluster issuer and volume claim templates",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       setStatefulSet(setVolumeClaimTemplates(dashboard)),
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-istio-cluster-issuer-volume-claim-templates",
		},
		{
			name: "istio templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.IstioDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       convertSecureEndpoints(dashboard),
			framework:         frameworkWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-istio",
		},
		{
			name: "traefik templates with cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-traefik-cluster-issuer",
		},
		{
			name: "traefik templates without cluster issuer",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       convertSecureEndpoints(dashboard),
			framework:         frameworkWithoutClusterIssuer,
			wantYamlsFilename: "dashboard-traefik",
		},
		{
			name: "traefik templates with cluster issuer and resource requirements",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			wantYamlsFilename: "dashboard-traefik-cluster-issuer",
		},
		{
			name: "traefik templates with cluster issuer w/ alternate group",
			opts: []Option{
				WithTemplates(templates.TraefikDefaultTemplates),
				WithExposedPorts(exportedPorts),
			},
			application:       dashboard,
			framework:         frameworkWithClusterIssuer,
			group:             "shipa.io",
			wantYamlsFilename: "dashboard-traefik-cluster-issuer-shipa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.group != "" {
				original := ketchv1.Group
				ketchv1.Group = tt.group
				defer func() {
					ketchv1.Group = original
				}()
			}
			got, err := New(tt.application, tt.framework, tt.opts...)
			if tt.wantErr {
				require.NotNil(t, err, "New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Nil(t, err)

			expectedFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s.yaml", tt.wantYamlsFilename))
			actualFilename := filepath.Join(chartDirectory, fmt.Sprintf("%s.output.yaml", tt.wantYamlsFilename))

			chartConfig := ChartConfig{
				Version: "0.0.1",
				AppName: tt.application.Name,
			}

			client := HelmClient{cfg: &action.Configuration{KubeClient: &fake.PrintingKubeClient{}, Releases: storage.Init(driver.NewMemory())}, namespace: tt.framework.Spec.NamespaceName, c: clientfake.NewClientBuilder().Build()}
			release, err := client.UpdateChart(*got, chartConfig, func(install *action.Install) {
				install.DryRun = true
				install.ClientOnly = true
			})
			require.Nil(t, err, "error = %v", err)

			actualManifests := strings.TrimSpace(release.Manifest)
			err = ioutil.WriteFile(actualFilename, []byte(actualManifests), 0755)
			require.Nil(t, err)
			expected, err := ioutil.ReadFile(expectedFilename)
			require.Nil(t, err)
			require.Equal(t, string(expected), actualManifests)
		})
	}
}

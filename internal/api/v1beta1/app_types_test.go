package v1beta1

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intRef(i int) *int {
	return &i
}

func versionRef(i DeploymentVersion) *DeploymentVersion {
	return &i
}

func stringRef(s string) *string {
	return &s
}

func defaultSpec() AppSpec {
	return AppSpec{
		Deployments: []AppDeploymentSpec{
			{
				Version: 1,
				Processes: []ProcessSpec{
					{Name: "web", Units: intRef(1)},
					{Name: "worker", Units: intRef(2)},
				},
			},
			{
				Version: 2,
				Processes: []ProcessSpec{
					{Name: "web", Units: intRef(0)},
					{Name: "worker", Units: intRef(0)},
				},
			},
		},
	}
}

func TestApp_SetUnits(t *testing.T) {
	tests := []struct {
		name string

		spec     AppSpec
		selector Selector

		wantSpec AppSpec
		wantErr  error
	}{
		{
			name:     "empty selector, process not found",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("database")},
			wantErr:  ErrProcessNotFound,
		},
		{
			name:     "empty selector, deployment not found",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(8)},
			wantErr:  ErrDeploymentNotFound,
		},
		{
			name:     "empty selector, all processes of all deployments",
			spec:     defaultSpec(),
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
		{
			name:     "filter by process",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("web")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment and process",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2), Process: stringRef("worker")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(4)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Spec: tt.spec}
			if err := app.SetUnits(tt.selector, 4); err != tt.wantErr {
				t.Errorf("SetUnits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_AddUnits(t *testing.T) {
	tests := []struct {
		name     string
		spec     AppSpec
		selector Selector
		quantity int
		wantSpec AppSpec
		wantErr  error
	}{
		{
			name:     "empty selector, process not found",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("database")},
			wantErr:  ErrProcessNotFound,
		},
		{
			name:     "empty selector, deployment not found",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(8)},
			wantErr:  ErrDeploymentNotFound,
		},
		{
			spec:     defaultSpec(),
			selector: Selector{},
			quantity: 5,
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(6)},
							{Name: "worker", Units: intRef(7)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(5)},
							{Name: "worker", Units: intRef(5)},
						},
					},
				},
			},
		},
		{
			name:     "filter by process",
			spec:     defaultSpec(),
			selector: Selector{Process: stringRef("web")},
			quantity: 3,
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(3)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2)},
			quantity: 3,
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(3)},
							{Name: "worker", Units: intRef(3)},
						},
					},
				},
			},
		},
		{
			name:     "filter by deployment and process",
			spec:     defaultSpec(),
			selector: Selector{DeploymentVersion: versionRef(2), Process: stringRef("worker")},
			quantity: 3,
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(3)},
						},
					},
				},
			},
		},
		{
			name:     "adding negative quantity",
			spec:     defaultSpec(),
			selector: Selector{},
			quantity: -1,
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(1)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{Spec: tt.spec}
			if err := app.AddUnits(tt.selector, tt.quantity); err != tt.wantErr {
				t.Errorf("AddUnits() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr != nil {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_SetEnvs(t *testing.T) {
	tests := []struct {
		name        string
		initialEnvs []Env
		setEnvs     []Env
		wantEnvs    map[string]Env
	}{
		{
			name: "no new variables",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			setEnvs: []Env{},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "true"},
				"THEKETCH": {Name: "THEKETCH", Value: "true"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name: "partially update",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			setEnvs: []Env{
				{Name: "KETCH", Value: "false"},
				{Name: "THEKETCH", Value: "1"},
			},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "false"},
				"THEKETCH": {Name: "THEKETCH", Value: "1"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name:        "update all",
			initialEnvs: []Env{},
			setEnvs: []Env{
				{Name: "KETCH", Value: "false"},
				{Name: "THEKETCH", Value: "1"},
			},
			wantEnvs: map[string]Env{
				"KETCH":    {Name: "KETCH", Value: "false"},
				"THEKETCH": {Name: "THEKETCH", Value: "1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			app.SetEnvs(tt.setEnvs)
			got := make(map[string]Env, len(app.Spec.Env))
			for _, env := range app.Spec.Env {
				got[env.Name] = env
			}
			if diff := cmp.Diff(got, tt.wantEnvs); diff != "" {
				t.Errorf("Envs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Envs(t *testing.T) {
	tests := []struct {
		name        string
		want        map[string]string
		initialEnvs []Env
		names       []string
	}{
		{
			name: "all the asked envs are present",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			names: []string{"KETCH", "API_KEY"},
			want: map[string]string{
				"KETCH":   "true",
				"API_KEY": "xxx",
			},
		},
		{
			name: "some of the asked envs are present",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
			},
			names: []string{"KETCH", "API_KEY", "SOME_VAR"},
			want: map[string]string{
				"KETCH": "true",
			},
		},
		{
			name:  "app has no envs",
			names: []string{"KETCH", "API_KEY", "SOME_VAR"},
			want:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			if got := app.Envs(tt.names); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Envs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_UnsetEnvs(t *testing.T) {
	tests := []struct {
		name        string
		initialEnvs []Env
		unset       []string
		wantEnvs    map[string]Env
	}{
		{
			name: "unset all",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			unset:    []string{"KETCH", "THEKETCH", "API_KEY", "SOME_KEY"},
			wantEnvs: map[string]Env{},
		},
		{
			name: "unset partially",
			initialEnvs: []Env{
				{Name: "KETCH", Value: "true"},
				{Name: "THEKETCH", Value: "true"},
				{Name: "API_KEY", Value: "xxx"},
			},
			unset: []string{"KETCH", "SOME_KEY"},
			wantEnvs: map[string]Env{
				"THEKETCH": {Name: "THEKETCH", Value: "true"},
				"API_KEY":  {Name: "API_KEY", Value: "xxx"},
			},
		},
		{
			name:        "app has no envs",
			initialEnvs: nil,
			unset:       []string{"KETCH", "SOME_KEY"},
			wantEnvs:    map[string]Env{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Env: tt.initialEnvs,
				},
			}
			app.UnsetEnvs(tt.unset)
			got := make(map[string]Env, len(app.Spec.Env))
			for _, env := range app.Spec.Env {
				got[env.Name] = env
			}
			if diff := cmp.Diff(got, tt.wantEnvs); diff != "" {
				t.Errorf("Envs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_DefaultURL(t *testing.T) {
	tests := []struct {
		name                 string
		appName              string
		generateDefaultCname bool
		pool                 *Pool
		want                 *string
	}{
		{
			name:                 "url with a custom domain",
			appName:              "app-1",
			generateDefaultCname: true,
			pool: &Pool{
				Spec: PoolSpec{
					IngressController: IngressControllerSpec{
						Domain:          "theketch.io",
						ServiceEndpoint: "10.20.30.40",
					},
				},
			},
			want: stringRef("app-1.10.20.30.40.theketch.io"),
		},
		{
			name:                 "url with default domain",
			appName:              "app-2",
			generateDefaultCname: true,
			pool: &Pool{
				Spec: PoolSpec{
					IngressController: IngressControllerSpec{
						ServiceEndpoint: "20.20.20.20",
					},
				},
			},
			want: stringRef("app-2.20.20.20.20.shipa.cloud"),
		},
		{
			name:                 "no service endpoint - no default url",
			appName:              "app-1",
			generateDefaultCname: true,
			pool:                 &Pool{},
		},
		{
			name:                 "do not generate default cname - no default url",
			appName:              "app-1",
			generateDefaultCname: false,
			pool: &Pool{
				Spec: PoolSpec{
					IngressController: IngressControllerSpec{
						ServiceEndpoint: "20.20.20.20",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.appName,
				},
				Spec: AppSpec{
					Ingress: IngressSpec{
						GenerateDefaultCname: tt.generateDefaultCname,
					},
				},
			}
			got := app.DefaultURL(tt.pool)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("DefaultURL() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_URLs(t *testing.T) {
	tests := []struct {
		name                 string
		generateDefaultCname bool
		cnames               []string
		want                 []string
	}{
		{
			name:                 "with default cname",
			generateDefaultCname: true,
			cnames:               []string{"theketch.io", "app.theketch.io"},
			want:                 []string{"ketch.10.20.30.40.theketch.io", "theketch.io", "app.theketch.io"},
		},
		{
			name:                 "without default cname",
			generateDefaultCname: false,
			cnames:               []string{"theketch.io", "app.theketch.io"},
			want:                 []string{"theketch.io", "app.theketch.io"},
		},
		{
			name:                 "empty cnames",
			generateDefaultCname: false,
			want:                 []string{},
		},
		{
			name:                 "empty cnames with default cname",
			generateDefaultCname: true,
			want:                 []string{"ketch.10.20.30.40.theketch.io"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ketch",
				},
				Spec: AppSpec{
					Ingress: IngressSpec{
						GenerateDefaultCname: tt.generateDefaultCname,
						Cnames:               tt.cnames,
					},
				},
			}
			pool := &Pool{
				Spec: PoolSpec{
					IngressController: IngressControllerSpec{
						Domain:          "theketch.io",
						ServiceEndpoint: "10.20.30.40",
					},
				},
			}
			got := app.URLs(pool)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("URLs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_TemplatesConfigMapName(t *testing.T) {
	tests := []struct {
		name        string
		app         App
		ingressType IngressControllerType

		want string
	}{
		{
			name: "user provided configmap",
			app: App{
				Spec: AppSpec{
					Chart: ChartSpec{
						TemplatesConfigMapName: stringRef("user-provided-configmap"),
					},
				},
			},
			want: "user-provided-configmap",
		},
		{
			name:        "istio configmap",
			app:         App{},
			ingressType: IstioIngressControllerType,
			want:        "ingress-istio-templates",
		},
		{
			name:        "traefik configmap",
			app:         App{},
			ingressType: Traefik17IngressControllerType,
			want:        "ingress-traefik17-templates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.app.TemplatesConfigMapName(tt.ingressType)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("TemplatesConfigMapName() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Units(t *testing.T) {
	tests := []struct {
		name string
		app  App
		want int
	}{
		{
			name: "app without units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version:   1,
							Processes: []ProcessSpec{},
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "app with units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(8)},
								{Name: "worker", Units: intRef(10)},
							},
						},
					},
				},
			},
			want: 21,
		},
		{
			name: "each process has default number of units",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Version: 1, Processes: []ProcessSpec{{Name: "web"}, {Name: "worker"}}},
						{Version: 2, Processes: []ProcessSpec{{Name: "web"}, {Name: "worker"}}},
					},
				},
			},
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.Units(); got != tt.want {
				t.Errorf("Units() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApp_Stop(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector

		wantSpec AppSpec
		wantErr  bool
	}{
		{
			name:     "stop by process",
			selector: Selector{Process: stringRef("web")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(5)},
						},
					},
				},
			},
		},
		{
			name:     "stop by deployment",
			selector: Selector{DeploymentVersion: versionRef(1)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
						},
					},
				},
			},
		},
		{
			name:     "stop by deployment and process",
			selector: Selector{DeploymentVersion: versionRef(2), Process: stringRef("worker")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "stop all",
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(0)},
							{Name: "worker", Units: intRef(0)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(4)},
								{Name: "worker", Units: intRef(5)},
							},
						},
					},
				},
			}
			if err := app.Stop(tt.selector); (err != nil) != tt.wantErr {
				t.Errorf("Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_Start(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector

		wantSpec AppSpec
		wantErr  bool
	}{
		{
			name:     "start all",
			selector: Selector{},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
				},
			},
		},
		{
			name:     "start by process",
			selector: Selector{Process: stringRef("db")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
				},
			},
		},
		{
			name:     "start by deployment",
			selector: Selector{DeploymentVersion: versionRef(2)},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db"},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(1)},
						},
					},
				},
			},
		},
		{
			name:     "start by deployment and process",
			selector: Selector{DeploymentVersion: versionRef(1), Process: stringRef("db")},
			wantSpec: AppSpec{
				Deployments: []AppDeploymentSpec{
					{
						Version: 1,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(1)},
							{Name: "worker", Units: intRef(2)},
							{Name: "db", Units: intRef(1)},
							{Name: "db-2", Units: intRef(0)},
						},
					},
					{
						Version: 2,
						Processes: []ProcessSpec{
							{Name: "web", Units: intRef(4)},
							{Name: "worker", Units: intRef(5)},
							{Name: "db"},
							{Name: "db-2", Units: intRef(0)},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{
							Version: 1,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(1)},
								{Name: "worker", Units: intRef(2)},
								{Name: "db"},
								{Name: "db-2", Units: intRef(0)},
							},
						},
						{
							Version: 2,
							Processes: []ProcessSpec{
								{Name: "web", Units: intRef(4)},
								{Name: "worker", Units: intRef(5)},
								{Name: "db"},
								{Name: "db-2", Units: intRef(0)},
							},
						},
					},
				},
			}
			if err := app.Start(tt.selector); (err != nil) != tt.wantErr {
				t.Errorf("Stop() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(app.Spec, tt.wantSpec); diff != "" {
				t.Errorf("AppSpec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

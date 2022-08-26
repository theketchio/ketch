package v1beta1

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
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

func TestApp_DefaultCname(t *testing.T) {
	tests := []struct {
		name                 string
		appName              string
		generateDefaultCname bool
		ingressController    IngressControllerSpec
		want                 *string
	}{
		{
			name:                 "cname with default domain",
			appName:              "app-2",
			generateDefaultCname: true,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "20.20.20.20"},
			want:                 stringRef("app-2.20.20.20.20.shipa.cloud"),
		},
		{
			name:                 "no service endpoint - no default cname",
			appName:              "app-1",
			generateDefaultCname: true,
		},
		{
			name:                 "do not generate default cname - no default cname",
			appName:              "app-1",
			generateDefaultCname: false,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "20.20.20.20"},
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
						Controller:           tt.ingressController,
					},
				},
			}
			got := app.DefaultCname()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("DefaultCname() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApp_CNames(t *testing.T) {
	tests := []struct {
		name                 string
		generateDefaultCname bool
		ingressController    IngressControllerSpec
		cnames               []Cname
		want                 []string
	}{
		{
			name:                 "with default cname",
			generateDefaultCname: true,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "10.20.30.40"},
			cnames:               []Cname{{Name: "theketch.io"}, {Name: "app.theketch.io"}},
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud", "http://theketch.io", "http://app.theketch.io"},
		},
		{
			name:                 "with default cname and framework with cluster issuer",
			generateDefaultCname: true,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "10.20.30.40", ClusterIssuer: "letsencrypt"},
			cnames:               []Cname{{Name: "theketch.io"}, {Name: "app.theketch.io", Secure: true}},
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud", "http://theketch.io", "https://app.theketch.io"},
		},
		{
			name:                 "without default cname",
			generateDefaultCname: false,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "10.20.30.40"},
			cnames:               []Cname{{Name: "theketch.io"}, {Name: "app.theketch.io"}},
			want:                 []string{"http://theketch.io", "http://app.theketch.io"},
		},
		{
			name:                 "empty cnames",
			generateDefaultCname: false,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "10.20.30.40"},
			want:                 []string{},
		},
		{
			name:                 "empty cnames with default cname",
			generateDefaultCname: true,
			ingressController:    IngressControllerSpec{ServiceEndpoint: "10.20.30.40"},
			want:                 []string{"http://ketch.10.20.30.40.shipa.cloud"},
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
						Controller:           tt.ingressController,
					},
				},
			}
			got := app.CNames()
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CNames() mismatch (-want +got):\n%s", diff)
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
			err := app.Start(tt.selector)
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantSpec, app.Spec)
		})
	}
}

func TestApp_Phase(t *testing.T) {
	tests := []struct {
		name string
		app  App
		want AppPhase
	}{
		{
			name: "1 unit - status is running",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Processes: []ProcessSpec{{Units: intRef(1)}}},
					},
				},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionTrue},
					},
				},
			},
			want: AppRunning,
		},
		{
			name: "schedule issue - status is error",
			app: App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{
						{Processes: []ProcessSpec{{Units: intRef(1)}}},
					},
				},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionFalse},
					},
				},
			},
			want: AppError,
		},
		{
			name: "no units - status is created",
			app: App{
				Spec: AppSpec{},
				Status: AppStatus{
					Conditions: []Condition{
						{Type: Scheduled, Status: v1.ConditionTrue},
					},
				},
			},
			want: AppCreated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := tt.app.Phase()
			require.Equal(t, tt.want, phase)
		})
	}
}

func TestApp_SetCondition(t *testing.T) {

	t1 := metav1.NewTime(time.Now())
	t2 := metav1.NewTime(time.Now())

	tests := []struct {
		name              string
		currentConditions []Condition
		wantConditions    []Condition
	}{
		{
			name:              "add condition",
			currentConditions: nil,
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "message updated",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "old-message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "status updated",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionFalse, LastTransitionTime: &t1, Message: "message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t2, Message: "message"},
			},
		},
		{
			name:              "no need to update",
			currentConditions: []Condition{{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "message"}},
			wantConditions: []Condition{
				{Type: Scheduled, Status: v1.ConditionTrue, LastTransitionTime: &t1, Message: "message"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{
				Status: AppStatus{
					Conditions: tt.currentConditions,
				},
			}
			condition := Condition{
				Type:               Scheduled,
				Status:             v1.ConditionTrue,
				LastTransitionTime: &t2,
				Message:            "message",
			}
			app.SetCondition(condition.Type, condition.Status, condition.Message, *condition.LastTransitionTime)
			require.Equal(t, tt.wantConditions, app.Status.Conditions)
		})
	}
}

func TestApp_DoCanary(t *testing.T) {

	timeRef := func(hours int, minutes int) *metav1.Time {
		t := metav1.Date(2021, 2, 1, hours, minutes, 0, 0, time.UTC)
		return &t
	}
	tests := []struct {
		name           string
		app            App
		now            metav1.Time
		disableScaling map[string]bool
		wantNoChanges  bool
		wantApp        App
		wantErr        string
	}{
		{
			name: "happy path - do canary",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       1,
						Active:            true,
						Target:            map[string]uint16{"p1": 8},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 67}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 33}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 40),
						CurrentStep:       2,
						Active:            true,
						Target:            map[string]uint16{"p1": 8},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(2)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(6)}}},
					},
				},
			},
		},
		{
			name: "happy path - the last step of canary",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       3,
						Active:            true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:           3,
						StepWeight:      33,
						StepTimeInteval: 10 * time.Minute,
						CurrentStep:     4,
						Active:          false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 100}},
					},
				},
			},
		},
		{
			// process not in target should be updated to 1 unit
			name: "updated version's process not in target",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       1,
						Active:            true,
						Target:            map[string]uint16{"p1": 7},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 80}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 20}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}, {Name: "p2", Units: intRef(4)}}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 40),
						CurrentStep:       2,
						Active:            true,
						Target:            map[string]uint16{"p1": 7},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 60}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(4)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 40}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(3)}, {Name: "p2", Units: intRef(1)}}},
					},
				},
			},
		},
		{
			// process not in target should keep its units until canary completes
			name: "previous version's process not in target",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       1,
						Active:            true,
						Target:            map[string]uint16{"p1": 7},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 80}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}, {Name: "p2", Units: intRef(4)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 20}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(1)}}},
					},
				},
			},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 40),
						CurrentStep:       2,
						Active:            true,
						Target:            map[string]uint16{"p1": 7},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 60}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(4)}, {Name: "p2", Units: intRef(4)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 40}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(3)}}},
					},
				},
			},
		},
		{
			name:          "canary is not active - no changes",
			now:           *timeRef(10, 31),
			wantNoChanges: true,
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       2,
						Active:            false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			name:          "too early to do canary - no changes",
			now:           *timeRef(10, 25),
			wantNoChanges: true,
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             3,
						StepWeight:        33,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       2,
						Active:            false,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			name:          "error - nextScheduledTime is not set",
			now:           *timeRef(10, 45),
			wantNoChanges: true,
			wantErr:       "canary is active but the next step is not scheduled",
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:           3,
						StepWeight:      33,
						StepTimeInteval: 10 * time.Minute,
						CurrentStep:     2,
						Active:          true,
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 34}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 66}},
					},
				},
			},
		},
		{
			// process' weight should still change, but units should remain the same if specified in disableScaling
			name: "disable pod scaling per process",
			now:  *timeRef(10, 31),
			app: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 30),
						CurrentStep:       1,
						Active:            true,
						Target:            map[string]uint16{"p1": 7, "p2": 10},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 80}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(7)}, {Name: "p2", Units: intRef(8)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 20}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(7)}, {Name: "p2", Units: intRef(2)}}},
					},
				},
			},
			disableScaling: map[string]bool{"p1": true},
			wantApp: App{
				Spec: AppSpec{
					Canary: CanarySpec{
						Steps:             5,
						StepWeight:        20,
						StepTimeInteval:   10 * time.Minute,
						NextScheduledTime: timeRef(10, 40),
						CurrentStep:       2,
						Active:            true,
						Target:            map[string]uint16{"p1": 7, "p2": 10},
					},
					Deployments: []AppDeploymentSpec{
						{Version: 2, RoutingSettings: RoutingSettings{Weight: 60}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(7)}, {Name: "p2", Units: intRef(6)}}},
						{Version: 3, RoutingSettings: RoutingSettings{Weight: 40}, Processes: []ProcessSpec{{Name: "p1", Units: intRef(7)}, {Name: "p2", Units: intRef(4)}}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.app.DoCanary(tt.now, logr.Discard(), &record.FakeRecorder{}, tt.disableScaling)
			originalApp := *tt.app.DeepCopy()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, err.Error(), tt.wantErr)
				return
			}
			require.Nil(t, err)

			if tt.wantNoChanges {
				require.Equal(t, originalApp, tt.app)
				return
			}
			fmt.Printf("%+v\n", tt.app)
			require.Equal(t, tt.wantApp, tt.app)
		})
	}
}

func TestAppAddLabel(t *testing.T) {
	tests := []struct {
		description string
		app         *App
		labels      map[string]string
		target      Target
		expected    []MetadataItem
	}{
		{
			description: "add label",
			app: &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{{
						Version: 1,
						Processes: []ProcessSpec{{
							Name: "process-1",
						}},
					}},
				},
			},
			labels: map[string]string{"mykey": "myvalue"},
			target: Target{Kind: "Pod", APIVersion: "v1"},
			expected: []MetadataItem{{
				Apply:             map[string]string{"mykey": "myvalue"},
				DeploymentVersion: 1,
				Target:            Target{Kind: "Pod", APIVersion: "v1"},
				ProcessName:       "process-1",
			}},
		},
		{
			description: "replace label",
			app: &App{
				Spec: AppSpec{
					Deployments: []AppDeploymentSpec{{
						Version: 1,
						Processes: []ProcessSpec{{
							Name: "process-1",
						}},
					}},
					Labels: []MetadataItem{
						{
							Apply:             map[string]string{"mykey": "myOLDvalue"},
							DeploymentVersion: 1,
							Target:            Target{Kind: "Pod", APIVersion: "v1"},
							ProcessName:       "process-1",
						},
						{
							Apply:             map[string]string{"mykey": "myDEPLOYMENTvalue"},
							DeploymentVersion: 1,
							Target:            Target{Kind: "Deployment", APIVersion: "v1"},
							ProcessName:       "process-1",
						},
						{
							Apply:             map[string]string{"anotherkey": "myRETAINEDvalue"},
							DeploymentVersion: 1,
							Target:            Target{Kind: "Pod", APIVersion: "v1"},
							ProcessName:       "process-1",
						},
					},
				},
			},
			labels: map[string]string{"mykey": "myvalue"},
			target: Target{Kind: "Pod", APIVersion: "v1"},
			expected: []MetadataItem{
				{
					Apply:             map[string]string{"mykey": "myDEPLOYMENTvalue"},
					DeploymentVersion: 1,
					Target:            Target{Kind: "Deployment", APIVersion: "v1"},
					ProcessName:       "process-1",
				},
				{
					Apply:             map[string]string{"anotherkey": "myRETAINEDvalue"},
					DeploymentVersion: 1,
					Target:            Target{Kind: "Pod", APIVersion: "v1"},
					ProcessName:       "process-1",
				},
				{
					Apply:             map[string]string{"mykey": "myvalue"},
					DeploymentVersion: 1,
					Target:            Target{Kind: "Pod", APIVersion: "v1"},
					ProcessName:       "process-1",
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tc.app.AddLabel(tc.labels, tc.target)
			fmt.Println(tc.expected)
			fmt.Println(tc.app.Spec.Labels)
			require.EqualValues(t, tc.expected, tc.app.Spec.Labels)
		})
	}
}

func TestValidateMetadataItem(t *testing.T) {
	tests := []struct {
		description  string
		metadataItem MetadataItem
		expected     error
	}{
		{
			description: "ok",
			metadataItem: MetadataItem{
				Apply: map[string]string{"theketch.io/test-item": "test-value"},
			},
			expected: nil,
		},
		{
			description: "ok - no prefix",
			metadataItem: MetadataItem{
				Apply: map[string]string{"test-item": "test-value"},
			},
			expected: nil,
		},
		{
			description: "must begin with alphanumeric",
			metadataItem: MetadataItem{
				Apply: map[string]string{"_theketch.io/test-item": "test-value"},
			},
			expected: errors.New("malformed metadata key"),
		},
		{
			description: "invalid characters",
			metadataItem: MetadataItem{
				Apply: map[string]string{"theketch.io/test@item": "test-value"},
			},
			expected: errors.New("malformed metadata key"),
		},
	}
	for _, tt := range tests {
		err := tt.metadataItem.Validate()
		require.Equal(t, tt.expected, err, tt.description)
	}
}

func TestGetUpdatedUnits(t *testing.T) {
	tests := []struct {
		name   string
		weight uint8
		target uint16

		expectedSource int
		expectedDest   int
	}{
		{
			name:   "small weight, big target",
			weight: 1,
			target: 10,

			expectedSource: 1,
			expectedDest:   10,
		},
		{
			name:   "big weight, small target",
			weight: 75,
			target: 1,

			expectedSource: 1,
			expectedDest:   1,
		},
		{
			name:   "big weight, small target",
			weight: 50,
			target: 1,

			expectedSource: 1,
			expectedDest:   1,
		},
		{
			name:   "equal distribution",
			weight: 50,
			target: 2,

			expectedSource: 1,
			expectedDest:   1,
		},
		{
			name:   "more in second",
			weight: 50,
			target: 3,

			expectedSource: 1,
			expectedDest:   2,
		},
		{
			name:   "odd number",
			weight: 33,
			target: 10,

			expectedSource: 3,
			expectedDest:   7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, dest := getUpdatedUnits(tt.weight, tt.target)
			if source != tt.expectedSource || dest != tt.expectedDest {
				t.Errorf("FAIL: got source: %d, want source: %d, got dest: %d, want dest: %d", source, tt.expectedSource, dest, tt.expectedDest)
			}
		})
	}
}

func TestCanaryEvent_Message(t *testing.T) {
	expectedAnnotations := map[string]string{
		CanaryAnnotationAppName:            "app1",
		CanaryAnnotationDevelopmentVersion: "2",
		CanaryAnnotationDescription:        "started",
		CanaryAnnotationEventName:          "CanaryStarted",
	}
	event := newCanaryEvent(&App{
		ObjectMeta: metav1.ObjectMeta{Name: "app1"},
		Spec: AppSpec{
			Canary: CanarySpec{CurrentStep: 10},
			Deployments: []AppDeploymentSpec{
				{Version: 1, RoutingSettings: RoutingSettings{Weight: 30}},
				{Version: 2, RoutingSettings: RoutingSettings{Weight: 70}},
			},
		}},
		CanaryStarted, CanaryStartedDesc,
	)
	require.Equal(t, expectedAnnotations, event.Annotations)
}

func TestCanaryNextStepEvent_Message(t *testing.T) {
	expectedAnnotations := map[string]string{
		CanaryAnnotationAppName:            "app1",
		CanaryAnnotationDevelopmentVersion: "2",
		CanaryAnnotationDescription:        "weight change",
		CanaryAnnotationEventName:          "CanaryNextStep",
		CanaryAnnotationStep:               "10",
		CanaryAnnotationVersionDest:        "2",
		CanaryAnnotationVersionSource:      "1",
		CanaryAnnotationWeightDest:         "70",
		CanaryAnnotationWeightSource:       "30",
	}
	event := newCanaryNextStepEvent(&App{
		ObjectMeta: metav1.ObjectMeta{Name: "app1"},
		Spec: AppSpec{
			Canary: CanarySpec{CurrentStep: 10},
			Deployments: []AppDeploymentSpec{
				{Version: 1, RoutingSettings: RoutingSettings{Weight: 30}},
				{Version: 2, RoutingSettings: RoutingSettings{Weight: 70}},
			},
		}},
	)
	require.Equal(t, expectedAnnotations, event.Event.Annotations)
}

func TestCanaryTargetChangeEvent_Annotations(t *testing.T) {
	expectedAnnotations := map[string]string{
		CanaryAnnotationAppName:            "app1",
		CanaryAnnotationDevelopmentVersion: "2",
		CanaryAnnotationDescription:        "units change",
		CanaryAnnotationProcessUnitsDest:   "5",
		CanaryAnnotationEventName:          "CanaryStepTarget",
		CanaryAnnotationProcessName:        "p1",
		CanaryAnnotationProcessUnitsSource: "2",
		CanaryAnnotationVersionDest:        "2",
		CanaryAnnotationVersionSource:      "1",
	}
	event := newCanaryTargetChangeEvent(&App{
		ObjectMeta: metav1.ObjectMeta{Name: "app1"},
		Spec: AppSpec{
			Canary: CanarySpec{CurrentStep: 10},
			Deployments: []AppDeploymentSpec{
				{Version: 1, RoutingSettings: RoutingSettings{Weight: 30}},
				{Version: 2, RoutingSettings: RoutingSettings{Weight: 70}},
			},
		}},
		"p1", 2, 5,
	)
	require.Equal(t, expectedAnnotations, event.Event.Annotations)
}

func TestAppReconcileOutcome_String(t *testing.T) {
	event := AppReconcileOutcome{
		AppName:         "app1",
		DeploymentCount: 5,
	}
	require.Equal(t, "app app1 5 reconcile success", event.String())
	require.Equal(t, "app app1 5 reconcile fail: [failed to do something]", event.String(fmt.Errorf("failed to do something")))
}

func TestParseAppReconcileOutcome(t *testing.T) {
	msg := "app app1 5 reconcile success"
	outcome, err := ParseAppReconcileOutcome(msg)
	require.Nil(t, err)
	require.Equal(t, AppReconcileOutcome{
		AppName:         "app1",
		DeploymentCount: 5,
	}, *outcome)
}

func TestParseAppReconcileOutcome_Multiple(t *testing.T) {
	tests := []struct {
		msg             string
		expectedApp     string
		expectedVersion int
		expectedErr     string
		expectedString  string
	}{
		{
			msg: "app test 1 reconcile success",

			expectedApp:     "test",
			expectedVersion: 1,
			expectedString:  "app test 1 reconcile success",
			expectedErr:     "",
		},
		{
			msg: "app test34s 1 reconcile success",

			expectedApp:     "test34s",
			expectedVersion: 1,
			expectedString:  "app test34s 1 reconcile success",
			expectedErr:     "",
		},
		{
			msg: "app test34s 2 reconcile success",

			expectedApp:     "test34s",
			expectedVersion: 2,
			expectedString:  "app test34s 2 reconcile success",
			expectedErr:     "",
		},
		{
			msg:         "sdfsdf",
			expectedErr: "unable to parse reconcile reason: input does not match format",
		},
		{
			msg:         "",
			expectedErr: "unable to parse reconcile reason: unexpected EOF",
		},
		{
			msg: "app test 1asdfadfasfasdf reconcile 34rt w",

			expectedApp:     "test",
			expectedVersion: 1,
			expectedString:  "app test 1 reconcile asdfadfasfasdf 34rt w",
			expectedErr:     "unable to parse reconcile reason: expected space in input to match format",
		},
		{
			msg: "app test 1 reconcile asdfadfasfasdf 34rt w",

			expectedApp:     "test",
			expectedVersion: 1,
			expectedString:  "app test 1 reconcile success",
			expectedErr:     "",
		},
	}

	for _, test := range tests {
		got, err := ParseAppReconcileOutcome(test.msg)
		if err != nil {
			require.Equal(t, test.expectedErr, err.Error())
		} else {
			require.Equal(t, test.expectedApp, got.AppName)
			require.Equal(t, test.expectedVersion, got.DeploymentCount)
			require.Equal(t, test.expectedString, got.String())
		}
	}
}

func TestAppType(t *testing.T) {
	at := AppType("NonExistantType")
	tt := []struct {
		name     string
		appType  *AppType
		expected AppType
	}{
		{
			name:     "return specified type",
			appType:  &at,
			expected: at,
		},
		{
			name:     "type is nil",
			appType:  nil,
			expected: DeploymentAppType,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			as := AppSpec{
				Type: tc.appType,
			}
			appType := as.GetType()
			require.Equal(t, tc.expected, appType)
		})
	}
}

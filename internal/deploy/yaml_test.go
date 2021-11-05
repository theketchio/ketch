package deploy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
	"github.com/theketchio/ketch/internal/utils/conversions"
)

func TestGetChangeSetFromYaml(t *testing.T) {
	tests := []struct {
		description string
		yaml        string
		options     *Options
		changeSet   *ChangeSet
		errStr      string
	}{
		{
			description: "success",
			yaml: `version: v1
type: Application
name: test
image: gcr.io/kubernetes/sample-app:latest
framework: myframework
description: a test
builder: heroku/buildpacks:20
buildPacks:
  - test-buildpack
environment:
  - PORT=6666
  - FOO=bar
processes:
  - name: web
    units: 1
  - name: worker
    units: 1
cname:
  dnsName: test.10.10.10.20`,
			options: &Options{
				Timeout:       "1m",
				Wait:          true,
				AppSourcePath: ".",
			},
			changeSet: &ChangeSet{
				appName:              "test",
				yamlStrictDecoding:   true,
				sourcePath:           conversions.StrPtr("."),
				image:                conversions.StrPtr("gcr.io/kubernetes/sample-app:latest"),
				description:          conversions.StrPtr("a test"),
				envs:                 &[]string{"PORT=6666", "FOO=bar"},
				framework:            conversions.StrPtr("myframework"),
				dockerRegistrySecret: nil,
				builder:              conversions.StrPtr("heroku/buildpacks:20"),
				buildPacks:           &[]string{"test-buildpack"},
				cname:                &ketchv1.CnameList{{Name: "test.10.10.10.20", Secure: false}},
				timeout:              conversions.StrPtr("1m"),
				wait:                 conversions.BoolPtr(true),
				processes: &[]ketchv1.ProcessSpec{
					{
						Name:  "web",
						Units: conversions.IntPtr(1),
						Env: []ketchv1.Env{
							{
								Name:  "PORT",
								Value: "6666",
							},
							{
								Name:  "FOO",
								Value: "bar",
							},
						},
					},
					{
						Name:  "worker",
						Units: conversions.IntPtr(1),
						Env: []ketchv1.Env{
							{
								Name:  "PORT",
								Value: "6666",
							},
							{
								Name:  "FOO",
								Value: "bar",
							},
						},
					},
				},
				appVersion: conversions.StrPtr("v1"),
				appType:    conversions.StrPtr("Application"),
			},
		},
		{
			description: "success - defaults",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest`,
			options: &Options{},
			changeSet: &ChangeSet{
				appName:            "test",
				yamlStrictDecoding: true,
				image:              conversions.StrPtr("gcr.io/kubernetes/sample-app:latest"),
				framework:          conversions.StrPtr("myframework"),
				appVersion:         conversions.StrPtr("v1"),
				appType:            conversions.StrPtr("Application"),
				timeout:            conversions.StrPtr(""),
				wait:               conversions.BoolPtr(false),
			},
		},
		{
			description: "validation error - framework",
			yaml: `name: test
image: gcr.io/kubernetes/sample-app:latest`,
			options: &Options{},
			errStr:  "missing required field framework",
		},
		{
			description: "validation error - processes without sourcePath",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest
processes:
  - name: web
    cmd: python app.py`,
			options: &Options{},
			errStr:  "running defined processes require a sourcePath",
		},
		{
			description: "success - use appUnits as process.units when units are not specified",
			yaml: `version: v1
type: Application
name: test
image: gcr.io/kubernetes/sample-app:latest
framework: myframework
description: a test
builder: heroku/buildpacks:20
processes:
  - name: web
    cmd: python app.py
    units: 1
  - name: worker
    cmd: python app.py`,
			options: &Options{
				AppSourcePath: ".",
			},
			changeSet: &ChangeSet{
				appName:            "test",
				yamlStrictDecoding: true,
				sourcePath:         conversions.StrPtr("."),
				image:              conversions.StrPtr("gcr.io/kubernetes/sample-app:latest"),
				description:        conversions.StrPtr("a test"),
				builder:            conversions.StrPtr("heroku/buildpacks:20"),
				framework:          conversions.StrPtr("myframework"),
				timeout:            conversions.StrPtr(""),
				wait:               conversions.BoolPtr(false),
				processes: &[]ketchv1.ProcessSpec{
					{
						Name:  "web",
						Units: conversions.IntPtr(1),
					},
					{
						Name:  "worker",
						Units: conversions.IntPtr(1),
					},
				},
				appVersion: conversions.StrPtr("v1"),
				appType:    conversions.StrPtr("Application"),
			},
		},
		{
			description: "success - no cname",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest
`,
			options: &Options{},
			changeSet: &ChangeSet{
				appName:            "test",
				yamlStrictDecoding: true,
				image:              conversions.StrPtr("gcr.io/kubernetes/sample-app:latest"),
				framework:          conversions.StrPtr("myframework"),
				appVersion:         conversions.StrPtr("v1"),
				appType:            conversions.StrPtr("Application"),
				timeout:            conversions.StrPtr(""),
				wait:               conversions.BoolPtr(false),
			},
		},
		{
			description: "error - malformed envvar",
			yaml: `name: test
framework: myframework
image: gcr.io/kubernetes/sample-app:latest
environment:
  - bad:variable
`,
			options: &Options{},
			errStr:  "env variables should have NAME=VALUE format",
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "*.yaml")
			require.Nil(t, err)
			_, err = file.Write([]byte(tt.yaml))
			require.Nil(t, err)
			defer os.Remove(file.Name())

			cs, err := tt.options.GetChangeSetFromYaml(file.Name())
			if tt.errStr != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.errStr)
			} else {
				require.Nil(t, err)
				require.Equal(t, tt.changeSet, cs)
			}
		})
	}
}

func TestGetApplicationFromKetchApp(t *testing.T) {
	tests := []struct {
		description string
		app         ketchv1.App
		application *Application
	}{
		{
			description: "minimum required fields",
			app: ketchv1.App{
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
				Spec: ketchv1.AppSpec{
					Framework: "myframework",
				},
			},
			application: &Application{
				Type:      conversions.StrPtr(typeApplication),
				Name:      conversions.StrPtr("test"),
				Framework: conversions.StrPtr("myframework"),
			},
		},
		{
			description: "all fields",
			app: ketchv1.App{
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
				Spec: ketchv1.AppSpec{
					Framework:      "myframework",
					Version:        conversions.StrPtr("v1"),
					Description:    "a test",
					Env:            []ketchv1.Env{{Name: "TEST_KEY", Value: "TEST_VALUE"}},
					DockerRegistry: ketchv1.DockerRegistrySpec{SecretName: "a_secret"},
					Builder:        "builder",
					BuildPacks:     []string{"test/buildpack"},
					Deployments: []ketchv1.AppDeploymentSpec{
						{
							Version: ketchv1.DeploymentVersion(1), // not latest deployment
							Image:   "gcr.io/shipa-ci/sample-go-app:not_latest",
						},
						{
							Version: ketchv1.DeploymentVersion(3),
							Image:   "gcr.io/shipa-ci/sample-go-app:latest",
							Processes: []ketchv1.ProcessSpec{
								{Name: "process-1", Units: conversions.IntPtr(1)},
								{Name: "process-2", Units: conversions.IntPtr(2)},
								{Name: "process-3", Units: conversions.IntPtr(1)},
							},
						},
						{
							Version: ketchv1.DeploymentVersion(2), // not latest deployment
							Image:   "gcr.io/shipa-ci/sample-go-app:not_latest",
						},
					},
					Ingress: ketchv1.IngressSpec{Cnames: ketchv1.CnameList{{Name: "test.com"}, {Name: "another.com"}}},
				},
			},
			application: &Application{
				Version:        conversions.StrPtr("v1"),
				Type:           conversions.StrPtr(typeApplication),
				Name:           conversions.StrPtr("test"),
				Image:          conversions.StrPtr("gcr.io/shipa-ci/sample-go-app:latest"),
				Framework:      conversions.StrPtr("myframework"),
				Description:    conversions.StrPtr("a test"),
				Environment:    []string{"TEST_KEY=TEST_VALUE"},
				RegistrySecret: conversions.StrPtr("a_secret"),
				Builder:        conversions.StrPtr("builder"),
				BuildPacks:     []string{"test/buildpack"},
				CName: &CName{
					DNSName: "test.com",
				},
				Processes: []Process{
					{
						Name:  "process-1",
						Units: conversions.IntPtr(1),
					},
					{
						Name:  "process-2",
						Units: conversions.IntPtr(2),
					},
					{
						Name:  "process-3",
						Units: conversions.IntPtr(1),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			res := GetApplicationFromKetchApp(tt.app)
			require.Equal(t, tt.application, res)
		})
	}
}

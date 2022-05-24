package deploy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	ketchv1 "github.com/theketchio/ketch/internal/api/v1beta1"
)

func TestErrors(t *testing.T) {
	require.True(t, isMissing(newMissingError("oops")))
	require.False(t, isValid(newInvalidValueError("oops")))
	require.False(t, isValid(newInvalidUsageError("oops")))
	require.True(t, isMissing(fmt.Errorf("some error %w", newMissingError("oops"))))
	require.False(t, isValid(fmt.Errorf("some error %w", newInvalidValueError("oops"))))
	require.False(t, isValid(fmt.Errorf("some error %w", newInvalidUsageError("oops"))))
}

func TestDeployValidations(t *testing.T) {

	tests := []struct {
		name    string
		cs      *ChangeSet
		app     *ketchv1.App
		want    error
		wantErr string
	}{
		{
			name: "valid changeset spec",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(1),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			want: nil,
		},
		{
			name: "invalid unit",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(0),
				version:            intRef(1),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"units" invalid value units must be 1 or greater`,
		},
		{
			name: "invalid unit",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(0),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"unit-version" invalid value unit-version must be 1 or greater`,
		},
		{
			name: "invalid process",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"unit-process" used improperly unit-process must be used with units flag`,
		},
		{
			name: "invalid volume",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(1),
				process:            stringRef("worker"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"volume-mount-path" used improperly volume-mount-path must be used with volume flag`,
		},
		{
			name: "invalid volume mount",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(1),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "asdf"},
				runAsUser:          func(val int64) *int64 { return &val }(0),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"volume-mount-options" used improperly readOnly must be either true or false`,
		},
		{
			name: "invalid run as user",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(1),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(-1),
				fsGroup:            func(val int64) *int64 { return &val }(1001),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"run-as-user" invalid value run-as-user must be 0 or greater`,
		},
		{
			name: "invalid fs-group",
			cs: &ChangeSet{
				image:              stringRef("docker.io/shipasoftware/bulletinboard:1.0"),
				units:              intRef(1),
				version:            intRef(1),
				process:            stringRef("worker"),
				volume:             stringRef("pvc-1"),
				volumeMountPath:    stringRef("/opt/pkg"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				runAsUser:          func(val int64) *int64 { return &val }(1),
				fsGroup:            func(val int64) *int64 { return &val }(0),
			},
			app: &ketchv1.App{
				Spec: ketchv1.AppSpec{
					Deployments: []ketchv1.AppDeploymentSpec{},
				},
			},
			wantErr: `"fs-group" invalid value fs-group must be 1 or greater`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeploy(tt.cs, tt.app)
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.Equal(t, tt.want, err)
		})
	}
}

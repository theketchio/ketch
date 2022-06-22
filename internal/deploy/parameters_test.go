package deploy

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

func intRef(i int) *int {
	return &i
}

func stringRef(str string) *string {
	return &str
}

func TestChangeSet_getStepWeight(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    uint8
		wantErr string
	}{
		{
			name: "happy path",
			set:  ChangeSet{steps: intRef(4)},
			want: 25,
		},
		{
			name: "happy path",
			set:  ChangeSet{steps: intRef(5)},
			want: 20,
		},
		{
			name:    "error - no steps",
			set:     ChangeSet{},
			wantErr: `"steps" missing`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight, err := tt.set.getStepWeight()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, weight)
		})
	}
}

func TestChangeSet_getVolumeName(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    string
		wantErr string
	}{
		{
			name: "valid volume name",
			set:  ChangeSet{volume: stringRef("pvc-1")},
			want: "pvc-1",
		},
		{
			name:    "invalid volume name",
			set:     ChangeSet{},
			wantErr: `"volume" missing`,
		},
		{
			name:    "invalid volume name",
			set:     ChangeSet{volume: stringRef("aaa/bbb")},
			wantErr: `"volume" invalid value volume: aaa/bbb. A volume name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc')`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeName, err := tt.set.getVolumeName()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.want, volumeName)
		})
	}
}

func TestChangeSet_getVolumes(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    []v1.Volume
		wantErr string
	}{
		{
			name:    "volume name not included",
			set:     ChangeSet{},
			wantErr: `"volume" missing`,
		},
		{
			name: "no volume source",
			set:  ChangeSet{volume: stringRef("pvc-1")},
			action: func(set *ChangeSet) {
			},
			want: []v1.Volume{{
				Name: "pvc-1",
			}},
			wantErr: "false",
		},
		{
			name: "incorrect volume source",
			set:  ChangeSet{volume: stringRef("pvc-1")},
			action: func(set *ChangeSet) {
			},
			want: []v1.Volume{{
				Name:         "pvc-1",
				VolumeSource: v1.VolumeSource{ConfigMap: &v1.ConfigMapVolumeSource{}},
			}},
			wantErr: "false",
		},
		{
			name: "valid volume",
			set:  ChangeSet{volume: stringRef("pvc-1")},
			action: func(set *ChangeSet) {
			},
			want: []v1.Volume{{
				Name: "pvc-1",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "pvc-1",
					},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumes, err := tt.set.getVolumes()
			if err != nil {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.Nil(t, err)

			if len(tt.wantErr) > 0 {
				isIdenticalType := reflect.DeepEqual(tt.want, volumes)
				require.Equal(t, tt.wantErr, strconv.FormatBool(isIdenticalType))
				return
			}

			require.Equal(t, tt.want, volumes)
		})
	}
}

func TestChangeSet_getVolumeMounts(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    []v1.VolumeMount
		wantErr string
	}{
		{
			name: "no volume mount needed",
			set:  ChangeSet{},
			want: nil,
		},
		{
			name:    "volume mount path set without volume name",
			set:     ChangeSet{volumeMountPath: stringRef("/opt/pkg")},
			wantErr: `"volume-mount-path" used improperly volume-mount-path must be used with volume flag`,
		},
		{
			name:    "volume mount path set without volume name",
			set:     ChangeSet{volumeMountOptions: &map[string]string{"readOnly": "true"}},
			wantErr: `"volume-mount-options" used improperly volume-mount-options must be used with volume flag`,
		},
		{
			name:    "volume mount path set with invalid readOnly value",
			set:     ChangeSet{volume: stringRef("pvc-1"), volumeMountOptions: &map[string]string{"readOnly": "asdf"}},
			wantErr: `"volume-mount-options" used improperly readOnly must be either true or false`,
		},
		{
			name: "valid volume mount with options and path",
			set: ChangeSet{
				volume:             stringRef("pvc-1"),
				volumeMountOptions: &map[string]string{"readOnly": "true"},
				volumeMountPath:    stringRef("/opt/pkg"),
			},
			want: []v1.VolumeMount{{
				Name:      "pvc-1",
				MountPath: "/opt/pkg",
				ReadOnly:  true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeMounts, err := tt.set.getVolumeMounts()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, volumeMounts)
		})
	}

}

func TestChangeSet_getFSGroup(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    int64
		wantErr string
	}{
		{
			name: "no fs-group set",
			set:  ChangeSet{},
			want: 0,
		},
		{
			name:    "invalid fs-group value",
			set:     ChangeSet{fsGroup: func(val int64) *int64 { return &val }(-1)},
			wantErr: `"fs-group" invalid value fs-group must be 1 or greater`,
		},
		{
			name: "valid fs-group value",
			set:  ChangeSet{fsGroup: func(val int64) *int64 { return &val }(1000)},
			want: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsGroup, err := tt.set.getFSGroup()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, fsGroup)
		})
	}
}

func TestChangeSet_getRunAsUser(t *testing.T) {

	tests := []struct {
		name    string
		set     ChangeSet
		action  func(set *ChangeSet)
		want    int64
		wantErr string
	}{
		{
			name: "no run-as-user set",
			set:  ChangeSet{},
			want: 0,
		},
		{
			name:    "invalid run-as-user value",
			set:     ChangeSet{runAsUser: func(val int64) *int64 { return &val }(-1)},
			wantErr: `"run-as-user" invalid value run-as-user must be 0 or greater`,
		},
		{
			name: "valid run-as-user value",
			set:  ChangeSet{runAsUser: func(val int64) *int64 { return &val }(0)},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runAsUser, err := tt.set.getRunAsUser()
			if len(tt.wantErr) > 0 {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErr, err.Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, runAsUser)
		})
	}
}

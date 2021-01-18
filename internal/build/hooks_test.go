package build

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

		"github.com/stretchr/testify/require"
)

func Test_getHooks(t *testing.T) {
	const buildHookYaml = `hooks:
  restart:
    before:
      - python manage.py local_file
    after:
      - python manage.py clear_cache
  build:
    - python manage.py collectstatic --noinput
    - python manage.py compress
`

	tt := []struct {
		name string
		setupfn func(wd string)(searchPaths []string)
		hooks []string
		wantErr bool
	}{
		{
			name: "yaml happy path",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				shipaFile := path.Join(sp, "shipa.yaml")
				err = ioutil.WriteFile(shipaFile, []byte(buildHookYaml), 0644)
				require.Nil(t, err)

				return append(searchPaths, "path1")
			},
			hooks: []string{
				"python manage.py collectstatic --noinput",
				"python manage.py compress",
			},
		},
		{
			name: "yml happy path",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				shipaFile := path.Join(sp, "shipa.yml")
				err = ioutil.WriteFile(shipaFile, []byte(buildHookYaml), 0644)
				require.Nil(t, err)

				return append(searchPaths, "path1")
			},
			hooks: []string{
				"python manage.py collectstatic --noinput",
				"python manage.py compress",
			},
		},
		{
			name: "yml multiple paths",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				shipaFile := path.Join(sp, "shipa.yml")
				err = ioutil.WriteFile(shipaFile, []byte(buildHookYaml), 0644)
				require.Nil(t, err)
				emptyPath := path.Join(td, "empty")
				err = os.Mkdir(emptyPath, 0755)
				require.Nil(t, err)
				return append(searchPaths, "path1", "empty")
			},
			hooks: []string{
				"python manage.py collectstatic --noinput",
				"python manage.py compress",
			},
		},
		{
			name: "no yaml exists",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				emptyPath := path.Join(td, "empty")
				err = os.Mkdir(emptyPath, 0755)
				require.Nil(t, err)
				return append(searchPaths, "path1", "empty")
			},
		},
		{
			name: "yaml sad permissions",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				shipaFile := path.Join(sp, "shipa.yaml")
				err = ioutil.WriteFile(shipaFile, []byte(buildHookYaml), 0200)
				require.Nil(t, err)

				return append(searchPaths, "path1")
			},
			wantErr: true,
		},
		{
			name: "format of woe",
			setupfn: func(td string)(searchPaths []string){
				sp := path.Join(td, "path1")
				err := os.Mkdir(sp, 0755)
				require.Nil(t, err)
				shipaFile := path.Join(sp, "shipa.yaml")
				err = ioutil.WriteFile(shipaFile, []byte("garbage"), 0644)
				require.Nil(t, err)

				return append(searchPaths, "path1")
			},
			wantErr: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T){
			td := t.TempDir()
			hooks, err := getHooks(td, tc.setupfn(td))
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.ElementsMatch(t, hooks, tc.hooks )
		})
	}
}
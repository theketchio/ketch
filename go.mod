module github.com/shipa-corp/ketch

go 1.17

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/buildpacks/pack v0.15.1
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/google/go-containerregistry v0.1.4
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/thediveo/enumflag v0.10.1
	golang.org/x/mod v0.4.2
	gopkg.in/src-d/go-git.v4 v4.13.1
	gotest.tools v2.2.0+incompatible
	helm.sh/helm/v3 v3.6.3
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/cli-runtime v0.21.0
	k8s.io/client-go v0.21.3
	k8s.io/kubectl v0.21.0
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/controller-runtime v0.9.5
	sigs.k8s.io/yaml v1.2.0
)

require (
	github.com/containerd/containerd v1.4.11 // indirect
	github.com/opencontainers/runc v1.0.0-rc95 // indirect
)

package deploy

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/client-go/kubernetes"

	"github.com/shipa-corp/ketch/internal/errors"
)



type imageConfigRequest struct {
	imageName       string
	secretName      string
	secretNamespace string
	imageFn RemoteImageFn
	client kubernetes.Interface
}

type ImageConfiger interface {
	ConfigFile()(*registryv1.ConfigFile, error )
}

type RemoteImageFn func(ref name.Reference, options ...remote.Option) (ImageConfiger, error)

func getImageConfig(ctx context.Context, args imageConfigRequest) (*registryv1.ConfigFile, error) {
	ref, err := name.ParseReference(args.imageName)
	if err != nil {
		return nil, errors.Wrap(err,  "failed to parse reference for image %q", args.imageName)
	}
	var options []remote.Option
	if args.secretName != "" {
		keychainOpts := k8schain.Options{
			Namespace: args.secretNamespace,
			ImagePullSecrets: []string{args.secretName},
		}
		keychain, err := k8schain.New(ctx, args.client, keychainOpts)
		if err != nil {
			return nil, errors.Wrap(err, "could not get keychain")
		}
		options = append(options, remote.WithAuthFromKeychain(keychain))
	}
	img, err := args.imageFn(ref, options...)
	if err != nil {
		return nil, errors.Wrap(err, "could not get config for image %q", args.imageName)
	}
	return img.ConfigFile()
}

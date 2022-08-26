package chart

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"

	ketchv1 "github.com/theketchio/ketch/internal/api/v1"
)

// sslCertificateManager describes who is responsible for getting an SSL certificate.
type sslCertificateManager string

const (
	user        sslCertificateManager = "user"
	certManager sslCertificateManager = "cert-manager"
)

// httpsEndpoint holds configuration of a https endpoint.
type httpsEndpoint struct {
	// UniqueName is a unique and deterministic identifier that can be used to name a k8s resource for this https endpoint.
	UniqueName string `json:"uniqueName"`
	// Cname of this endpoint.
	Cname string `json:"cname"`
	// SecretName is a name of a k8s Secret containing an SSL certificate for the cname.
	// If the ManagedBy field is "cert-manager",
	// then cert-manager will use this name to store a Lets Encrypt certificate.
	// If the ManagedBy field is "user",
	// the secret must already contain a valid SSL certificate.
	SecretName string `json:"secretName"`
	// ManagedBy specifies who is responsible for getting an SSL certificate and storing it in the secret.
	ManagedBy sslCertificateManager `json:"managedBy"`
}

// Ingress contains information about entrypoints of an application.
// istio, traefik and nginx templates use "ingress" to render Kubernetes Ingress objects.
type ingress struct {

	// Https is a list of http entrypoints.
	Http []string `json:"http"`

	// Https is a list of https entrypoints.
	Https []httpsEndpoint `json:"https"`
}

func newIngress(app ketchv1.App, ingressController ketchv1.IngressControllerSpec) (*ingress, error) {

	// CNAMEs contain only:
	// A to Z ; upper case characters
	// a to z ; lower case characters
	// 0 to 9 ; numeric characters 0 to 9
	// - ; dash
	// Max length of a cname is 63 characters.
	// so here we are transforming each CNAME in a way that we can use them to name k8s resources.
	regex := regexp.MustCompile("[^a-z0-9]+")

	var http []string
	var https []httpsEndpoint

	for _, cname := range app.Spec.Ingress.Cnames {
		if !cname.Secure {
			http = append(http, cname.Name)
			continue
		}

		if len(ingressController.ClusterIssuer) == 0 {
			return nil, errors.New("secure cnames require a Ingress.ClusterIssuer to be specified")
		}

		strippedCname := regex.ReplaceAllString(cname.Name, "-")
		if len(cname.SecretName) > 0 {
			https = append(https, httpsEndpoint{
				Cname:      cname.Name,
				SecretName: cname.SecretName,
				UniqueName: fmt.Sprintf("%s-https-%s", app.Name, strippedCname),
				ManagedBy:  user,
			})
		} else {
			https = append(https, httpsEndpoint{
				Cname:      cname.Name,
				SecretName: fmt.Sprintf("%s-cname-%s", app.Name, strippedCname),
				UniqueName: fmt.Sprintf("%s-https-%s", app.Name, strippedCname),
				ManagedBy:  certManager,
			})
		}
	}
	defaultCname := app.DefaultCname()
	if defaultCname != nil {
		http = append(http, *defaultCname)
	}
	return &ingress{
		Http:  http,
		Https: https,
	}, nil
}

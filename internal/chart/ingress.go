package chart

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
)

// httpsEndpoint holds cname and its corresponding secret name with SSL certificates.
type httpsEndpoint struct {
	// UniqueName is a unique and deterministic identifier that can be used to name a k8s resource for this https endpoint.
	UniqueName string `json:"uniqueName"`
	// Cname of this endpoint.
	Cname string `json:"cname"`
	// SecretName is a name of a Kubernetes Secret to store SSL certificate for the cname.
	SecretName string `json:"secretName"`
}

// Ingress contains information about entrypoints of an application.
// istio, traefik and nginx templates use "ingress" to render Kubernetes Ingress objects.
type ingress struct {

	// Https is a list of http entrypoints.
	Http []string `json:"http"`

	// Https is a list of https entrypoints.
	Https []httpsEndpoint `json:"https"`
}

func newIngress(app ketchv1.App, framework ketchv1.Framework) (*ingress, error) {
	var http []string
	var https []string

	for _, cname := range app.Spec.Ingress.Cnames {
		if cname.Secure {
			if len(framework.Spec.IngressController.ClusterIssuer) == 0 {
				return nil, errors.New("secure cnames require a framework.Ingress.ClusterIssuer to be specified")
			}
			https = append(https, cname.Name)
		} else {
			http = append(http, cname.Name)
		}
	}

	// CNAMEs contain only:
	// A to Z ; upper case characters
	// a to z ; lower case characters
	// 0 to 9 ; numeric characters 0 to 9
	// - ; dash
	// Max length of a cname is 63 characters.
	// so here we are transforming each CNAME in a way that we can use them to name k8s resources.
	regex := regexp.MustCompile("[^a-z0-9]+")

	var httpsEndpoints []httpsEndpoint
	for _, cname := range https {
		strippedCname := regex.ReplaceAllString(cname, "-")
		httpsEndpoints = append(httpsEndpoints, httpsEndpoint{
			Cname:      cname,
			SecretName: fmt.Sprintf("%s-cname-%s", app.Name, strippedCname),
			UniqueName: fmt.Sprintf("%s-https-%s", app.Name, strippedCname),
		})
	}
	defaultCname := app.DefaultCname(&framework)
	if defaultCname != nil {
		http = append(http, *defaultCname)
	}
	return &ingress{
		Http:  http,
		Https: httpsEndpoints,
	}, nil
}

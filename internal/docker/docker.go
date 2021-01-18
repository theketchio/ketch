// Package docker wraps a docker client and is used to build and push images
package docker

import (
	"context"
	"github.com/docker/docker/client"
)
type Client struct {
	docker *client.Client
}



func New(ctx context.Context)(*Client,error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &Client{
		docker: cli,
	}, nil
}

func (c *Client) Build(ctx context.Context) error {
	c.docker.Im
	return nil
}
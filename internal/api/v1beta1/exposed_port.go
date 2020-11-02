package v1beta1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ExposedPort represents a port exposed by a docker image.
// Native format is "port/PROTOCOL" string, we parse it and keep it as ExposedPort.
type ExposedPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

func (p ExposedPort) ToDockerFormat() string {
	if p.Port == 0 && p.Protocol == "" {
		return ""
	}
	return fmt.Sprintf("%d/%s", p.Port, p.Protocol)
}

// NewExposedPort parses the port exposed from a container. The port should have "port/PROTOCOL" format.
func NewExposedPort(port string) (*ExposedPort, error) {
	parts := strings.SplitN(port, "/", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid port: " + port)
	}
	portInt, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, errors.New("invalid port: " + port)
	}
	return &ExposedPort{
		Port:     portInt,
		Protocol: strings.ToUpper(parts[1]),
	}, nil
}

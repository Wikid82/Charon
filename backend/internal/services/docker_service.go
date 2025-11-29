package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type DockerPort struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
	Type        string `json:"type"`
}

type DockerContainer struct {
	ID      string       `json:"id"`
	Names   []string     `json:"names"`
	Image   string       `json:"image"`
	State   string       `json:"state"`
	Status  string       `json:"status"`
	Network string       `json:"network"`
	IP      string       `json:"ip"`
	Ports   []DockerPort `json:"ports"`
}

type DockerService struct {
	client *client.Client
}

func NewDockerService() (*DockerService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerService{client: cli}, nil
}

func (s *DockerService) ListContainers(ctx context.Context, host string) ([]DockerContainer, error) {
	var cli *client.Client
	var err error

	if host == "" || host == "local" {
		cli = s.client
	} else {
		cli, err = client.NewClientWithOpts(client.WithHost(host), client.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("failed to create remote client: %w", err)
		}
		defer func() { _ = cli.Close() }()
	}

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []DockerContainer
	for _, c := range containers {
		// Get the first network's IP address if available
		networkName := ""
		ipAddress := ""
		if c.NetworkSettings != nil && len(c.NetworkSettings.Networks) > 0 {
			for name, net := range c.NetworkSettings.Networks {
				networkName = name
				ipAddress = net.IPAddress
				break // Just take the first one for now
			}
		}

		// Clean up names (remove leading slash)
		names := make([]string, len(c.Names))
		for i, name := range c.Names {
			names[i] = strings.TrimPrefix(name, "/")
		}

		// Map ports
		var ports []DockerPort
		for _, p := range c.Ports {
			ports = append(ports, DockerPort{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		result = append(result, DockerContainer{
			ID:      c.ID[:12], // Short ID
			Names:   names,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Network: networkName,
			IP:      ipAddress,
			Ports:   ports,
		})
	}

	return result, nil
}

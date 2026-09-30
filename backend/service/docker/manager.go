package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ContainerInfo 扫描到的容器端口信息
type ContainerInfo struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Image string        `json:"image"`
	State string        `json:"state"` // running/exited/...
	Ports []PortMapping `json:"ports"`
}

// PortMapping 端口映射：宿主机端口 → 容器端口
type PortMapping struct {
	HostIP      string `json:"host_ip"`
	HostPort    int    `json:"host_port"`
	ContainerIP string `json:"container_ip"`
	// ContainerPort 容器内部端口。published 端口容器 IP 为空时用 127.0.0.1+HostPort 访问
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp/udp
}

type containerSummary struct {
	ID    string `json:"Id"`
	Names []string
	Image string
	State string
	Ports []struct {
		IP          string
		PrivatePort int
		PublicPort  int
		Type        string
	}
}

// client 通过 unix socket 直连 Docker API（stdlib 实现，避免引入 docker/docker 依赖）
type client struct {
	http *http.Client
}

func newClient() *client {
	return &client{
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", "/var/run/docker.sock")
				},
			},
			Timeout: 5 * time.Second,
		},
	}
}

func (c *client) get(path string, out interface{}) error {
	resp, err := c.http.Get("http://localhost" + path)
	if err != nil {
		return fmt.Errorf("Docker API 不可达（检查 /var/run/docker.sock 权限）: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Docker API 返回 %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ListContainers 列出运行中容器的端口映射，生成推荐穿透规则所需的信息。
// 仅返回有端口映射的容器。
func ListContainers() ([]ContainerInfo, error) {
	c := newClient()
	var summaries []containerSummary
	// 只取运行中的容器
	if err := c.get("/containers/json?filters=%7B%22status%22%3A%5B%22running%22%5D%7D", &summaries); err != nil {
		return nil, err
	}

	result := make([]ContainerInfo, 0, len(summaries))
	for _, s := range summaries {
		info := ContainerInfo{
			ID:    s.ID[:12],
			Name:  strings.TrimPrefix(firstOr(s.Names, ""), "/"),
			Image: s.Image,
			State: s.State,
			Ports: []PortMapping{},
		}
		for _, p := range s.Ports {
			if p.PublicPort == 0 && p.IP == "" {
				// exposed 但未 published，跳过（不可从宿主机直接访问）
				continue
			}
			info.Ports = append(info.Ports, PortMapping{
				HostIP:        p.IP,
				HostPort:      p.PublicPort,
				ContainerPort: p.PrivatePort,
				Protocol:      p.Type,
			})
		}
		if len(info.Ports) == 0 {
			continue
		}
		result = append(result, info)
	}
	return result, nil
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}

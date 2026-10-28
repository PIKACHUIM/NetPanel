package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ContainerInfo 扫描到的容器端口信息
type ContainerInfo struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Image   string        `json:"image"`
	State   string        `json:"state"`  // running/exited/...
	Status  string        `json:"status"` // 人类可读状态，如 "Up 2 hours"
	Created int64         `json:"created"`
	Ports   []PortMapping `json:"ports"`
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
	ID      string `json:"Id"`
	Names   []string
	Image   string
	State   string
	Status  string
	Created int64
	Ports   []struct {
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
	return list(false)
}

// ListAllContainers 列出全部容器（含已停止），供本地容器管理页使用。
func ListAllContainers() ([]ContainerInfo, error) {
	return list(true)
}

func list(all bool) ([]ContainerInfo, error) {
	c := newClient()
	path := "/containers/json"
	if all {
		path += "?all=true"
	} else {
		path += "?filters=%7B%22status%22%3A%5B%22running%22%5D%7D"
	}
	var summaries []containerSummary
	if err := c.get(path, &summaries); err != nil {
		return nil, err
	}

	result := make([]ContainerInfo, 0, len(summaries))
	for _, s := range summaries {
		ports := make([]PortMapping, 0, len(s.Ports))
		for _, p := range s.Ports {
			if p.PublicPort == 0 && p.IP == "" {
				// exposed 但未 published，不可从宿主机直接访问
				continue
			}
			ports = append(ports, PortMapping{
				HostIP:        p.IP,
				HostPort:      p.PublicPort,
				ContainerPort: p.PrivatePort,
				Protocol:      p.Type,
			})
		}
		// 推荐穿透规则只关心有端口映射的运行中容器；容器管理页需要完整列表。
		if !all && len(ports) == 0 {
			continue
		}
		info := ContainerInfo{
			ID:      shortID(s.ID),
			Name:    strings.TrimPrefix(firstOr(s.Names, ""), "/"),
			Image:   s.Image,
			State:   s.State,
			Status:  s.Status,
			Created: s.Created,
			Ports:   ports,
		}
		result = append(result, info)
	}
	return result, nil
}

var containerIDRe = regexp.MustCompile(`^[0-9a-zA-Z][0-9a-zA-Z_.-]{0,127}$`)

// Action 对容器执行 start / stop / restart。
// id 只允许 Docker 容器名或短 ID 的合法字符，避免拼接到 URL 造成路径注入。
func Action(id, op string) error {
	switch op {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("不支持的操作: %s", op)
	}
	if !containerIDRe.MatchString(id) {
		return fmt.Errorf("非法的容器 ID")
	}
	return newClient().post("/containers/" + id + "/" + op)
}

// post 向 Docker API 发起无请求体的 POST，成功状态为 204/304。
func (c *client) post(path string) error {
	req, err := http.NewRequest(http.MethodPost, "http://localhost"+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Docker API 不可达（检查 /var/run/docker.sock 权限）: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		return fmt.Errorf("Docker API 返回 %d", resp.StatusCode)
	}
	return nil
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func firstOr(s []string, def string) string {
	if len(s) > 0 {
		return s[0]
	}
	return def
}

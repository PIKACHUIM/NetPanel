package model

import "time"

// FrpMasterNode frpc 多节点 Master：由面板（Master）统一管理的远程 frpc 节点。
//
// 节点以独立进程运行，仅通过控制通道（心跳 / 状态上报）与 Master 保持连接；
// Master 负责节点注册、配置下发（frpc.toml）与状态聚合。
// 规划：#50（frpc 多节点 Master 模式）、#84（多线路池总方案 M3）。
type FrpMasterNode struct {
	BaseModel
	Name   string `gorm:"size:100;not null" json:"name"`
	Region string `gorm:"size:50" json:"region"` // 就近优先用；空 = 不限

	// NodeTokenHash 节点 token 的 SHA-256（hex）。明文仅在创建接口返回一次，
	// 控制通道（心跳/状态上报/取配置）凭它认证。
	NodeTokenHash string `gorm:"size:64" json:"-"`

	// 该节点 frpc 连接的目标 frps 服务端（Master 据此生成 frpc.toml）。
	ServerAddr string `gorm:"size:255" json:"server_addr"`
	ServerPort int    `gorm:"default:7000" json:"server_port"`
	FrpsToken  string `gorm:"size:255" json:"-"` // frps 认证 token，不回显

	// LastSeen 最近一次心跳时间（状态派生依据）。
	LastSeen time.Time `json:"last_seen"`
	// LastTunnels 最近一次上报的隧道列表 JSON（节点状态聚合，M4 解析展示）。
	LastTunnels string `gorm:"type:text" json:"-"`
	Remark      string `gorm:"size:500" json:"remark"`

	// Status 派生状态（online/offline），按 LastSeen 计算，不入库。
	Status string `gorm:"-" json:"status"`
}

import React, { useEffect, useState, useCallback, useRef } from 'react'
import {
  Button, Space, Typography, Tag, Spin, Card, Tooltip, Badge, Descriptions, Divider, Empty,
} from 'antd'
import {
  SyncOutlined, ApartmentOutlined, FullscreenOutlined,
  CheckCircleOutlined, CloseCircleOutlined,
} from '@ant-design/icons'
import { meshNodeApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text, Title } = Typography

// 节点颜色映射
const NODE_COLORS = {
  local: '#1677ff',
  online: '#52c41a',
  offline: '#d9d9d9',
}

// 边类型颜色
const EDGE_COLORS: Record<string, string> = {
  frp: '#fa8c16',
  nps: '#722ed1',
  easytier: '#13c2c2',
}

// 边方向标签
const DIRECTION_LABELS: Record<string, string> = {
  unidirectional: '→ 单向',
  bidirectional: '↔ 双向',
  p2p: '⟷ P2P',
}

interface TopologyNode {
  id: number
  name: string
  is_local: boolean
  is_online: boolean
  node_ip: string
  latency?: number
  tunnels?: any
}

interface TopologyEdge {
  source: number
  target: number
  tunnel_type: string
  tunnel_name: string
  direction: string
  source_label: string
  target_label: string
}

const MeshTopology: React.FC = () => {
  const tableStyle = useTableStyle()
  const [loading, setLoading] = useState(false)
  const [nodes, setNodes] = useState<TopologyNode[]>([])
  const [edges, setEdges] = useState<TopologyEdge[]>([])
  const [selectedNode, setSelectedNode] = useState<TopologyNode | null>(null)
  const [hoveredNode, setHoveredNode] = useState<number | null>(null)
  const canvasRef = useRef<HTMLDivElement>(null)

  const fetchTopology = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await meshNodeApi.getTopology()
      const data = res.data || {}
      setNodes(data.nodes || [])
      setEdges(data.edges || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchTopology() }, [fetchTopology])

  // 计算节点位置（圆形布局）
  const getNodePositions = useCallback(() => {
    const positions: Record<number, { x: number; y: number }> = {}
    const centerX = 400
    const centerY = 300
    const radius = Math.min(200, 80 + nodes.length * 30)

    if (nodes.length === 0) return positions

    // 本机节点放中心
    const localNode = nodes.find(n => n.is_local)
    if (localNode) {
      positions[localNode.id] = { x: centerX, y: centerY }
    }

    // 其他节点围绕中心
    const otherNodes = nodes.filter(n => !n.is_local)
    otherNodes.forEach((node, i) => {
      const angle = (2 * Math.PI * i) / otherNodes.length - Math.PI / 2
      positions[node.id] = {
        x: centerX + radius * Math.cos(angle),
        y: centerY + radius * Math.sin(angle),
      }
    })

    return positions
  }, [nodes])

  const positions = getNodePositions()

  // 获取节点相关的边
  const getNodeEdges = (nodeId: number) => {
    return edges.filter(e => e.source === nodeId || e.target === nodeId)
  }

  // 渲染SVG边
  const renderEdges = () => {
    return edges.map((edge, i) => {
      const from = positions[edge.source]
      const to = positions[edge.target]
      if (!from || !to) return null

      const color = EDGE_COLORS[edge.tunnel_type] || '#999'
      const isHighlighted = hoveredNode !== null && (edge.source === hoveredNode || edge.target === hoveredNode)
      const opacity = hoveredNode !== null ? (isHighlighted ? 1 : 0.15) : 0.6

      // 计算中点（用于标签）
      const midX = (from.x + to.x) / 2
      const midY = (from.y + to.y) / 2

      // 箭头标记
      const markerId = `arrow-${i}`
      const isDashed = edge.direction === 'p2p'

      return (
        <g key={i}>
          <defs>
            <marker id={markerId} markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill={color} opacity={opacity} />
            </marker>
          </defs>
          <line
            x1={from.x} y1={from.y} x2={to.x} y2={to.y}
            stroke={color}
            strokeWidth={isHighlighted ? 3 : 2}
            strokeDasharray={isDashed ? '8,4' : undefined}
            opacity={opacity}
            markerEnd={edge.direction === 'unidirectional' ? `url(#${markerId})` : undefined}
          />
          {/* 边标签 */}
          {isHighlighted && (
            <g>
              <rect
                x={midX - 40} y={midY - 12} width={80} height={24}
                rx={4} fill="rgba(0,0,0,0.75)"
              />
              <text x={midX} y={midY + 4} textAnchor="middle" fill="#fff" fontSize={11}>
                {edge.tunnel_type.toUpperCase()} {DIRECTION_LABELS[edge.direction] || ''}
              </text>
            </g>
          )}
        </g>
      )
    })
  }

  // 渲染SVG节点
  const renderNodes = () => {
    return nodes.map(node => {
      const pos = positions[node.id]
      if (!pos) return null

      const isSelected = selectedNode?.id === node.id
      const isHovered = hoveredNode === node.id
      const color = node.is_local ? NODE_COLORS.local : node.is_online ? NODE_COLORS.online : NODE_COLORS.offline
      const nodeEdges = getNodeEdges(node.id)
      const dimmed = hoveredNode !== null && hoveredNode !== node.id && !edges.some(e =>
        (e.source === hoveredNode && e.target === node.id) || (e.target === hoveredNode && e.source === node.id)
      )

      return (
        <g
          key={node.id}
          style={{ cursor: 'pointer', opacity: dimmed ? 0.2 : 1, transition: 'opacity 0.2s' }}
          onClick={() => setSelectedNode(node)}
          onMouseEnter={() => setHoveredNode(node.id)}
          onMouseLeave={() => setHoveredNode(null)}
        >
          {/* 外圈光晕 */}
          {(isSelected || isHovered) && (
            <circle cx={pos.x} cy={pos.y} r={38} fill="none" stroke={color} strokeWidth={2} opacity={0.3} />
          )}
          {/* 节点圆 */}
          <circle
            cx={pos.x} cy={pos.y} r={30}
            fill={color}
            opacity={0.9}
            stroke={isSelected ? '#fff' : 'none'}
            strokeWidth={isSelected ? 3 : 0}
          />
          {/* 节点名称 */}
          <text x={pos.x} y={pos.y - 2} textAnchor="middle" fill="#fff" fontSize={12} fontWeight={600}>
            {node.name.length > 6 ? node.name.substring(0, 6) + '..' : node.name}
          </text>
          {/* 隧道数量 */}
          <text x={pos.x} y={pos.y + 12} textAnchor="middle" fill="rgba(255,255,255,0.7)" fontSize={10}>
            {nodeEdges.length}条连接
          </text>
          {/* 在线状态指示器 */}
          <circle
            cx={pos.x + 22} cy={pos.y - 22} r={6}
            fill={node.is_online ? '#52c41a' : '#ff4d4f'}
            stroke="#fff" strokeWidth={2}
          />
        </g>
      )
    })
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={4} style={{ margin: 0 }}>
          <ApartmentOutlined style={{ marginRight: 8 }} />
          组网节点布局
        </Title>
        <Space>
          <Button icon={<SyncOutlined />} onClick={fetchTopology} loading={loading}>刷新</Button>
        </Space>
      </div>

      <div style={{ display: 'flex', gap: 16 }}>
        {/* 拓扑图区域 */}
        <Card
          style={{ flex: 1, minHeight: 600 }}
          bodyStyle={{ padding: 0, position: 'relative' }}
        >
          {loading ? (
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 600 }}>
              <Spin size="large" />
            </div>
          ) : nodes.length === 0 ? (
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 600 }}>
              <Empty description="暂无节点数据，请先添加组网节点" />
            </div>
          ) : (
            <div ref={canvasRef}>
              <svg width="100%" height="600" viewBox="0 0 800 600">
                {/* 背景网格 */}
                <defs>
                  <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
                    <path d="M 40 0 L 0 0 0 40" fill="none" stroke="rgba(0,0,0,0.04)" strokeWidth="1" />
                  </pattern>
                </defs>
                <rect width="800" height="600" fill="url(#grid)" />

                {/* 边 */}
                {renderEdges()}

                {/* 节点 */}
                {renderNodes()}
              </svg>

              {/* 图例 */}
              <div style={{
                position: 'absolute', bottom: 16, left: 16,
                background: 'rgba(255,255,255,0.95)', borderRadius: 8, padding: '8px 12px',
                boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
              }}>
                <Text strong style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>图例</Text>
                <Space size={12} wrap>
                  <Space size={4}><div style={{ width: 12, height: 12, borderRadius: '50%', background: NODE_COLORS.local }} /><Text style={{ fontSize: 11 }}>本机</Text></Space>
                  <Space size={4}><div style={{ width: 12, height: 12, borderRadius: '50%', background: NODE_COLORS.online }} /><Text style={{ fontSize: 11 }}>在线</Text></Space>
                  <Space size={4}><div style={{ width: 12, height: 12, borderRadius: '50%', background: NODE_COLORS.offline }} /><Text style={{ fontSize: 11 }}>离线</Text></Space>
                  <Divider type="vertical" />
                  <Space size={4}><div style={{ width: 20, height: 3, background: EDGE_COLORS.frp }} /><Text style={{ fontSize: 11 }}>FRP</Text></Space>
                  <Space size={4}><div style={{ width: 20, height: 3, background: EDGE_COLORS.nps }} /><Text style={{ fontSize: 11 }}>NPS</Text></Space>
                  <Space size={4}><div style={{ width: 20, height: 3, background: EDGE_COLORS.easytier, borderTop: '2px dashed #13c2c2' }} /><Text style={{ fontSize: 11 }}>EasyTier</Text></Space>
                  <Divider type="vertical" />
                  <Text style={{ fontSize: 11 }}>→ 单向 | ↔ 双向 | ⟷ P2P</Text>
                </Space>
              </div>
            </div>
          )}
        </Card>

        {/* 节点详情面板 */}
        <Card
          title={selectedNode ? `${selectedNode.name} 详情` : '节点详情'}
          style={{ width: 320, flexShrink: 0 }}
          size="small"
        >
          {selectedNode ? (
            <div>
              <Descriptions column={1} size="small" style={{ marginBottom: 12 }}>
                <Descriptions.Item label="名称">{selectedNode.name}</Descriptions.Item>
                <Descriptions.Item label="IP">{selectedNode.node_ip || '-'}</Descriptions.Item>
                <Descriptions.Item label="状态">
                  {selectedNode.is_online ? (
                    <Badge status="success" text="在线" />
                  ) : selectedNode.is_local ? (
                    <Badge status="success" text="本机" />
                  ) : (
                    <Badge status="error" text="离线" />
                  )}
                </Descriptions.Item>
                {selectedNode.latency !== undefined && selectedNode.latency >= 0 && (
                  <Descriptions.Item label="延迟">
                    <Tag color="blue">{selectedNode.latency}ms</Tag>
                  </Descriptions.Item>
                )}
              </Descriptions>

              {/* 隧道统计 */}
              {selectedNode.tunnels && (
                <>
                  <Divider style={{ margin: '8px 0' }} />
                  <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>隧道统计</Text>
                  <Space wrap size={[4, 4]}>
                    <Tag>总计: {selectedNode.tunnels.total || 0}</Tag>
                    {selectedNode.tunnels.port_forward > 0 && <Tag color="blue">端口转发: {selectedNode.tunnels.port_forward}</Tag>}
                    {selectedNode.tunnels.stun > 0 && <Tag color="cyan">STUN: {selectedNode.tunnels.stun}</Tag>}
                    {selectedNode.tunnels.frp > 0 && <Tag color="orange">FRP: {selectedNode.tunnels.frp}</Tag>}
                    {selectedNode.tunnels.nps > 0 && <Tag color="purple">NPS: {selectedNode.tunnels.nps}</Tag>}
                    {selectedNode.tunnels.easytier > 0 && <Tag color="green">EasyTier: {selectedNode.tunnels.easytier}</Tag>}
                  </Space>
                </>
              )}

              {/* 连接关系 */}
              {(() => {
                const nodeEdges = getNodeEdges(selectedNode.id)
                if (nodeEdges.length === 0) return null
                return (
                  <>
                    <Divider style={{ margin: '8px 0' }} />
                    <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>连接关系</Text>
                    {nodeEdges.map((edge, i) => {
                      const isSource = edge.source === selectedNode.id
                      const peerId = isSource ? edge.target : edge.source
                      const peerNode = nodes.find(n => n.id === peerId)
                      return (
                        <div key={i} style={{ marginBottom: 6, padding: '4px 8px', background: 'rgba(0,0,0,0.02)', borderRadius: 6 }}>
                          <Space size={4}>
                            <Tag color={EDGE_COLORS[edge.tunnel_type]} style={{ fontSize: 11 }}>
                              {edge.tunnel_type.toUpperCase()}
                            </Tag>
                            <Text style={{ fontSize: 12 }}>
                              {isSource ? edge.source_label : edge.target_label}
                            </Text>
                            <Text type="secondary" style={{ fontSize: 12 }}>
                              {DIRECTION_LABELS[edge.direction]}
                            </Text>
                            <Text style={{ fontSize: 12 }}>
                              {peerNode?.name || `节点#${peerId}`}
                            </Text>
                          </Space>
                        </div>
                      )
                    })}
                  </>
                )
              })()}

              {/* 隧道详情列表 */}
              {selectedNode.tunnels?.details && selectedNode.tunnels.details.length > 0 && (
                <>
                  <Divider style={{ margin: '8px 0' }} />
                  <Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>
                    隧道列表 ({selectedNode.tunnels.details.length})
                  </Text>
                  <div style={{ maxHeight: 200, overflow: 'auto' }}>
                    {selectedNode.tunnels.details.map((t: any, i: number) => (
                      <div key={i} style={{ marginBottom: 4, padding: '3px 6px', background: 'rgba(0,0,0,0.02)', borderRadius: 4, fontSize: 12 }}>
                        <Space size={4}>
                          <Tag style={{ fontSize: 10 }}>{t.type}</Tag>
                          <Text>{t.name}</Text>
                          <Tag color={t.status === 'running' ? 'success' : 'default'} style={{ fontSize: 10 }}>{t.status}</Tag>
                        </Space>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
          ) : (
            <Empty description="点击节点查看详情" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}
        </Card>
      </div>
    </div>
  )
}

export default MeshTopology

import React, { useEffect, useState, useCallback, useMemo } from 'react'
import {
  Select, Typography, Tag, Tabs, Badge,
  Space, Empty, Button,
} from 'antd'
import {
  SyncOutlined, CloudServerOutlined, ApiOutlined,
  NodeIndexOutlined, WifiOutlined, SwapOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { meshNodeApi, createRemoteTunnelApi } from '../api'
import { TunnelApiProvider } from '../contexts/TunnelApiContext'

// 原有页面组件
import PortForward from './PortForward'
import Stun from './Stun'
import FrpClient from './FrpClient'
import FrpServer from './FrpServer'
import NpsClient from './NpsClient'
import NpsServer from './NpsServer'
import EasytierClient from './EasytierClient'
import EasytierServer from './EasytierServer'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text } = Typography

// 隧道类型定义
const TUNNEL_TYPES = [
  { key: 'port-forward', label: '端口转发', icon: <NodeIndexOutlined /> },
  { key: 'stun', label: 'STUN穿透', icon: <WifiOutlined /> },
  { key: 'frpc', label: 'FRP客户端', icon: <ApiOutlined /> },
  { key: 'frps', label: 'FRP服务端', icon: <CloudServerOutlined /> },
  { key: 'nps-client', label: 'NPS客户端', icon: <ApiOutlined /> },
  { key: 'nps-server', label: 'NPS服务端', icon: <CloudServerOutlined /> },
  { key: 'easytier-client', label: 'EasyTier客户端', icon: <ApiOutlined /> },
  { key: 'easytier-server', label: 'EasyTier服务端', icon: <CloudServerOutlined /> },
]

// 隧道类型 -> 页面组件映射
const TUNNEL_COMPONENTS: Record<string, React.FC> = {
  'port-forward': PortForward,
  'stun': Stun,
  'frpc': FrpClient,
  'frps': FrpServer,
  'nps-client': NpsClient,
  'nps-server': NpsServer,
  'easytier-client': EasytierClient,
  'easytier-server': EasytierServer,
}

const MeshTunnels: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [nodes, setNodes] = useState<any[]>([])
  const [selectedNodeId, setSelectedNodeId] = useState<number | null>(null)
  const [selectedType, setSelectedType] = useState('frpc')
  const [nodesLoading, setNodesLoading] = useState(false)
  // 用于强制刷新子组件
  const [refreshKey, setRefreshKey] = useState(0)

  // 加载节点列表
  const fetchNodes = useCallback(async () => {
    setNodesLoading(true)
    try {
      const res: any = await meshNodeApi.listNodes()
      const nodeList = res.data || []
      setNodes(nodeList)
      // 默认选择第一个启用的节点
      if (nodeList.length > 0 && selectedNodeId === null) {
        const enabledNode = nodeList.find((n: any) => n.enable && n.is_online)
        setSelectedNodeId(enabledNode?.id || nodeList[0].id)
      }
    } finally {
      setNodesLoading(false)
    }
  }, [selectedNodeId])

  useEffect(() => { fetchNodes() }, [])

  // 获取选中节点信息
  const selectedNode = nodes.find((n: any) => n.id === selectedNodeId)

  // 创建远程 API 适配器（根据节点和隧道类型）
  const tunnelApi = useMemo(() => {
    if (selectedNodeId === null || !selectedNode) return null
    return createRemoteTunnelApi(selectedType, selectedNodeId, selectedNode.is_local || false)
  }, [selectedNodeId, selectedNode, selectedType])

  // TunnelApiProvider 的 value
  const tunnelCtxValue = useMemo(() => {
    if (!tunnelApi) return null
    return {
      api: tunnelApi,
      isRemoteMode: true,
      onRefresh: () => setRefreshKey(k => k + 1),
    }
  }, [tunnelApi])

  // 获取当前隧道类型对应的页面组件
  const TunnelComponent = TUNNEL_COMPONENTS[selectedType]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <SwapOutlined style={{ marginRight: 8 }} />
          组网隧道管理
        </Typography.Title>
        <Button icon={<SyncOutlined />} onClick={() => setRefreshKey(k => k + 1)}>刷新</Button>
      </div>

      {/* 节点选择器 */}
      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
        <Text strong>选择节点：</Text>
        <Select
          value={selectedNodeId}
          onChange={(val) => {
            setSelectedNodeId(val)
            setRefreshKey(k => k + 1)
          }}
          loading={nodesLoading}
          style={{ minWidth: 250 }}
          placeholder="请选择节点"
        >
          {nodes.filter((n: any) => n.enable).map((node: any) => (
            <Select.Option key={node.id} value={node.id}>
              <Space>
                <Badge status={node.is_online ? 'success' : 'error'} />
                {node.name}
                {node.is_local && <Tag color="blue" style={{ fontSize: 11, padding: '0 4px' }}>本机</Tag>}
                <Text type="secondary" style={{ fontSize: 12 }}>({node.node_ip || node.url})</Text>
                {node.latency >= 0 && !node.is_local && <Tag color="blue" style={{ fontSize: 11 }}>{node.latency}ms</Tag>}
              </Space>
            </Select.Option>
          ))}
        </Select>
        {selectedNode && !selectedNode.is_online && (
          <Tag color="error">节点离线，可能无法获取数据</Tag>
        )}
      </div>

      {/* 隧道类型选择 */}
      <Tabs
        activeKey={selectedType}
        onChange={(key) => {
          setSelectedType(key)
          setRefreshKey(k => k + 1)
        }}
        items={TUNNEL_TYPES.map(tt => ({
          key: tt.key,
          label: (
            <Space>
              {tt.icon}
              {tt.label}
            </Space>
          ),
        }))}
        style={{ marginBottom: 0 }}
      />

      {/* 隧道内容区域 - 嵌入原有页面组件 */}
      {selectedNodeId === null ? (
        <Empty description="请先选择一个节点" style={{ marginTop: 48 }} />
      ) : tunnelCtxValue && TunnelComponent ? (
        <TunnelApiProvider value={tunnelCtxValue}>
          <TunnelComponent key={`${selectedNodeId}-${selectedType}-${refreshKey}`} />
        </TunnelApiProvider>
      ) : (
        <Empty description="不支持的隧道类型" style={{ marginTop: 48 }} />
      )}
    </div>
  )
}

export default MeshTunnels

import React, { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Table, Tag, Space, Button, App, Spin } from 'antd'
import {
  CloudServerOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import ReactECharts from 'echarts-for-react'
import * as echarts from 'echarts'
import { monitorApi } from '../api'
import { useTranslation } from 'react-i18next'

// 世界地图 GeoJSON CDN 地址
const WORLD_MAP_URL = 'https://geo.datav.aliyun.com/areas_v3/bound/100000_full.json'

interface Server {
  id: number
  name: string
  display_name: string
  is_online: boolean
  country: string
  latitude: number
  longitude: number
  last_heartbeat: string
  cpu_usage?: number
  mem_usage?: number
  disk_usage?: number
}

interface Metric {
  cpu_usage: number
  mem_usage: number
  disk_usage: number
  net_sent: number
  net_recv: number
  load_avg_1: number
}

const MonitorDashboard: React.FC = () => {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const [loading, setLoading] = useState(false)
  const [mapReady, setMapReady] = useState(false)
  const [servers, setServers] = useState<Server[]>([])
  const [mapData, setMapData] = useState<any[]>([])
  const [stats, setStats] = useState({
    total: 0,
    online: 0,
    offline: 0,
    warning: 0,
  })

  // 加载世界地图数据
  useEffect(() => {
    const loadWorldMap = async () => {
      try {
        if (!echarts.getMap('world')) {
          const response = await fetch(WORLD_MAP_URL)
          const geoJson = await response.json()
          echarts.registerMap('world', geoJson)
        }
        setMapReady(true)
      } catch (error) {
        console.error('加载地图失败:', error)
        message.warning('地图加载失败，使用备用显示方式')
        setMapReady(true) // 即使失败也设置为 true，使用备用方案
      }
    }
    loadWorldMap()
  }, [])

  useEffect(() => {
    if (mapReady) {
      loadData()
      const timer = setInterval(loadData, 30000) // 每 30 秒刷新
      return () => clearInterval(timer)
    }
  }, [mapReady])

  const loadData = async () => {
    setLoading(true)
    try {
      const response = await monitorApi.listServers()
      const serverList = response.data || []
      setServers(serverList)

      // 统计数据
      const total = serverList.length
      const online = serverList.filter((s: Server) => s.is_online).length
      const offline = total - online
      const warning = serverList.filter(
        (s: Server) => s.is_online && (s.cpu_usage || 0) > 80
      ).length

      setStats({ total, online, offline, warning })

      // 地图数据
      const mapPoints = serverList
        .filter((s: Server) => s.latitude && s.longitude)
        .map((s: Server) => ({
          name: s.display_name || s.name,
          value: [s.longitude, s.latitude, s.is_online ? 1 : 0],
          itemStyle: {
            color: s.is_online ? '#52c41a' : '#ff4d4f',
          },
        }))
      setMapData(mapPoints)
    } catch (error: any) {
      message.error(error.message || '加载数据失败')
    } finally {
      setLoading(false)
    }
  }

  const getMapOption = () => {
    return {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => {
          if (params.componentSubType === 'scatter' || params.componentSubType === 'effectScatter') {
            return `${params.name}<br/>状态: ${params.value[2] ? '在线' : '离线'}`
          }
          return params.name
        },
      },
      geo: {
        map: 'world',
        roam: true,
        zoom: 1.2,
        center: [0, 0],
        itemStyle: {
          areaColor: '#0F131C',
          borderColor: '#1E2636',
          borderWidth: 0.5,
        },
        emphasis: {
          itemStyle: {
            areaColor: '#161D2B',
          },
        },
        label: {
          show: false,
        },
      },
      series: [
        {
          name: '服务器',
          type: 'scatter',
          coordinateSystem: 'geo',
          data: mapData,
          symbolSize: 14,
          label: {
            show: false,
          },
          emphasis: {
            label: {
              show: true,
              formatter: '{b}',
              position: 'top',
              color: '#fff',
              fontSize: 12,
            },
            itemStyle: {
              borderColor: '#fff',
              borderWidth: 2,
            },
          },
        },
        {
          name: '在线服务器',
          type: 'effectScatter',
          coordinateSystem: 'geo',
          data: mapData.filter((d) => d.value[2] === 1),
          symbolSize: 10,
          showEffectOn: 'render',
          rippleEffect: {
            brushType: 'stroke',
            scale: 3,
            period: 4,
          },
          itemStyle: {
            color: '#52c41a',
            shadowBlur: 10,
            shadowColor: '#52c41a',
          },
        },
      ],
    }
  }

  const columns = [
    {
      title: t('monitor.server_name'),
      dataIndex: 'display_name',
      key: 'display_name',
      render: (text: string, record: Server) => (
        <Space>
          <CloudServerOutlined />
          <span>{text || record.name}</span>
        </Space>
      ),
    },
    {
      title: t('monitor.status'),
      dataIndex: 'is_online',
      key: 'is_online',
      render: (online: boolean) => (
        <Tag icon={online ? <CheckCircleOutlined /> : <CloseCircleOutlined />} color={online ? 'success' : 'error'}>
          {online ? t('monitor.online') : t('monitor.offline')}
        </Tag>
      ),
    },
    {
      title: t('monitor.location'),
      dataIndex: 'country',
      key: 'country',
    },
    {
      title: 'CPU',
      dataIndex: 'cpu_usage',
      key: 'cpu_usage',
      render: (usage: number) => {
        if (!usage) return '-'
        const color = usage > 80 ? 'red' : usage > 60 ? 'orange' : 'green'
        return <Tag color={color}>{usage.toFixed(1)}%</Tag>
      },
    },
    {
      title: t('monitor.memory'),
      dataIndex: 'mem_usage',
      key: 'mem_usage',
      render: (usage: number) => {
        if (!usage) return '-'
        const color = usage > 80 ? 'red' : usage > 60 ? 'orange' : 'green'
        return <Tag color={color}>{usage.toFixed(1)}%</Tag>
      },
    },
    {
      title: t('monitor.disk'),
      dataIndex: 'disk_usage',
      key: 'disk_usage',
      render: (usage: number) => {
        if (!usage) return '-'
        const color = usage > 80 ? 'red' : usage > 60 ? 'orange' : 'green'
        return <Tag color={color}>{usage.toFixed(1)}%</Tag>
      },
    },
    {
      title: t('monitor.last_heartbeat'),
      dataIndex: 'last_heartbeat',
      key: 'last_heartbeat',
      render: (time: string) => {
        if (!time) return '-'
        return new Date(time).toLocaleString()
      },
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {/* 顶部统计卡片 */}
        <Row gutter={16}>
          <Col span={6}>
            <Card>
              <Statistic
                title={t('monitor.total_servers')}
                value={stats.total}
                prefix={<CloudServerOutlined />}
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title={t('monitor.online_servers')}
                value={stats.online}
                prefix={<CheckCircleOutlined />}
                valueStyle={{ color: '#52c41a' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title={t('monitor.offline_servers')}
                value={stats.offline}
                prefix={<CloseCircleOutlined />}
                valueStyle={{ color: '#ff4d4f' }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card>
              <Statistic
                title={t('monitor.warning_servers')}
                value={stats.warning}
                prefix={<ExclamationCircleOutlined />}
                valueStyle={{ color: '#faad14' }}
              />
            </Card>
          </Col>
        </Row>

        {/* 世界地图 */}
        <Card
          title={t('monitor.server_map')}
          extra={
            <Button icon={<ReloadOutlined />} onClick={loadData} loading={loading}>
              {t('common.refresh')}
            </Button>
          }
        >
          <Spin spinning={loading || !mapReady}>
            {mapReady ? (
              <ReactECharts option={getMapOption()} style={{ height: '500px' }} notMerge={true} lazyUpdate={true} />
            ) : (
              <div style={{ height: '500px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                加载地图中...
              </div>
            )}
          </Spin>
        </Card>

        {/* 服务器列表 */}
        <Card title={t('monitor.server_list')}>
          <Table
            columns={columns}
            dataSource={servers}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10 }}
          />
        </Card>
      </Space>
    </div>
  )
}

export default MonitorDashboard

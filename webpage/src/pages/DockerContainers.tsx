import React, { useCallback, useEffect, useState } from 'react'
import { Table, Button, Space, Tag, Tooltip, message, Typography, Switch } from 'antd'
import {
  PlayCircleOutlined, StopOutlined, SyncOutlined, RedoOutlined,
} from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { dockerApi } from '../api'
import { useTableStyle } from '../hooks/useTableStyle'

const { Text } = Typography

type PortMapping = {
  host_ip: string
  host_port: number
  container_port: number
  protocol: string
}

type Container = {
  id: string
  name: string
  image: string
  state: string
  status: string
  created: number
  ports: PortMapping[]
}

const DockerContainers: React.FC = () => {
  const { t } = useTranslation()
  const tableStyle = useTableStyle()
  const [data, setData] = useState<Container[]>([])
  const [loading, setLoading] = useState(false)
  const [showAll, setShowAll] = useState(true)
  const [acting, setActing] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await dockerApi.list(showAll)
      setData((res?.data ?? []) as Container[])
    } catch (e: any) {
      const msg = e?.response?.data?.message || e?.message || t('docker.loadFailed')
      message.error(msg)
    } finally {
      setLoading(false)
    }
  }, [showAll, t])

  useEffect(() => { load() }, [load])

  const act = async (c: Container, op: 'start' | 'stop' | 'restart') => {
    setActing(c.id + op)
    try {
      await dockerApi[op](c.id)
      message.success(t('common.success'))
      await load()
    } catch (e: any) {
      message.error(e?.response?.data?.message || t('docker.actionFailed'))
    } finally {
      setActing('')
    }
  }

  const columns = [
    {
      title: t('common.name'),
      key: 'name',
      render: (_: any, row: Container) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.name || '-'}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{row.id}</Text>
        </Space>
      ),
    },
    {
      title: t('docker.image'),
      dataIndex: 'image',
      key: 'image',
      ellipsis: true,
    },
    {
      title: t('common.status'),
      key: 'state',
      width: 190,
      render: (_: any, row: Container) => (
        <Space direction="vertical" size={0}>
          <Tag color={row.state === 'running' ? 'green' : 'default'}>
            {row.state === 'running' ? t('common.running') : row.state}
          </Tag>
          {row.status ? (
            <Text type="secondary" style={{ fontSize: 12 }}>{row.status}</Text>
          ) : null}
        </Space>
      ),
    },
    {
      title: t('docker.ports'),
      key: 'ports',
      render: (_: any, row: Container) => (
        row.ports.length === 0
          ? <Text type="secondary">-</Text>
          : (
            <Space size={[4, 4]} wrap>
              {row.ports.map((p) => (
                <Tag key={`${p.host_port}-${p.container_port}-${p.protocol}`}>
                  {p.host_port}→{p.container_port}/{p.protocol}
                </Tag>
              ))}
            </Space>
          )
      ),
    },
    {
      title: t('common.action'),
      key: 'action',
      width: 150,
      render: (_: any, row: Container) => (
        <Space size={4}>
          {row.state === 'running' ? (
            <>
              <Tooltip title={t('common.stop')}>
                <Button
                  size="small"
                  danger
                  icon={<StopOutlined />}
                  loading={acting === row.id + 'stop'}
                  onClick={() => act(row, 'stop')}
                />
              </Tooltip>
              <Tooltip title={t('common.restart')}>
                <Button
                  size="small"
                  icon={<RedoOutlined />}
                  loading={acting === row.id + 'restart'}
                  onClick={() => act(row, 'restart')}
                />
              </Tooltip>
            </>
          ) : (
            <Tooltip title={t('common.start')}>
              <Button
                size="small"
                type="primary"
                icon={<PlayCircleOutlined />}
                loading={acting === row.id + 'start'}
                onClick={() => act(row, 'start')}
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Text strong style={{ fontSize: 16 }}>{t('docker.title')}</Text>
        <Space>
          <Space size={6}>
            <Text type="secondary">{t('docker.showAll')}</Text>
            <Switch checked={showAll} onChange={setShowAll} size="small" />
          </Space>
          <Button icon={<SyncOutlined />} onClick={load} loading={loading}>
            {t('common.refresh')}
          </Button>
        </Space>
      </div>

      <Table<Container>
        rowKey="id"
        columns={columns as any}
        dataSource={data}
        loading={loading}
        size="small"
        scroll={{ x: 900 }}
        pagination={{ pageSize: 20 }}
        locale={{ emptyText: t('docker.empty') }}
        {...tableStyle}
      />
    </div>
  )
}

export default DockerContainers

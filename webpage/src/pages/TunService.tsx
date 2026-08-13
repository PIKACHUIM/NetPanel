import React, {useEffect, useState} from 'react'
import {
    Badge,
    Button,
    Drawer,
    Form,
    Input,
    InputNumber,
    message,
    Modal,
    Popconfirm,
    Select,
    Space,
    Switch,
    Table,
    Tag,
    Tooltip,
    Typography,
} from 'antd'
import {
    DeleteOutlined,
    EditOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
    StopOutlined,
    SettingOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {lineregApi, tunserviceApi} from '../api'
import StatusTag from '../components/StatusTag'

const {Option} = Select
const {Text} = Typography

const toolColor: Record<string, string> = {
    frp: 'blue',
    nps: 'cyan',
    easytier: 'purple',
    wireguard: 'green',
    cloudflare: 'orange',
}

const layerColor: Record<string, string> = {
    port: 'default',
    domain: 'gold',
}

const TunService: React.FC = () => {
    const {t} = useTranslation()
    const [data, setData] = useState<any[]>([])
    const [loading, setLoading] = useState(false)
    const [modalOpen, setModalOpen] = useState(false)
    const [editing, setEditing] = useState<any>(null)
    const [detailOpen, setDetailOpen] = useState(false)
    const [detail, setDetail] = useState<any>(null)
    const [history, setHistory] = useState<Record<string, any[]>>({})
    const [form] = Form.useForm()
    const [probeOpen, setProbeOpen] = useState(false)
    const [probeLoading, setProbeLoading] = useState(false)
    const [probeForm] = Form.useForm()

    const load = async () => {
        setLoading(true)
        try {
            const res = await tunserviceApi.list()
            setData(res?.data || [])
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        load()
        const timer = setInterval(load, 5000)
        return () => clearInterval(timer)
    }, [])

    const openCreate = () => {
        setEditing(null)
        form.resetFields()
        form.setFieldsValue({protocol: 'tcp', enable: false, line_refs: '[]'})
        setModalOpen(true)
    }

    const openEdit = (row: any) => {
        setEditing(row)
        form.resetFields()
        form.setFieldsValue({
            ...row,
            line_refs: row.line_refs || '[]',
        })
        setModalOpen(true)
    }

    const submit = async () => {
        const values = await form.validateFields()
        // 归一化 line_refs：接受 JSON 字符串
        if (typeof values.line_refs === 'string') {
            try {
                JSON.parse(values.line_refs)
            } catch {
                message.error(t('tunservice.lineRefsInvalid'))
                return
            }
        }
        try {
            if (editing) {
                await tunserviceApi.update(editing.id, {...editing, ...values})
                message.success(t('common.updated'))
            } else {
                await tunserviceApi.create(values)
                message.success(t('common.created'))
            }
            setModalOpen(false)
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const start = async (id: number) => {
        try {
            await tunserviceApi.start(id)
            message.success(t('common.started'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const stop = async (id: number) => {
        try {
            await tunserviceApi.stop(id)
            message.success(t('common.stopped'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const remove = async (id: number) => {
        try {
            await tunserviceApi.delete(id)
            message.success(t('common.deleted'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const openProbe = async () => {
        setProbeOpen(true)
        setProbeLoading(true)
        try {
            const res = await lineregApi.getConfig()
            probeForm.setFieldsValue({
                interval_sec: res?.data?.interval_sec ?? 60,
                failure_threshold: res?.data?.failure_threshold ?? 2,
                tolerance_ms: res?.data?.tolerance_ms ?? 50,
                max_concurrent: res?.data?.max_concurrent ?? 8,
            })
        } catch {
            message.error(t('tunservice.loadFailed'))
        } finally {
            setProbeLoading(false)
        }
    }

    const saveProbe = async () => {
        const values = await probeForm.validateFields()
        try {
            await lineregApi.updateConfig(values)
            message.success(t('tunservice.saved'))
            setProbeOpen(false)
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const showDetail = async (id: number) => {
        setDetailOpen(true)
        setHistory({})
        try {
            const [detailRes, historyRes] = await Promise.all([
                tunserviceApi.get(id),
                tunserviceApi.history(id, 100),
            ])
            setDetail(detailRes?.data || null)
            setHistory(historyRes?.data || {})
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    // 迷你延迟趋势图（SVG 折线，不可用点标红）
    const Sparkline = ({points}: {points: any[]}) => {
        const W = 240
        const H = 48
        if (!points || points.length < 2) {
            return <Text type="secondary" style={{fontSize: 12}}>{t('tunservice.noHistory')}</Text>
        }
        const lat = points.map(p => {
            const v = (p.http_latency || 0) > 0 ? p.http_latency : p.tcp_latency
            return v > 0 ? v / 1e6 : null // ms
        })
        const valid = lat.filter(v => v !== null) as number[]
        if (valid.length === 0) {
            return <Text type="secondary" style={{fontSize: 12}}>{t('tunservice.noHistory')}</Text>
        }
        const max = Math.max(...valid, 1)
        const n = points.length
        const stepX = n > 1 ? W / (n - 1) : W
        const coords = points.map((p, i) => {
            const v = lat[i]
            const y = v === null ? H : H - (v / max) * (H - 6) - 3
            return {x: i * stepX, y, ok: p.available}
        })
        const line = coords.map(c => `${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(' ')
        const down = coords.filter(c => !c.ok)
        return (
            <svg width={W} height={H} style={{display: 'block'}}>
                <polyline points={line} fill="none" stroke="#1677ff" strokeWidth={1.5}/>
                {down.map((c, i) => (
                    <circle key={i} cx={c.x} cy={c.y} r={2.5} fill="#ff4d4f"/>
                ))}
            </svg>
        )
    }

    const renderLines = (lines: any[]) => {
        if (!lines || lines.length === 0) {
            return <Text type="secondary">{t('tunservice.noLines')}</Text>
        }
        return (
            <Space size={4} wrap>
                {lines.map((l: any) => (
                    <Tooltip key={l.id} title={`${l.id} · ${l.address || '-'}`}>
                        <Tag color={toolColor[l.tool] || 'default'}>
                            {l.name || l.id}
                            <Tag color={layerColor[l.layer] || 'default'} style={{marginLeft: 4}}>{l.layer || 'port'}</Tag>
                            {l.status === 'running' ? <Badge status="success"/> : l.status === 'error' ? <Badge status="error"/> : <Badge status="default"/>}
                        </Tag>
                    </Tooltip>
                ))}
            </Space>
        )
    }

    const columns = [
        {
            title: 'ID',
            dataIndex: 'id',
            width: 60,
        },
        {
            title: t('common.name'),
            dataIndex: 'name',
            width: 160,
        },
        {
            title: t('tunservice.target'),
            key: 'target',
            width: 200,
            render: (_: any, row: any) => `${row.target_address}:${row.target_port} (${row.protocol || 'tcp'})`,
        },
        {
            title: t('tunservice.lines'),
            dataIndex: 'lines',
            render: (_: any, row: any) => renderLines(row.lines),
        },
        {
            title: t('common.status'),
            dataIndex: 'status',
            width: 100,
            render: (v: string) => <StatusTag status={v}/>,
        },
        {
            title: t('common.actions'),
            key: 'actions',
            width: 240,
            render: (_: any, row: any) => (
                <Space>
                    {row.status === 'running' ? (
                        <Tooltip title={t('common.stop')}>
                            <Button size="small" danger icon={<StopOutlined/>} onClick={() => stop(row.id)}/>
                        </Tooltip>
                    ) : (
                        <Tooltip title={t('common.start')}>
                            <Button size="small" type="primary" icon={<PlayCircleOutlined/>} onClick={() => start(row.id)}/>
                        </Tooltip>
                    )}
                    <Tooltip title={t('tunservice.detail')}>
                        <Button size="small" onClick={() => showDetail(row.id)}>{t('tunservice.detail')}</Button>
                    </Tooltip>
                    <Tooltip title={t('common.edit')}>
                        <Button size="small" icon={<EditOutlined/>} onClick={() => openEdit(row)}/>
                    </Tooltip>
                    <Popconfirm title={t('common.confirmDelete')} onConfirm={() => remove(row.id)}>
                        <Tooltip title={t('common.delete')}>
                            <Button size="small" danger icon={<DeleteOutlined/>}/>
                        </Tooltip>
                    </Popconfirm>
                </Space>
            ),
        },
    ]

    return (
        <div>
            <Space style={{marginBottom: 16}}>
                <Button type="primary" icon={<PlusOutlined/>} onClick={openCreate}>
                    {t('common.create')}
                </Button>
                <Button icon={<ReloadOutlined/>} onClick={load}>
                    {t('common.refresh')}
                </Button>
                <Button icon={<SettingOutlined/>} onClick={openProbe}>
                    {t('tunservice.probeConfig')}
                </Button>
            </Space>

            <Table
                rowKey="id"
                loading={loading}
                columns={columns}
                dataSource={data}
                pagination={{pageSize: 10, showSizeChanger: false}}
                size="middle"
            />

            <Modal
                title={editing ? t('common.edit') : t('common.create')}
                open={modalOpen}
                onOk={submit}
                onCancel={() => setModalOpen(false)}
                destroyOnClose
                width={560}
            >
                <Form form={form} layout="vertical">
                    <Form.Item name="name" label={t('common.name')} rules={[{required: true}]}>
                        <Input/>
                    </Form.Item>
                    <Form.Item name="target_address" label={t('tunservice.targetAddress')} rules={[{required: true}]}>
                        <Input placeholder="127.0.0.1 或 nas.local"/>
                    </Form.Item>
                    <Form.Item name="target_port" label={t('tunservice.targetPort')} rules={[{required: true}]}>
                        <InputNumber min={1} max={65535} style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item name="protocol" label={t('tunservice.protocol')}>
                        <Select>
                            <Option value="tcp">TCP</Option>
                            <Option value="udp">UDP</Option>
                        </Select>
                    </Form.Item>
                    <Form.Item
                        name="line_refs"
                        label={t('tunservice.lineRefs')}
                        tooltip={t('tunservice.lineRefsTip')}
                    >
                        <Input.TextArea rows={3} placeholder='["frp:1","cftunnel:2"]'/>
                    </Form.Item>
                    <Form.Item name="locked_line" label={t('tunservice.lockLine')}>
                        <Select allowClear placeholder={t('tunservice.auto')}>
                            <Option value="">{t('tunservice.auto')}</Option>
                            {editing?.lines?.map((l: any) => (
                                <Option key={l.id} value={l.id}>{l.name || l.id}</Option>
                            ))}
                        </Select>
                    </Form.Item>
                    <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                        <Switch/>
                    </Form.Item>
                    <Form.Item name="remark" label={t('common.remark')}>
                        <Input.TextArea rows={2}/>
                    </Form.Item>
                </Form>
            </Modal>

            <Modal
                title={t('tunservice.probeConfig')}
                open={probeOpen}
                onOk={saveProbe}
                onCancel={() => setProbeOpen(false)}
                destroyOnClose
                width={480}
                confirmLoading={probeLoading}
            >
                <Form form={probeForm} layout="vertical">
                    <Form.Item name="interval_sec" label={t('tunservice.probeInterval')} rules={[{required: true}]}>
                        <InputNumber min={5} max={3600} style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item name="failure_threshold" label={t('tunservice.failureThreshold')} rules={[{required: true}]}>
                        <InputNumber min={1} max={10} style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item name="tolerance_ms" label={t('tunservice.toleranceMs')} rules={[{required: true}]}>
                        <InputNumber min={0} max={5000} style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item name="max_concurrent" label={t('tunservice.maxConcurrent')} rules={[{required: true}]}>
                        <InputNumber min={1} max={64} style={{width: '100%'}}/>
                    </Form.Item>
                </Form>
            </Modal>

            <Drawer
                title={detail ? detail.name : t('tunservice.detail')}
                open={detailOpen}
                onClose={() => setDetailOpen(false)}
                width={480}
            >
                {detail && (
                    <Space direction="vertical" style={{width: '100%'}}>
                        <div>
                            <Text type="secondary">{t('tunservice.target')}: </Text>
                            <Text strong>{detail.target_address}:{detail.target_port} ({detail.protocol})</Text>
                        </div>
                        <div>
                            <Text type="secondary">{t('common.status')}: </Text>
                            <StatusTag status={detail.status}/>
                        </div>
                        <div>
                            <Text type="secondary">{t('tunservice.lines')}: </Text>
                            <div style={{marginTop: 8}}>{renderLines(detail.lines)}</div>
                        </div>
                        {detail.lines?.length > 0 && (
                            <Table
                                rowKey="id"
                                size="small"
                                pagination={false}
                                dataSource={detail.lines}
                                columns={[
                                    {title: 'ID', dataIndex: 'id', width: 110},
                                    {title: t('common.name'), dataIndex: 'name'},
                                    {
                                        title: t('tunservice.latency'),
                                        dataIndex: 'latency',
                                        width: 100,
                                        render: (v: number) => (v > 0 ? `${(v / 1e6).toFixed(1)} ms` : '-'),
                                    },
                                    {
                                        title: t('tunservice.trend'),
                                        key: 'trend',
                                        render: (_: any, row: any) => <Sparkline points={history[row.id] || []}/>,
                                    },
                                    {
                                        title: t('common.status'),
                                        dataIndex: 'status',
                                        width: 90,
                                        render: (v: string) => <StatusTag status={v}/>,
                                    },
                                ]}
                            />
                        )}
                    </Space>
                )}
            </Drawer>
        </div>
    )
}

export default TunService

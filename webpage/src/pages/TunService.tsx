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
    ThunderboltOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import ReactECharts from 'echarts-for-react'
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
    // 线路趋势图弹窗状态
    const [lineTrendOpen, setLineTrendOpen] = useState(false)
    const [lineTrendLineId, setLineTrendLineId] = useState<string>('')
    const [lineTrendData, setLineTrendData] = useState<any[]>([])
    const [lineTrendLoading, setLineTrendLoading] = useState(false)
    const [form] = Form.useForm()
    const [probeOpen, setProbeOpen] = useState(false)
    const [probeLoading, setProbeLoading] = useState(false)
    const [probeForm] = Form.useForm()
    const [speedtestOpen, setSpeedtestOpen] = useState(false)
    const [speedtestLoading, setSpeedtestLoading] = useState(false)
    const [speedtestRow, setSpeedtestRow] = useState<any>(null)
    const [speedtestData, setSpeedtestData] = useState<any[]>([])

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
                tool_filter: res?.data?.tool_filter ?? '',
                rebind_mode: res?.data?.rebind_mode ?? 'auto',
            })
        } catch {
            // 加载失败时回填默认值，用户仍可直接修改后保存
            probeForm.setFieldsValue({
                interval_sec: 60,
                failure_threshold: 2,
                tolerance_ms: 50,
                max_concurrent: 8,
            })
            message.warning(t('tunservice.loadFailedFallback'))
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

    // 半自动模式：手动触发全部待重绑的端口层服务（先提示数量，再执行）。
    const applyPendingRebinds = async () => {
        try {
            const pending = await lineregApi.rebindPending()
            const n = Object.keys(pending?.data || {}).length
            if (n === 0) {
                message.info(t('tunservice.rebindApplied', {n: 0}))
                return
            }
            const res = await lineregApi.rebindApply()
            message.success(t('tunservice.rebindApplied', {n: res?.data?.applied ?? n}))
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    // 测速：对服务关联线路做一次即时并发测速，弹窗展示（延迟升序、失败标红）。
    const runSpeedtest = async (row: any) => {
        setSpeedtestRow(row)
        setSpeedtestOpen(true)
        setSpeedtestLoading(true)
        setSpeedtestData([])
        try {
            const res = await tunserviceApi.speedtest(row.id)
            setSpeedtestData(res?.data || [])
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        } finally {
            setSpeedtestLoading(false)
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

    // 加载单线路探测历史（用于 ECharts 趋势图弹窗）
    const loadLineTrend = async (lineId: string) => {
        setLineTrendOpen(true)
        setLineTrendLineId(lineId)
        setLineTrendData([])
        setLineTrendLoading(true)
        try {
            const res = await lineregApi.getLineHistory(lineId, 100)
            setLineTrendData(res?.data || [])
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('tunservice.lineTrendLoadFailed'))
        } finally {
            setLineTrendLoading(false)
        }
    }

    // 构建 ECharts option（延迟趋势 + 可用性标记）
    const buildTrendOption = (data: any[]) => {
        if (!data || data.length === 0) {
            return {
                title: {text: t('tunservice.noHistory'), left: 'center', top: 'center', textStyle: {color: '#999'}},
                xAxis: {type: 'category', show: false},
                yAxis: {type: 'value', show: false},
                series: [],
            }
        }
        // 取 http_latency（有值优先）或 tcp_latency，转 ms
        const latencies = data.map(p => {
            const v = (p.http_latency || 0) > 0 ? p.http_latency : p.tcp_latency
            return v > 0 ? v / 1e6 : null
        })
        const times = data.map(p => {
            const d = new Date(p.created_at || p.id * 1000)
            return d.toLocaleTimeString()
        })
        const maxLat = Math.max(...latencies.filter(v => v !== null) as number[], 1)
        // 失败点：Y 轴顶端（maxLat * 1.2），成功点按实际延迟
        const failData = data
            .map((p, i) => (p.available === false ? {value: maxLat * 1.2, time: times[i]} : null))
            .filter(Boolean) as any[]
        const successData = data
            .map((p, i) => (p.available !== false && latencies[i] !== null ? {value: latencies[i], time: times[i]} : null))
            .filter(Boolean) as any[]
        return {
            tooltip: {trigger: 'axis'},
            legend: {data: ['延迟 (ms)', '失败点'], bottom: 0},
            grid: {left: 50, right: 20, top: 20, bottom: 40},
            xAxis: {type: 'category', data: times, axisLabel: {rotate: 45, fontSize: 10}},
            yAxis: {
                type: 'value',
                name: 'ms',
                max: maxLat * 1.3,
                axisLabel: {fontSize: 10},
                splitLine: {lineStyle: {color: '#eee'}},
            },
            series: [
                {
                    name: '延迟 (ms)',
                    type: 'line',
                    smooth: true,
                    data: successData.map((d: any) => d.value),
                    itemStyle: {color: '#1677ff'},
                    areaStyle: {
                        color: {
                            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
                            colorStops: [{offset: 0, color: 'rgba(22,119,255,0.25)'}, {offset: 1, color: 'rgba(22,119,255,0.03)'}],
                        },
                    },
                },
                {
                    name: '失败点',
                    type: 'scatter',
                    data: failData.map((d: any) => [d.time, d.value]),
                    itemStyle: {color: '#ff4d4f'},
                    symbolSize: 8,
                },
            ],
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
                    <Tooltip title={t('tunservice.speedtest')}>
                        <Button size="small" icon={<ThunderboltOutlined/>} onClick={() => runSpeedtest(row)}/>
                    </Tooltip>
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
                    <Form.Item name="tool_filter" label={t('tunservice.toolFilter')} tooltip={t('tunservice.toolFilterTip')}>
                        <Input placeholder="wireguard"/>
                    </Form.Item>
                    <Form.Item name="rebind_mode" label={t('tunservice.rebindMode')}>
                        <Select>
                            <Option value="auto">{t('tunservice.rebindAuto')}</Option>
                            <Option value="manual">{t('tunservice.rebindManual')}</Option>
                            <Option value="off">{t('tunservice.rebindOff')}</Option>
                        </Select>
                    </Form.Item>
                    <Form.Item>
                        <Button block onClick={applyPendingRebinds} icon={<ReloadOutlined/>}>
                            {t('tunservice.rebindApply')}
                        </Button>
                    </Form.Item>
                </Form>
            </Modal>

            <Modal
                title={`${t('tunservice.speedtest')} · ${speedtestRow?.name || ''}`}
                open={speedtestOpen}
                onCancel={() => setSpeedtestOpen(false)}
                footer={[
                    <Button key="retest" icon={<ThunderboltOutlined/>} loading={speedtestLoading} onClick={() => runSpeedtest(speedtestRow)}>
                        {t('tunservice.speedtestRetest')}
                    </Button>,
                    <Button key="close" type="primary" onClick={() => setSpeedtestOpen(false)}>
                        {t('common.close')}
                    </Button>,
                ]}
                destroyOnClose
                width={520}
            >
                <Table
                    rowKey="id"
                    size="small"
                    loading={speedtestLoading}
                    pagination={false}
                    dataSource={speedtestData}
                    columns={[
                        {title: 'ID', dataIndex: 'id', width: 120},
                        {title: t('common.name'), dataIndex: 'name'},
                        {
                            title: t('tunservice.latency'),
                            dataIndex: 'latency',
                            width: 110,
                            render: (v: number, row: any) => {
                                if (row.error) {
                                    return <Text type="danger">{t('tunservice.speedtestDown')}</Text>
                                }
                                return v > 0 ? <Text strong style={{color: '#faad14'}}>{`${(v / 1e6).toFixed(1)} ms`}</Text> : '-'
                            },
                        },
                        {
                            title: t('common.status'),
                            dataIndex: 'error',
                            width: 90,
                            render: (v: string) => (v ? <Badge status="error" text={t('tunservice.speedtestDown')}/> : <Badge status="success" text={t('common.normal')}/>),
                        },
                    ]}
                />
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
                                        render: (_: any, row: any) => (
                                            <Button
                                                type="link"
                                                size="small"
                                                onClick={() => loadLineTrend(row.id)}
                                                icon={<ReloadOutlined style={{fontSize: 11}}/>}
                                            >
                                                <Sparkline points={history[row.id] || []}/>
                                            </Button>
                                        ),
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

            {/* 线路延迟趋势图弹窗 */}
            <Modal
                title={`${t('tunservice.lineTrendTitle')} · ${lineTrendLineId}`}
                open={lineTrendOpen}
                onCancel={() => setLineTrendOpen(false)}
                footer={[
                    <Button key="close" type="primary" onClick={() => setLineTrendOpen(false)}>
                        {t('common.close')}
                    </Button>,
                ]}
                width={720}
                destroyOnClose
            >
                {lineTrendLoading ? (
                    <div style={{textAlign: 'center', padding: 40}}>{t('common.loading')}</div>
                ) : (
                    <ReactECharts
                        option={buildTrendOption(lineTrendData)}
                        style={{height: 280, width: '100%'}}
                        notMerge={true}
                    />
                )}
            </Modal>
        </div>
    )
}

export default TunService

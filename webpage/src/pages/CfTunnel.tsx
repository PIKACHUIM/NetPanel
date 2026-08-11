import React, {useEffect, useState} from 'react'
import {
    Alert,
    Badge,
    Button,
    Drawer,
    Form,
    Input,
    message,
    Modal,
    Popconfirm,
    Progress,
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
    DownloadOutlined,
    EditOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
    StopOutlined,
    UnorderedListOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {cftunnelApi} from '../api'
import StatusTag from '../components/StatusTag'

const {Option} = Select
const {Text, Link} = Typography

const modeColor: Record<string, string> = {
    quick: 'green',
    named: 'blue',
    token: 'orange',
}

const CfTunnel: React.FC = () => {
    const {t} = useTranslation()
    const [data, setData] = useState<any[]>([])
    const [loading, setLoading] = useState(false)
    const [modalOpen, setModalOpen] = useState(false)
    const [editing, setEditing] = useState<any>(null)
    const [logsOpen, setLogsOpen] = useState(false)
    const [logs, setLogs] = useState<string[]>([])
    const [logsLoading, setLogsLoading] = useState(false)
    const [form] = Form.useForm()
    
    // 二进制检测和下载
    const [binaryPath, setBinaryPath] = useState<string>('')
    const [binaryExists, setBinaryExists] = useState(false)
    const [downloadInfo, setDownloadInfo] = useState<any>(null)
    const [downloading, setDownloading] = useState(false)
    const [downloadProgress, setDownloadProgress] = useState(0)

    const load = async () => {
        setLoading(true)
        try {
            const res = await cftunnelApi.list()
            setData(res?.data || [])
        } finally {
            setLoading(false)
        }
    }

    const checkBinary = async () => {
        try {
            const pathRes = await cftunnelApi.getBinaryPath()
            const path = pathRes?.data?.binary_path || ''
            setBinaryPath(path)
            // 简单检测：如果路径存在且不为空，认为存在
            setBinaryExists(!!path)
            
            const infoRes = await cftunnelApi.getDownloadInfo()
            setDownloadInfo(infoRes?.data || null)
        } catch (e) {
            console.error('检测二进制失败:', e)
        }
    }

    useEffect(() => {
        load()
        checkBinary()
        // 周期刷新状态
        const timer = setInterval(load, 5000)
        return () => clearInterval(timer)
    }, [])

    const openCreate = () => {
        setEditing(null)
        form.resetFields()
        form.setFieldsValue({mode: 'quick', protocol: 'http', enable: false})
        setModalOpen(true)
    }

    const openEdit = (row: any) => {
        setEditing(row)
        form.resetFields()
        form.setFieldsValue(row)
        setModalOpen(true)
    }

    const submit = async () => {
        const values = await form.validateFields()
        try {
            if (editing) {
                await cftunnelApi.update(editing.id, {...editing, ...values})
                message.success(t('common.updated'))
            } else {
                await cftunnelApi.create(values)
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
            await cftunnelApi.start(id)
            message.success(t('common.started'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const stop = async (id: number) => {
        try {
            await cftunnelApi.stop(id)
            message.success(t('common.stopped'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const remove = async (id: number) => {
        try {
            await cftunnelApi.delete(id)
            message.success(t('common.deleted'))
            load()
        } catch (e: any) {
            message.error(e?.response?.data?.message || t('common.failed'))
        }
    }

    const showLogs = async (id: number) => {
        setLogsOpen(true)
        setLogsLoading(true)
        try {
            const res = await cftunnelApi.getLogs(id)
            setLogs(res?.data || [])
        } finally {
            setLogsLoading(false)
        }
    }

    const downloadBinary = async () => {
        if (!downloadInfo?.supported) {
            message.error(t('cftunnel.downloadFailed') + ': ' + t('common.unsupported'))
            return
        }

        setDownloading(true)
        setDownloadProgress(0)
        message.loading({content: t('cftunnel.downloadingBinary'), key: 'download', duration: 0})

        const url = cftunnelApi.downloadBinary()
        const eventSource = new EventSource(url)

        eventSource.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data)
                if (data.error) {
                    message.error({content: t('cftunnel.downloadFailed') + ': ' + data.error, key: 'download'})
                    setDownloading(false)
                    eventSource.close()
                } else if (data.done) {
                    message.success({content: t('cftunnel.downloadSuccess'), key: 'download'})
                    setDownloading(false)
                    setDownloadProgress(100)
                    eventSource.close()
                    checkBinary() // 重新检测
                } else if (data.percent !== undefined) {
                    setDownloadProgress(Math.round(data.percent))
                }
            } catch (e) {
                console.error('解析下载进度失败:', e)
            }
        }

        eventSource.onerror = () => {
            message.error({content: t('cftunnel.downloadFailed'), key: 'download'})
            setDownloading(false)
            eventSource.close()
        }
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
            title: t('cftunnel.mode'),
            dataIndex: 'mode',
            width: 100,
            render: (v: string) => <Tag color={modeColor[v] || 'default'}>{v}</Tag>,
        },
        {
            title: t('cftunnel.localUrl'),
            dataIndex: 'local_url',
            width: 200,
            render: (v: string) => v || '-',
        },
        {
            title: t('cftunnel.tunnelName'),
            dataIndex: 'tunnel_name',
            width: 160,
            render: (v: string) => v || '-',
        },
        {
            title: t('common.status'),
            dataIndex: 'status',
            width: 100,
            render: (v: string) => <StatusTag status={v}/>,
        },
        {
            title: t('common.enable'),
            dataIndex: 'enable',
            width: 80,
            render: (v: boolean) => (v ? <Badge status="success"/> : <Badge status="default"/>),
        },
        {
            title: t('common.actions'),
            key: 'actions',
            width: 220,
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
                    <Tooltip title={t('common.logs')}>
                        <Button size="small" icon={<UnorderedListOutlined/>} onClick={() => showLogs(row.id)}/>
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

    const mode = Form.useWatch('mode', form)

    return (
        <div>
            {!binaryExists && (
                <Alert
                    message={t('cftunnel.binaryNotFound')}
                    description={
                        <div>
                            <p>{t('cftunnel.binaryNotFoundDesc')}</p>
                            {binaryPath && <p><Text code>{binaryPath}</Text></p>}
                            {downloadInfo?.supported && (
                                <>
                                    <p>{t('cftunnel.autoDownloadSupported')}</p>
                                    <Button
                                        type="primary"
                                        icon={<DownloadOutlined/>}
                                        onClick={downloadBinary}
                                        loading={downloading}
                                        disabled={downloading}
                                    >
                                        {t('cftunnel.downloadBinary')}
                                    </Button>
                                    {downloading && (
                                        <Progress percent={downloadProgress} style={{marginTop: 12, maxWidth: 400}}/>
                                    )}
                                </>
                            )}
                            <p style={{marginTop: 12}}>
                                {t('cftunnel.manualDownloadHint')}
                                <Link href="https://github.com/cloudflare/cloudflared/releases" target="_blank" style={{marginLeft: 8}}>
                                    GitHub Releases
                                </Link>
                            </p>
                        </div>
                    }
                    type="warning"
                    showIcon
                    closable
                    style={{marginBottom: 16}}
                />
            )}

            <Space style={{marginBottom: 16}}>
                <Button type="primary" icon={<PlusOutlined/>} onClick={openCreate}>
                    {t('common.create')}
                </Button>
                <Button icon={<ReloadOutlined/>} onClick={load}>
                    {t('common.refresh')}
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
                width={560}
            >
                <Form form={form} layout="vertical">
                    <Form.Item name="name" label={t('common.name')} rules={[{required: true}]}>
                        <Input/>
                    </Form.Item>
                    <Form.Item name="mode" label={t('cftunnel.mode')} rules={[{required: true}]}>
                        <Select>
                            <Option value="quick">{t('cftunnel.modeQuick')}</Option>
                            <Option value="named">{t('cftunnel.modeNamed')}</Option>
                            <Option value="token">{t('cftunnel.modeToken')}</Option>
                        </Select>
                    </Form.Item>
                    <Form.Item
                        name="local_url"
                        label={t('cftunnel.localUrl')}
                        rules={mode === 'quick' ? [{required: true}] : []}
                        tooltip="http://127.0.0.1:8080"
                    >
                        <Input placeholder="http://127.0.0.1:8080"/>
                    </Form.Item>
                    {mode === 'named' && (
                        <>
                            <Form.Item name="tunnel_name" label={t('cftunnel.tunnelName')} rules={[{required: true}]}>
                                <Input placeholder="tunnel-name 或 UUID"/>
                            </Form.Item>
                            <Form.Item name="credentials_file" label={t('cftunnel.credentialsFile')}>
                                <Input placeholder="~/.cloudflared/<uuid>.json（可选）"/>
                            </Form.Item>
                            <Form.Item name="config_file" label={t('cftunnel.configFile')}>
                                <Input placeholder="config.yml 路径（可选，留空自动生成）"/>
                            </Form.Item>
                        </>
                    )}
                    {mode === 'token' && (
                        <Form.Item name="token" label={t('cftunnel.token')} rules={[{required: true}]}>
                            <Input.Password placeholder="eyJhIjoi..."/>
                        </Form.Item>
                    )}
                    <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                        <Switch/>
                    </Form.Item>
                    <Form.Item name="remark" label={t('common.remark')}>
                        <Input.TextArea rows={2}/>
                    </Form.Item>
                </Form>
            </Modal>

            <Drawer
                title={t('common.logs')}
                open={logsOpen}
                onClose={() => setLogsOpen(false)}
                width={640}
            >
                {logsLoading ? (
                    <Text type="secondary">Loading...</Text>
                ) : (
                    <pre style={{fontSize: 12, background: '#1f1f1f', color: '#d9d9d9', padding: 12, borderRadius: 6, maxHeight: '70vh', overflow: 'auto', whiteSpace: 'pre-wrap'}}>
                        {logs.length ? logs.join('\n') : t('common.noLogs')}
                    </pre>
                )}
            </Drawer>
        </div>
    )
}

export default CfTunnel

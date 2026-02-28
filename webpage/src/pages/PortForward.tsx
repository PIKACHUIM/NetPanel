import React, {useEffect, useState} from 'react'
import {
    Button,
    Col,
    Form,
    Input,
    InputNumber,
    message,
    Modal,
    Popconfirm,
    Row,
    Select,
    Space,
    Switch,
    Table,
    Tabs,
    Tag,
    Tooltip,
    Typography,
} from 'antd'
import {
    DeleteOutlined,
    EditOutlined,
    FileTextOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    StopOutlined
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {portForwardApi} from '../api'
import StatusTag from '../components/StatusTag'

const {Text} = Typography
const {Option} = Select

// 分组标题
const SectionTitle = ({children}: { children: React.ReactNode }) => (
    <div style={{display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 8px'}}>
        <div style={{width: 3, height: 14, background: '#1677ff', borderRadius: 2, flexShrink: 0}}/>
        <span style={{fontSize: 12, fontWeight: 600, color: '#595959', letterSpacing: '0.02em'}}>{children}</span>
        <div style={{flex: 1, height: 1, background: '#f0f0f0'}}/>
    </div>
)

interface PortForwardRule {
    id: number
    name: string
    enable: boolean
    listen_ip: string
    listen_port: number
    listen_port_type: string
    target_address: string
    target_port: number
    target_port_type: string
    protocol: string
    max_connections: number
    domain_cert_id: number
    status: string
    last_error: string
    remark: string
}

interface CertOption {
    id: number
    name: string
    domains: string
    status: string
}

const PORT_TYPE_COLOR: Record<string, string> = {
    tcp: 'blue',
    udp: 'orange',
    http: 'green',
    https: 'cyan',
    socks: 'purple',
    websocket: 'geekblue',
}

const PortForward: React.FC = () => {
    const {t} = useTranslation()
    const [data, setData] = useState<PortForwardRule[]>([])
    const [loading, setLoading] = useState(false)
    const [modalOpen, setModalOpen] = useState(false)
    const [editRecord, setEditRecord] = useState<PortForwardRule | null>(null)
    const [logModalOpen, setLogModalOpen] = useState(false)
    const [logs, setLogs] = useState<string[]>([])
    const [form] = Form.useForm()
    const [certs, setCerts] = useState<CertOption[]>([])
    const [listenPortType, setListenPortType] = useState<string>('tcp')

    const fetchCerts = async () => {
        try {
            const res: any = await portForwardApi.listCerts()
            setCerts(res.data || [])
        } catch {
            setCerts([])
        }
    }

    const fetchData = async () => {
        setLoading(true)
        try {
            const res: any = await portForwardApi.list()
            setData(res.data || [])
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchData()
    }, [])

    const handleCreate = () => {
        setEditRecord(null)
        form.resetFields()
        form.setFieldsValue({
            protocol: 'tcp',
            listen_ip: '0.0.0.0',
            listen_port_type: 'tcp',
            target_port_type: 'tcp',
            max_connections: 256,
            enable: true
        })
        setListenPortType('tcp')
        setModalOpen(true)
    }

    const handleEdit = (record: PortForwardRule) => {
        setEditRecord(record)
        form.setFieldsValue(record)
        setListenPortType(record.listen_port_type || 'tcp')
        if (record.listen_port_type === 'https') fetchCerts()
        setModalOpen(true)
    }

    const handleDelete = async (id: number) => {
        await portForwardApi.delete(id)
        message.success(t('common.success'))
        fetchData()
    }

    const handleToggle = async (record: PortForwardRule, checked: boolean) => {
        await portForwardApi.update(record.id, {...record, enable: checked})
        if (checked) {
            await portForwardApi.start(record.id)
        } else {
            await portForwardApi.stop(record.id)
        }
        fetchData()
    }

    const handleStart = async (id: number) => {
        await portForwardApi.start(id)
        message.success('已启动')
        fetchData()
    }

    const handleStop = async (id: number) => {
        await portForwardApi.stop(id)
        message.success('已停止')
        fetchData()
    }

    const handleViewLogs = async (id: number) => {
        const res: any = await portForwardApi.getLogs(id)
        setLogs(res.data || [])
        setLogModalOpen(true)
    }

    const handleSubmit = async () => {
        const values = await form.validateFields()
        if (editRecord) {
            await portForwardApi.update(editRecord.id, values)
        } else {
            await portForwardApi.create(values)
        }
        message.success(t('common.success'))
        setModalOpen(false)
        fetchData()
    }

    const columns = [
        {
            title: t('common.status'),
            dataIndex: 'status',
            width: 90,
            align: 'center' as const,
            render: (status: string) => <StatusTag status={status}/>,
        },
        {
            title: t('common.enable'),
            dataIndex: 'enable',
            width: 60,
            align: 'center' as const,
            render: (enable: boolean, record: PortForwardRule) => (
                <Switch
                    size="small"
                    checked={enable}
                    onChange={(checked) => handleToggle(record, checked)}
                />
            ),
        },
        {
            title: t('common.name'),
            dataIndex: 'name',
            width: 160,
            align: 'center' as const,
            render: (name: string, record: PortForwardRule) => (
                <div>
                    <Text strong style={{fontSize: 12}}>{name}</Text>
                    {record.remark && <div><Text type="secondary" style={{fontSize: 12}}>{record.remark}</Text></div>}
                </div>
            ),
        },
        {
            title: '监听地址',
            dataIndex: 'listen_ip',
            width: 130,
            align: 'center' as const,
            render: (v: string) => <Text code style={{fontSize: 12}}>{v || '0.0.0.0'}</Text>,
        },
        {
            title: '监听端口',
            dataIndex: 'listen_port',
            width: 120,
            align: 'center' as const,
            render: (v: number, record: PortForwardRule) => (
                <Space size={4}>
                    <Text code style={{fontSize: 12}}>{v}</Text>
                    <Tag color={PORT_TYPE_COLOR[record.listen_port_type?.toLowerCase()] || 'default'}
                         style={{fontSize: 11, padding: '0 4px'}}>{record.listen_port_type?.toUpperCase()}</Tag>
                </Space>
            ),
        },
        {
            title: '目标地址',
            dataIndex: 'target_address',
            width: 150,
            align: 'center' as const,
            render: (v: string) => <Text code style={{fontSize: 12}}>{v}</Text>,
        },
        {
            title: '目标端口',
            dataIndex: 'target_port',
            width: 120,
            align: 'center' as const,
            render: (v: number, record: PortForwardRule) => (
                <Space size={4}>
                    <Text code style={{fontSize: 12}}>{v}</Text>
                    <Tag color={PORT_TYPE_COLOR[record.target_port_type?.toLowerCase()] || 'default'}
                         style={{fontSize: 11, padding: '0 4px'}}>{record.target_port_type?.toUpperCase()}</Tag>
                </Space>
            ),
        },
        {
            title: t('portForward.maxConnections'),
            dataIndex: 'max_connections',
            width: 90,
            align: 'center' as const,
            render: (v: number) => <Text style={{fontSize: 12}}>{v ?? '-'}</Text>,
        },
        {
            title: t('common.action'),
            width: 160,
            align: 'center' as const,
            render: (_: any, record: PortForwardRule) => (
                <Space size={4}>
                    {record.status === 'running' ? (
                        <Tooltip title={t('common.stop')}>
                            <Button size="small" icon={<StopOutlined/>} onClick={() => handleStop(record.id)}/>
                        </Tooltip>
                    ) : (
                        <Tooltip title={t('common.start')}>
                            <Button size="small" type="primary" icon={<PlayCircleOutlined/>}
                                    onClick={() => handleStart(record.id)}/>
                        </Tooltip>
                    )}
                    <Tooltip title={t('common.viewLogs')}>
                        <Button size="small" icon={<FileTextOutlined/>} onClick={() => handleViewLogs(record.id)}/>
                    </Tooltip>
                    <Tooltip title={t('common.edit')}>
                        <Button size="small" icon={<EditOutlined/>} onClick={() => handleEdit(record)}/>
                    </Tooltip>
                    <Popconfirm title={t('common.deleteConfirm')} onConfirm={() => handleDelete(record.id)}>
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
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16}}>
                <Typography.Title level={4} style={{margin: 0}}>{t('portForward.title')}</Typography.Title>
                <Button type="primary" icon={<PlusOutlined/>} onClick={handleCreate}>
                    {t('common.create')}
                </Button>
            </div>

            <Table
                dataSource={data}
                columns={columns}
                rowKey="id"
                loading={loading}
                size="middle"
                style={{background: '#fff', borderRadius: 8}}
                pagination={{pageSize: 20, showSizeChanger: true}}
            />

            {/* 编辑弹窗 */}
            <Modal
                title={editRecord ? t('common.edit') : t('common.create')}
                open={modalOpen}
                onOk={handleSubmit}
                onCancel={() => setModalOpen(false)}
                width={580}
                destroyOnHidden
                styles={{body: {padding: '4px 24px 0'}}}
            >
                <Tabs
                    defaultActiveKey="forward"
                    style={{marginTop: -8}}
                    items={[{
                        key: 'forward',
                        label: t('portForward.forwardConfig'),
                        children: (
                            <Form form={form} layout="vertical" style={{paddingTop: 4}}>
                                {/* 基本配置 */}
                                <Row gutter={16}>
                                    <Col span={18}>
                                        <Form.Item name="name" label={t('common.name')}
                                                   rules={[{required: true, message: '请填写名称'}]}>
                                            <Input placeholder="规则名称" style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={6}>
                                        <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                                            <Switch/>
                                        </Form.Item>
                                    </Col>
                                </Row>
                                <Form.Item name="remark" label={t('common.remark')}>
                                    <Input.TextArea rows={2} placeholder="备注（可选）" style={{width: '100%'}}/>
                                </Form.Item>

                                {/* 监听设置 */}
                                <SectionTitle>{t('portForward.listenSettings')}</SectionTitle>
                                <Row gutter={16}>
                                    <Col span={8}>
                                        <Form.Item
                                            name="listen_ip"
                                            label={t('portForward.listenIP')}
                                            extra={<span
                                                style={{fontSize: 11}}>填 <code>0.0.0.0</code> 监听所有网卡</span>}
                                        >
                                            <Input placeholder="0.0.0.0" style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={8}>
                                        <Form.Item name="listen_port" label={t('portForward.listenPort')}
                                                   rules={[{required: true, message: '请填写端口'}]}>
                                            <InputNumber placeholder="端口" min={1} max={65535}
                                                         style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={8}>
                                        <Form.Item name="listen_port_type" label={t('portForward.listenPortType')}
                                                   rules={[{required: true}]}>
                                            <Select
                                                style={{width: '100%'}}
                                                onChange={(val: string) => {
                                                    setListenPortType(val)
                                                    if (val === 'https') fetchCerts()
                                                    else form.setFieldValue('domain_cert_id', undefined)
                                                }}
                                            >
                                                <Option value="tcp">TCP</Option>
                                                <Option value="udp">UDP</Option>
                                                <Option value="http">HTTP</Option>
                                                <Option value="https">HTTPS</Option>
                                                <Option value="socks">SOCKS</Option>
                                                <Option value="websocket">WEBSOCKET</Option>
                                            </Select>
                                        </Form.Item>
                                    </Col>
                                </Row>


                                {/* 目标设置 */}
                                <SectionTitle>{t('portForward.targetSettings')}</SectionTitle>
                                <Row gutter={16}>
                                    <Col span={8}>
                                        <Form.Item
                                            name="target_address"
                                            label={t('portForward.targetAddress')}
                                            rules={[{required: true, message: '请填写目标地址'}]}
                                            extra={<span style={{fontSize: 11}}>目标服务的 IP 或域名</span>}
                                        >
                                            <Input placeholder="目标IP/域名" style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={8}>
                                        <Form.Item name="target_port" label={t('portForward.targetPort')}
                                                   rules={[{required: true, message: '请填写端口'}]}>
                                            <InputNumber placeholder="端口" min={1} max={65535}
                                                         style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={8}>
                                        <Form.Item name="target_port_type" label={t('portForward.targetPortType')}
                                                   rules={[{required: true}]}>
                                            <Select style={{width: '100%'}}>
                                                <Option value="tcp">TCP</Option>
                                                <Option value="udp">UDP</Option>
                                                <Option value="http">HTTP</Option>
                                                <Option value="https">HTTPS</Option>
                                                <Option value="socks">SOCKS</Option>
                                                <Option value="websocket">WEBSOCKET</Option>
                                            </Select>
                                        </Form.Item>
                                    </Col>
                                </Row>

                                {/* 高级设置 */}
                                <SectionTitle>{t('portForward.advancedSettings')}</SectionTitle>
                                <Row gutter={16}>
                                    <Col span={8}>
                                        <Form.Item
                                            name="max_connections"
                                            label={t('portForward.maxConnections')}
                                            extra={<span style={{fontSize: 11}}>最大并发连接数</span>}
                                        >
                                            <InputNumber min={1} max={10000} style={{width: '100%'}}/>
                                        </Form.Item>
                                    </Col>
                                    <Col span={16}>
                                        {/* HTTPS 证书选择 */}
                                        {listenPortType === 'https' && (
                                            <Form.Item
                                                name="domain_cert_id"
                                                label="SSL 证书"
                                                extra={<span style={{fontSize: 11}}>选择「域名证书」中已签发的证书，留空则不加密</span>}
                                            >
                                                <Select
                                                    allowClear
                                                    placeholder="请选择 SSL 证书（可选）"
                                                    style={{width: '100%'}}
                                                    notFoundContent={certs.length === 0 ? <span style={{
                                                        fontSize: 12,
                                                        color: '#999'
                                                    }}>暂无可用证书，请先在「域名证书」中申请</span> : undefined}
                                                >
                                                    {certs.map(cert => (
                                                        <Option key={cert.id} value={cert.id}>
                                                            <span style={{fontWeight: 500}}>{cert.name}</span>
                                                            <span style={{
                                                                color: '#888',
                                                                fontSize: 11,
                                                                marginLeft: 6
                                                            }}>{cert.domains}</span>
                                                        </Option>
                                                    ))}
                                                </Select>
                                            </Form.Item>
                                        )}</Col>
                                </Row>

                            </Form>
                        ),
                    }]}
                />
            </Modal>

            {/* 日志弹窗 */}
            <Modal
                title={t('common.logs')}
                open={logModalOpen}
                onCancel={() => setLogModalOpen(false)}
                footer={null}
                width={700}
            >
                <div style={{
                    background: '#1a1a1a',
                    borderRadius: 6,
                    padding: 16,
                    maxHeight: 400,
                    overflow: 'auto',
                    fontFamily: 'monospace',
                    fontSize: 12,
                    color: '#d4d4d4',
                }}>
                    {logs.length > 0 ? logs.map((log, i) => (
                        <div key={i} style={{marginBottom: 2}}>{log}</div>
                    )) : <Text style={{color: '#666'}}>暂无日志</Text>}
                </div>
            </Modal>
        </div>
    )
}

export default PortForward

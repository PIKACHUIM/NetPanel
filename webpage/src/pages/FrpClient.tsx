import React, {useEffect, useState} from 'react'
import SectionTitle from '../components/SectionTitle'
import {FieldRow} from '../components/ConfigFields'
import {
    Badge,
    Button,
    Col,
    Drawer,
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
    ApiOutlined,
    DeleteOutlined,
    EditOutlined,
    HeartOutlined,
    LinkOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    ReloadOutlined,
    SettingOutlined,
    StopOutlined,
    ThunderboltOutlined,
    UnorderedListOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {frpcApi} from '../api'
import {useTunnelApi} from '../contexts/TunnelApiContext'
import StatusTag from '../components/StatusTag'
import request from '../api/request'
import { useTableStyle } from '../hooks/useTableStyle'

const {Option} = Select
const {Text} = Typography

// 连接协议选项
const TRANSPORT_OPTIONS = [
    {label: 'TCP（默认）', value: 'tcp'},
    {label: 'KCP（UDP加速）', value: 'kcp'},
    {label: 'QUIC', value: 'quic'},
    {label: 'WebSocket', value: 'websocket'},
    {label: 'WSS（WebSocket+TLS）', value: 'wss'},
]

// 认证方式选项
const AUTH_METHOD_OPTIONS = [
    {label: 'Token', value: 'token'},
    {label: 'OIDC', value: 'oidc'},
]

// 日志级别选项
const LOG_LEVEL_OPTIONS = [
    {label: 'trace', value: 'trace'},
    {label: 'debug', value: 'debug'},
    {label: 'info（推荐）', value: 'info'},
    {label: 'warn', value: 'warn'},
    {label: 'error', value: 'error'},
]

// 代理类型颜色
const proxyTypeColor: Record<string, string> = {
    tcp: 'blue', udp: 'green', http: 'orange', https: 'gold',
    stcp: 'purple', xtcp: 'magenta', tcpmux: 'cyan',
}

const FrpClient: React.FC = () => {
    const {t} = useTranslation()
    const tunnelCtx = useTunnelApi()
    const api = tunnelCtx?.api || frpcApi
    const isRemote = tunnelCtx?.isRemoteMode || false
    const tableStyle = useTableStyle()
    const [data, setData] = useState<any[]>([])
    const [loading, setLoading] = useState(false)
    const [modalOpen, setModalOpen] = useState(false)
    const [editRecord, setEditRecord] = useState<any>(null)
    const [form] = Form.useForm()

    // 代理管理
    const [proxyDrawerOpen, setProxyDrawerOpen] = useState(false)
    const [currentFrpc, setCurrentFrpc] = useState<any>(null)
    const [proxies, setProxies] = useState<any[]>([])
    const [proxyModalOpen, setProxyModalOpen] = useState(false)
    const [editProxy, setEditProxy] = useState<any>(null)
    const [proxyForm] = Form.useForm()
    const [proxyType, setProxyType] = useState('tcp')

    // 线路测速
    const [speedTestOpen, setSpeedTestOpen] = useState(false)
    const [speedTestData, setSpeedTestData] = useState<any[]>([])
    const [speedTesting, setSpeedTesting] = useState(false)

    const fetchData = async () => {
        setLoading(true)
        try {
            const res: any = await api.list()
            setData(res.data || [])
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchData()
    }, [])

    const fetchProxies = async (frpcId: number) => {
        const res: any = await request.get(`/v1/frpc/${frpcId}/proxies`)
        setProxies(res.data || [])
    }

    const runSpeedTest = async () => {
        setSpeedTestOpen(true)
        setSpeedTesting(true)
        setSpeedTestData([])
        try {
            const res: any = await request.get('/v1/frpc/speedtest')
            setSpeedTestData(res.data || [])
        } catch (e: any) {
            message.error(e?.message || t('frp.speedTestFailed'))
        } finally {
            setSpeedTesting(false)
        }
    }

    // 测速结果表格列
    const speedTestColumns = [
        {title: t('frp.clientName'), dataIndex: 'name'},
        {title: t('frp.serverAddr'), dataIndex: 'server_addr', render: (_: any, r: any) => `${r.server_addr}:${r.server_port}`},
        {title: t('frp.latency'), dataIndex: 'latency_ms', render: (v: number, r: any) => (r.status === 'ok' ? `${v} ms` : '-')},
        {title: t('frp.status'), dataIndex: 'status', render: (v: string) => (
            <Tag color={v === 'ok' ? 'green' : 'red'}>{v === 'ok' ? t('frp.reachable') : t('frp.unreachable')}</Tag>
        )},
    ]

    const openProxyDrawer = (record: any) => {
        setCurrentFrpc(record)
        fetchProxies(record.id)
        setProxyDrawerOpen(true)
    }

    const handleCreate = () => {
        setEditRecord(null)
        form.resetFields()
        form.setFieldsValue({
            enable: true,
            server_port: 7000,
            log_level: 'info',
            transport_protocol: 'tcp',
            tls_enable: true,
            pool_count: 5,
            tcp_mux: true,
            login_fail_exit: false,
            heartbeat_interval: 30,
            heartbeat_timeout: 90,
            dial_server_timeout: 10,
            udp_packet_size: 1500,
            auth_method: 'token',
        })
        setModalOpen(true)
    }

    const handleEdit = (record: any) => {
        setEditRecord(record)
        form.setFieldsValue(record)
        setModalOpen(true)
    }

    const handleSubmit = async () => {
        const values = await form.validateFields()
        if (editRecord) {
            await api.update(editRecord.id, values)
        } else {
            await api.create(values)
        }
        message.success(t('common.success'))
        setModalOpen(false)
        fetchData()
        tunnelCtx?.onRefresh?.()
    }

    const handleToggle = async (record: any, checked: boolean) => {
        await api.update(record.id, {...record, enable: checked})
        checked ? await api.start(record.id) : await api.stop(record.id)
        fetchData()
    }

    // 代理操作
    const handleCreateProxy = () => {
        setEditProxy(null)
        proxyForm.resetFields()
        proxyForm.setFieldsValue({
            type: 'tcp',
            local_ip: '127.0.0.1',
            enable: true,
            health_check_type: '',
            health_check_timeout_s: 3,
            health_check_max_failed: 3,
            health_check_interval_s: 10,
            bandwidth_limit_mode: 'client',
            multiplexer: 'httpconnect',
        })
        setProxyType('tcp')
        setProxyModalOpen(true)
    }

    const handleEditProxy = (proxy: any) => {
        setEditProxy(proxy)
        const type = proxy.type || 'tcp'
        proxyForm.setFieldsValue({...proxy, type})
        setProxyType(type)
        setProxyModalOpen(true)
    }

    const handleSubmitProxy = async () => {
        const values = await proxyForm.validateFields()
        if (editProxy) {
            await request.put(`/v1/frpc/${currentFrpc.id}/proxies/${editProxy.id}`, values)
        } else {
            await request.post(`/v1/frpc/${currentFrpc.id}/proxies`, values)
        }
        message.success(t('common.success'))
        setProxyModalOpen(false)
        fetchProxies(currentFrpc.id)
    }

    const handleDeleteProxy = async (proxyId: number) => {
        await request.delete(`/v1/frpc/${currentFrpc.id}/proxies/${proxyId}`)
        fetchProxies(currentFrpc.id)
    }

    const columns = [
        {
            title: t('common.status'), dataIndex: 'status', width: 100,
            render: (s: string) => <StatusTag status={s}/>,
        },
        {
            title: t('common.enable'), dataIndex: 'enable', width: 80,
            render: (v: boolean, r: any) => (
                <Switch size="small" checked={v} onChange={(c) => handleToggle(r, c)}/>
            ),
        },
        {
            title: t('common.name'), dataIndex: 'name',
            render: (name: string, r: any) => (
                <div>
                    <Text strong>{name}</Text>
                    {r.remark && <div><Text type="secondary" style={{fontSize: 12}}>{r.remark}</Text></div>}
                </div>
            ),
        },
        {
            title: t('frp.serverAddr'),
            render: (_: any, r: any) => (
                <Text code style={{fontSize: 12}}>{r.server_addr}:{r.server_port}</Text>
            ),
        },
        {
            title: '协议', dataIndex: 'transport_protocol', width: 80,
            render: (v: string) => v ? <Tag color="blue" style={{fontSize: 11}}>{v?.toUpperCase()}</Tag> :
                <Tag style={{fontSize: 11}}>TCP</Tag>,
        },
        {
            title: 'Token', dataIndex: 'token',
            render: (v: string) => v ? <Text type="secondary">••••••</Text> : '-',
        },
        {
            title: 'TLS', dataIndex: 'tls_enable', width: 60,
            render: (v: boolean) => v ? <Tag color="green" style={{fontSize: 11}}>TLS</Tag> : '-',
        },
        {
            title: '代理', width: 80,
            render: (_: any, r: any) => (
                <Button
                    size="small" type="link" icon={<UnorderedListOutlined/>}
                    onClick={() => openProxyDrawer(r)}
                    style={{padding: 0}}
                >
                    管理
                </Button>
            ),
        },
        {
            title: t('common.action'), width: 160,
            render: (_: any, r: any) => (
                <Space size={4}>
                    {r.status === 'running'
                        ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined/>}
                                                                    onClick={async () => {
                                                                        await api.stop(r.id);
                                                                        fetchData()
                                                                    }}/></Tooltip>
                        : <Tooltip title={t('common.start')}><Button size="small" type="primary"
                                                                     icon={<PlayCircleOutlined/>} onClick={async () => {
                            await api.start(r.id);
                            fetchData()
                        }}/></Tooltip>
                    }
                    <Tooltip title={t('common.restart')}>
                        <Button size="small" icon={<ReloadOutlined/>} onClick={async () => {
                            await api.restart(r.id);
                            fetchData()
                        }}/>
                    </Tooltip>
                    <Tooltip title={t('common.edit')}>
                        <Button size="small" icon={<EditOutlined/>} onClick={() => handleEdit(r)}/>
                    </Tooltip>
                    <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => {
                        await api.delete(r.id);
                        fetchData()
                    }}>
                        <Tooltip title={t('common.delete')}>
                            <Button size="small" danger icon={<DeleteOutlined/>}/>
                        </Tooltip>
                    </Popconfirm>
                </Space>
            ),
        },
    ]

    // 代理列表列
    const proxyColumns = [
        {
            title: '代理名称', dataIndex: 'name',
            render: (name: string, r: any) => (
                <Space>
                    <Tag color={proxyTypeColor[r.type] || 'default'}>{r.type?.toUpperCase()}</Tag>
                    <Text>{name}</Text>
                    {r.remark && <Text type="secondary" style={{fontSize: 11}}>({r.remark})</Text>}
                </Space>
            ),
        },
        {
            title: '本地地址',
            render: (_: any, r: any) => (
                <Text code style={{fontSize: 12}}>{r.local_ip}:{r.local_port}</Text>
            ),
        },
        {
            title: '远程端口 / 域名',
            render: (_: any, r: any) => {
                if (r.type === 'http' || r.type === 'https') {
                    return <Text code style={{fontSize: 12}}>{r.custom_domains || r.subdomain || '-'}</Text>
                }
                return r.remote_port ? <Text code style={{fontSize: 12}}>:{r.remote_port}</Text> : '-'
            },
        },
        {
            title: '选项', width: 120,
            render: (_: any, r: any) => (
                <Space size={4} wrap>
                    {r.use_encryption && <Tag color="green" style={{fontSize: 10}}>加密</Tag>}
                    {r.use_compression && <Tag color="blue" style={{fontSize: 10}}>压缩</Tag>}
                    {r.health_check_type && <Tag color="orange" style={{fontSize: 10}}>健康检查</Tag>}
                    {r.load_balancer_group && <Tag color="purple" style={{fontSize: 10}}>负载均衡</Tag>}
                </Space>
            ),
        },
        {
            title: t('common.enable'), dataIndex: 'enable', width: 70,
            render: (v: boolean) => <Badge status={v ? 'success' : 'default'} text={v ? '启用' : '禁用'}/>,
        },
        {
            title: t('common.action'), width: 100,
            render: (_: any, r: any) => (
                <Space size={4}>
                    <Button size="small" icon={<EditOutlined/>} onClick={() => handleEditProxy(r)}/>
                    <Popconfirm title={t('common.deleteConfirm')} onConfirm={() => handleDeleteProxy(r.id)}>
                        <Button size="small" danger icon={<DeleteOutlined/>}/>
                    </Popconfirm>
                </Space>
            ),
        },
    ]

    // ===== frpc 主配置 Tab =====
    const tabBasic = (
        <>
            <FieldRow
                fields={[
                    {name: 'name', label: '名称', span: 10, placeholder: '客户端名称', rules: [{required: true, message: '请填写名称'}]},
                    {name: 'user', label: '用户名', span: 10, placeholder: '留空不设置', hint: '代理名 user.proxyName'},
                    {name: 'enable', label: '启用', type: 'switch', span: 4},
                ]}
            />
            <SectionTitle>服务器连接</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'server_addr', label: '服务器地址', placeholder: 'frp.example.com', rules: [{required: true, message: '请填写服务器地址'}], hint: 'FRP 服务端的 IP 或域名'},
                    {name: 'server_port', label: '端口', type: 'number', min: 1, max: 65535, placeholder: '7000', rules: [{required: true, message: '请填写端口'}], hint: '默认 7000'},
                    {name: 'transport_protocol', label: '协议', type: 'select', span: 8, options: TRANSPORT_OPTIONS, hint: '连接协议'},
                ]}
            />
            <SectionTitle>认证</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'auth_method', label: '认证方式', type: 'select', options: AUTH_METHOD_OPTIONS, hint: '默认 token'},
                    {name: 'token', label: '认证 Token', type: 'password', span: 16, placeholder: '留空不认证', hint: '需与服务端配置一致，留空不认证'},
                ]}
            />
            <SectionTitle>TLS / 日志</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'tls_enable', label: 'TLS', type: 'switch', span: 4, hint: '加密传输'},
                    {name: 'log_level', label: '日志级别', type: 'select', span: 8, options: LOG_LEVEL_OPTIONS},
                    {name: 'pool_count', label: '连接池大小', type: 'number', min: 0, max: 100, placeholder: '5', hint: '预建连接数，默认 5'},
                    {name: 'udp_packet_size', label: 'UDP 包大小', type: 'number', min: 512, max: 65535, placeholder: '1500', hint: '默认 1500，需与服务端一致'},
                ]}
            />
        </>
    )

    const tabConnection = (
        <>
            <SectionTitle>心跳 &amp; 超时</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'heartbeat_interval', label: '心跳间隔（秒）', type: 'number', min: -1, max: 3600, placeholder: '30', hint: '默认 30，-1 禁用'},
                    {name: 'heartbeat_timeout', label: '心跳超时（秒）', type: 'number', min: 1, max: 3600, placeholder: '90', hint: '默认 90'},
                    {name: 'dial_server_timeout', label: '连接超时（秒）', type: 'number', min: 1, max: 300, placeholder: '10', hint: '连接服务端超时，默认 10'},
                ]}
            />
            <SectionTitle>TCP 多路复用</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'tcp_mux', label: 'TCP 多路复用', type: 'switch', span: 6, hint: '默认启用'},
                    {name: 'tcp_mux_keepalive_interval', label: 'tcp_mux 心跳间隔（秒）', type: 'number', span: 9, min: 0, max: 3600, placeholder: '0', hint: '0 表示不设置'},
                    {name: 'dial_server_keepalive', label: 'TCP keepalive 间隔（秒）', type: 'number', span: 9, min: 0, max: 3600, placeholder: '0', hint: '底层 TCP 保活，0 不设置'},
                ]}
            />
            <SectionTitle>网络 &amp; 代理</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'proxy_url', label: '代理地址', placeholder: 'http://127.0.0.1:8080', hint: '格式：http://user:pass@host:port 或 socks5://...'},
                    {name: 'connect_server_local_ip', label: '绑定本地 IP', placeholder: '留空自动选择', hint: '连接服务端时绑定的本地 IP，多网卡时使用'},
                ]}
            />
            <FieldRow
                fields={[
                    {name: 'nat_hole_stun_server', label: 'STUN 服务器', placeholder: 'stun.easyvoip.com:3478', hint: 'xtcp 打洞使用，默认 stun.easyvoip.com:3478'},
                    {name: 'dns_server', label: 'DNS 服务器', placeholder: '8.8.8.8', hint: '自定义 DNS，留空使用系统默认'},
                ]}
            />
            <SectionTitle>Web 管理</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'web_server_port', label: '管理端口', type: 'number', min: 1, max: 65535, placeholder: '7400', hint: '留空不启用'},
                    {name: 'web_server_user', label: '用户名', placeholder: 'admin'},
                    {name: 'web_server_password', label: '密码', type: 'password', placeholder: '管理密码'},
                ]}
            />
            <SectionTitle>其他</SectionTitle>
            <FieldRow
                fields={[
                    {name: 'login_fail_exit', label: '登录失败退出', type: 'switch', hint: '首次登录失败是否退出进程，建议关闭以保持隧道自动重连'},
                    {name: 'remark', label: '备注', type: 'textarea', rows: 2, span: 16, placeholder: '备注（可选）'},
                ]}
            />
        </>
    )

    // ===== 代理编辑 Tab：基本信息 =====
    const proxyTabBasic = () => (
        <>
            {/* TCP / UDP：本地服务 + 远程端口 */}
            {(proxyType === 'tcp' || proxyType === 'udp') && (
                <>
                    <SectionTitle>本地服务</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="local_ip"
                                label="本地 IP"
                                rules={[{required: true, message: '请填写本地 IP'}]}
                                extra={<span style={{fontSize: 11}}>要转发的本地服务地址</span>}
                            >
                                <Input placeholder="127.0.0.1" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="local_port"
                                label="本地端口"
                                rules={[{required: true, message: '请填写端口'}]}
                            >
                                <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="8080"/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <SectionTitle>远程映射</SectionTitle>
                    <Form.Item
                        name="remote_port"
                        label="远程端口"
                        rules={[{required: true, message: '请填写远程端口'}]}
                        extra={<span style={{fontSize: 11}}>服务端对外暴露的端口，填 0 由服务端随机分配</span>}
                    >
                        <InputNumber min={0} max={65535} style={{width: '100%'}} placeholder="6000"/>
                    </Form.Item>
                </>
            )}

            {/* HTTP / HTTPS：本地服务 + 域名配置 + HTTP 高级 */}
            {(proxyType === 'http' || proxyType === 'https') && (
                <>
                    <SectionTitle>本地服务</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="local_ip"
                                label="本地 IP"
                                rules={[{required: true, message: '请填写本地 IP'}]}
                            >
                                <Input placeholder="127.0.0.1" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="local_port"
                                label="本地端口"
                                rules={[{required: true, message: '请填写端口'}]}
                            >
                                <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="8080"/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <SectionTitle>域名配置</SectionTitle>
                    <Form.Item
                        name="custom_domains"
                        label="自定义域名"
                        extra={<span style={{fontSize: 11}}>多个域名用逗号分隔，需解析到服务端 IP</span>}
                    >
                        <Input placeholder="example.com,www.example.com" style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item
                        name="subdomain"
                        label="子域名前缀"
                        extra={<span
                            style={{fontSize: 11}}>需服务端配置 subdomain_host，如填 myapp 则访问 myapp.frps.com</span>}
                    >
                        <Input placeholder="myapp（不含主域名）" style={{width: '100%'}}/>
                    </Form.Item>
                    <SectionTitle>HTTP 高级</SectionTitle>
                    <Row gutter={16}>
                        <Col span={12}>
                            <Form.Item name="http_user" label="Basic Auth 用户名"
                                       extra={<span style={{fontSize: 11}}>访问此代理需要 HTTP Basic Auth</span>}>
                                <Input placeholder="admin" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={12}>
                            <Form.Item name="http_password" label="Basic Auth 密码">
                                <Input.Password placeholder="HTTP Basic Auth 密码" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <Row gutter={16}>
                        <Col span={12}>
                            <Form.Item
                                name="locations"
                                label="路径匹配（locations）"
                                extra={<span style={{fontSize: 11}}>多个路径用逗号分隔，如 /,/api</span>}
                            >
                                <Input placeholder="/,/api" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={12}>
                            <Form.Item
                                name="host_header_rewrite"
                                label="Host 头重写"
                                extra={<span style={{fontSize: 11}}>将请求的 Host 头替换为指定值</span>}
                            >
                                <Input placeholder="example.com" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <Form.Item
                        name="request_headers"
                        label="自定义请求头"
                        extra={<span style={{fontSize: 11}}>格式：key=value，多个用换行分隔</span>}
                    >
                        <Input.TextArea rows={2} placeholder={'x-from-where=frp\nx-custom-header=value'}
                                        style={{width: '100%'}}/>
                    </Form.Item>
                </>
            )}

            {/* STCP / SUDP：本地服务 + 私密配置 */}
            {(proxyType === 'stcp' || proxyType === 'sudp' || proxyType === 'xudp') && (
                <>
                    <SectionTitle>本地服务</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="local_ip"
                                label="本地 IP"
                                rules={[{required: true, message: '请填写本地 IP'}]}
                            >
                                <Input placeholder="127.0.0.1" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="local_port"
                                label="本地端口"
                                rules={[{required: true, message: '请填写端口'}]}
                            >
                                <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="8080"/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <SectionTitle>私密配置</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="secret_key"
                                label="密钥（secretKey）"
                                rules={[{required: true, message: '请填写密钥'}]}
                                extra={<span style={{fontSize: 11}}>访问者端需使用相同密钥才能连接</span>}
                            >
                                <Input.Password placeholder="访问密钥" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="allow_users"
                                label="允许用户"
                                extra={<span style={{fontSize: 11}}>* 表示所有用户</span>}
                            >
                                <Input placeholder="* 或 user1,user2" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                </>
            )}

            {/* XTCP：本地服务 + P2P 配置 */}
            {proxyType === 'xtcp' && (
                <>
                    <SectionTitle>本地服务</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="local_ip"
                                label="本地 IP"
                                rules={[{required: true, message: '请填写本地 IP'}]}
                            >
                                <Input placeholder="127.0.0.1" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="local_port"
                                label="本地端口"
                                rules={[{required: true, message: '请填写端口'}]}
                            >
                                <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="8080"/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <SectionTitle>P2P 打洞配置</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="secret_key"
                                label="密钥（secretKey）"
                                rules={[{required: true, message: '请填写密钥'}]}
                                extra={<span style={{fontSize: 11}}>访问者端需使用相同密钥，P2P 直连不经过服务端</span>}
                            >
                                <Input.Password placeholder="访问密钥" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="allow_users"
                                label="允许用户"
                                extra={<span style={{fontSize: 11}}>* 表示所有用户</span>}
                            >
                                <Input placeholder="* 或 user1,user2" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                </>
            )}

            {/* TCPMUX：本地服务 + 域名配置 */}
            {proxyType === 'tcpmux' && (
                <>
                    <SectionTitle>本地服务</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="local_ip"
                                label="本地 IP"
                                rules={[{required: true, message: '请填写本地 IP'}]}
                            >
                                <Input placeholder="127.0.0.1" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="local_port"
                                label="本地端口"
                                rules={[{required: true, message: '请填写端口'}]}
                            >
                                <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="8080"/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <SectionTitle>TCPMUX 配置</SectionTitle>
                    <Form.Item
                        name="multiplexer"
                        label="多路复用器"
                        extra={<span
                            style={{fontSize: 11}}>目前仅支持 httpconnect，通过 HTTP CONNECT 方法复用连接</span>}
                    >
                        <Select style={{width: '100%'}} defaultValue="httpconnect">
                            <Option value="httpconnect">httpconnect</Option>
                        </Select>
                    </Form.Item>
                    <Form.Item
                        name="custom_domains"
                        label="自定义域名"
                        extra={<span style={{fontSize: 11}}>多个域名用逗号分隔，需解析到服务端 IP</span>}
                    >
                        <Input placeholder="example.com,www.example.com" style={{width: '100%'}}/>
                    </Form.Item>
                    <Form.Item
                        name="subdomain"
                        label="子域名前缀"
                        extra={<span style={{fontSize: 11}}>需服务端配置 subdomain_host</span>}
                    >
                        <Input placeholder="myapp（不含主域名）" style={{width: '100%'}}/>
                    </Form.Item>
                    <Row gutter={16}>
                        <Col span={12}>
                            <Form.Item name="http_user" label="HTTP 认证用户名">
                                <Input placeholder="admin" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={12}>
                            <Form.Item name="http_password" label="HTTP 认证密码">
                                <Input.Password placeholder="HTTP Basic Auth 密码" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                </>
            )}

            <Form.Item name="remark" label="备注">
                <Input placeholder="备注（可选）" style={{width: '100%'}}/>
            </Form.Item>
        </>
    )

    // ===== 代理编辑 Tab：传输选项 =====
    const proxyTabTransport = () => (
        <>
            <SectionTitle>基础传输</SectionTitle>
            <Row gutter={16}>
                <Col span={8}>
                    <Form.Item name="use_encryption" label="加密传输" valuePropName="checked"
                               extra={<span style={{fontSize: 11}}>端到端加密</span>}>
                        <Switch/>
                    </Form.Item>
                </Col>
                <Col span={8}>
                    <Form.Item name="use_compression" label="压缩传输" valuePropName="checked"
                               extra={<span style={{fontSize: 11}}>减少带宽占用</span>}>
                        <Switch/>
                    </Form.Item>
                </Col>
            </Row>

            <SectionTitle>带宽限制</SectionTitle>
            <Row gutter={16}>
                <Col span={12}>
                    <Form.Item
                        name="bandwidth_limit"
                        label="带宽限制"
                        extra={<span style={{fontSize: 11}}>如 1MB、512KB，留空不限制</span>}
                    >
                        <Input placeholder="1MB" style={{width: '100%'}}/>
                    </Form.Item>
                </Col>
                <Col span={12}>
                    <Form.Item
                        name="bandwidth_limit_mode"
                        label="限速方向"
                        extra={<span style={{fontSize: 11}}>client：客户端限速；server：服务端限速</span>}
                    >
                        <Select style={{width: '100%'}} allowClear placeholder="默认 client">
                            <Option value="client">client（客户端）</Option>
                            <Option value="server">server（服务端）</Option>
                        </Select>
                    </Form.Item>
                </Col>
            </Row>

        </>
    )

    // ===== 代理编辑 Tab：健康检查 =====
    const proxyTabHealth = () => (
        <>
            <Form.Item
                name="health_check_type"
                label="健康检查类型"
                extra={<span style={{fontSize: 11}}>留空不启用健康检查</span>}
            >
                <Select style={{width: '100%'}} allowClear placeholder="不启用">
                    <Option value="tcp">TCP（连接检测）</Option>
                    <Option value="http">HTTP（接口检测）</Option>
                </Select>
            </Form.Item>

            <Row gutter={16}>
                <Col span={8}>
                    <Form.Item
                        name="health_check_timeout_s"
                        label="超时（秒）"
                        extra={<span style={{fontSize: 11}}>默认 3s</span>}
                    >
                        <InputNumber min={1} max={60} style={{width: '100%'}} placeholder="3"/>
                    </Form.Item>
                </Col>
                <Col span={8}>
                    <Form.Item
                        name="health_check_max_failed"
                        label="最大失败次数"
                        extra={<span style={{fontSize: 11}}>默认 3 次</span>}
                    >
                        <InputNumber min={1} max={20} style={{width: '100%'}} placeholder="3"/>
                    </Form.Item>
                </Col>
                <Col span={8}>
                    <Form.Item
                        name="health_check_interval_s"
                        label="检查间隔（秒）"
                        extra={<span style={{fontSize: 11}}>默认 10s</span>}
                    >
                        <InputNumber min={1} max={300} style={{width: '100%'}} placeholder="10"/>
                    </Form.Item>
                </Col>
            </Row>

            <Form.Item
                name="health_check_path"
                label="HTTP 检查路径"
                extra={<span style={{fontSize: 11}}>仅 HTTP 类型有效，如 /health</span>}
            >
                <Input placeholder="/health" style={{width: '100%'}}/>
            </Form.Item>
        </>
    )

    // ===== 代理编辑 Tab：负载均衡 =====
    const proxyTabLB = () => (
        <>
            <Form.Item
                name="load_balancer_group"
                label="负载均衡组名"
                extra={<span style={{fontSize: 11}}>同组代理将被 frps 负载均衡，留空不参与</span>}
            >
                <Input placeholder="test_group" style={{width: '100%'}}/>
            </Form.Item>
            <Form.Item
                name="load_balancer_group_key"
                label="组密钥"
                extra={<span style={{fontSize: 11}}>同组内所有代理需使用相同的组密钥</span>}
            >
                <Input.Password placeholder="group_key" style={{width: '100%'}}/>
            </Form.Item>
            <Form.Item name="remark" label="备注">
                <Input placeholder="备注（可选）" style={{width: '100%'}}/>
            </Form.Item>
        </>
    )

    return (
        <div>
            {!isRemote && (
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16}}>
                <Typography.Title level={4} style={{margin: 0}}>{t('frp.clientTitle')}</Typography.Title>
                <Space>
                    <Button
                        icon={<LinkOutlined/>}
                        href="https://github.com/fatedier/frp"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        FRP {t('common.officialSite')}
                    </Button>
                    <Button icon={<ThunderboltOutlined/>} loading={speedTesting} onClick={runSpeedTest}>
                        {t('frp.speedTest')}
                    </Button>
                    <Button type="primary" icon={<PlusOutlined/>} onClick={handleCreate}>{t('common.create')}</Button>
                </Space>
            </div>
            )}
            {isRemote && (
            <div style={{display: 'flex', justifyContent: 'flex-end', marginBottom: 12}}>
                <Button type="primary" icon={<PlusOutlined/>} onClick={handleCreate}>{t('common.create')}</Button>
            </div>
            )}

            <Table
                dataSource={data} columns={columns} rowKey="id" loading={loading}
                size="middle" style={tableStyle}
                pagination={{pageSize: 20, showSizeChanger: true}}
            />

            {/* 线路测速结果弹窗 */}
            <Modal
                title={t('frp.speedTestResult')}
                open={speedTestOpen}
                onCancel={() => setSpeedTestOpen(false)}
                footer={null}
                width={720}
            >
                <Table
                    dataSource={speedTestData} rowKey="id" loading={speedTesting}
                    size="small" pagination={false} columns={speedTestColumns}
                    locale={{emptyText: t('frp.noFrpc')}}
                />
            </Modal>

            {/* frpc 编辑弹窗 */}
            <Modal
                title={editRecord ? t('common.edit') : t('common.create')}
                open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
                width={640} destroyOnHidden
                styles={{body: {padding: '4px 24px 0'}}}
            >
                <Form form={form} layout="vertical" style={{paddingTop: 4}}>
                    <Tabs
                        size="small"
                        items={[
                            {key: 'basic', label: <span><SettingOutlined/> 连接设置</span>, children: tabBasic},
                            {key: 'connection', label: <span><LinkOutlined/> 高级设置</span>, children: tabConnection},
                        ]}
                    />
                </Form>
            </Modal>

            {/* 代理管理抽屉 */}
            <Drawer
                title={`代理管理 - ${currentFrpc?.name || ''}`}
                open={proxyDrawerOpen}
                onClose={() => setProxyDrawerOpen(false)}
                width={760}
                extra={
                    <Button type="primary" icon={<PlusOutlined/>} onClick={handleCreateProxy}>
                        {t('frp.addProxy')}
                    </Button>
                }
            >
                <Table
                    dataSource={proxies} columns={proxyColumns} rowKey="id"
                    size="small" pagination={false}
                />
            </Drawer>

            {/* 代理编辑弹窗 */}
            <Modal
                title={editProxy ? '编辑代理' : '添加代理'}
                open={proxyModalOpen} onOk={handleSubmitProxy}
                onCancel={() => setProxyModalOpen(false)} width={620} destroyOnHidden
                styles={{body: {padding: '4px 24px 0'}}}
            >
            <Form form={proxyForm} layout="vertical" style={{paddingTop: 4}}
                      onValuesChange={(changed) => {
                          if (changed.type !== undefined) setProxyType(changed.type)
                      }}>
                    {/* 代理基本信息：名称、类型、启用 —— 固定在 Tab 外部，确保类型切换能触发重渲染 */}
                    <Row gutter={16} style={{marginTop: 4}}>
                        <Col span={12}>
                            <Form.Item
                                name="name"
                                label="代理名称"
                                rules={[{required: true, message: '请填写代理名称'}]}
                                extra={<span style={{fontSize: 11}}>全局唯一，建议使用有意义的名称</span>}
                            >
                                <Input placeholder="my-proxy" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item name="type" label="代理类型" rules={[{required: true}]}>
                                <Select style={{width: '100%'}}>
                                    <Option value="tcp">TCP</Option>
                                    <Option value="udp">UDP</Option>
                                    <Option value="http">HTTP</Option>
                                    <Option value="https">HTTPS</Option>
                                    <Option value="stcp">STCP（私密 TCP）</Option>
                                    <Option value="sudp">SUDP（私密 UDP）</Option>
                                    <Option value="xtcp">XTCP（P2P 打洞）</Option>
                                    <Option value="tcpmux">TCPMUX（HTTP CONNECT）</Option>
                                </Select>
                            </Form.Item>
                        </Col>
                        <Col span={4}>
                            <Form.Item name="enable" label="启用" valuePropName="checked">
                                <Switch/>
                            </Form.Item>
                        </Col>
                    </Row>
                    <Tabs
                        size="small"
                        key={proxyType}
                        destroyInactiveTabPane
                        items={[
                            {key: 'basic', label: <span><SettingOutlined/> 基本配置</span>, children: proxyTabBasic()},
                            {
                                key: 'transport',
                                label: <span><ThunderboltOutlined/> 传输选项</span>,
                                children: proxyTabTransport()
                            },
                            {key: 'health', label: <span><HeartOutlined/> 健康检查</span>, children: proxyTabHealth()},
                            {key: 'lb', label: <span><ApiOutlined/> 负载均衡</span>, children: proxyTabLB()},
                        ]}
                    />
                </Form>
            </Modal>
        </div>
    )
}

export default FrpClient

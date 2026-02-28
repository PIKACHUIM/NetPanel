import React, {useEffect, useState} from 'react'
import {
    AutoComplete,
    Button,
    Card,
    Col,
    Form,
    Input,
    InputNumber,
    message,
    Modal,
    Popconfirm,
    Radio,
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
    InfoCircleOutlined,
    PlayCircleOutlined,
    PlusOutlined,
    SettingOutlined,
    StopOutlined,
} from '@ant-design/icons'
import {useTranslation} from 'react-i18next'
import {callbackTaskApi, stunApi} from '../api'
import StatusTag from '../components/StatusTag'

const {Text} = Typography
const {Option} = Select

// 预制 STUN 服务器列表
const STUN_SERVER_LIST = [
    'stun.miwifi.com:3478',
    'stun.12connect.com:3478',
    'stun.12voip.com:3478',
    'stun.1und1.de:3478',
    'stun.2talk.co.nz:3478',
    'stun.2talk.com:3478',
    'stun.3clogic.com:3478',
    'stun.3cx.com:3478',
    'stun.a-mm.tv:3478',
    'stun.aa.net.uk:3478',
    'stun.acrobits.cz:3478',
    'stun.actionvoip.com:3478',
    'stun.advfn.com:3478',
    'stun.aeta-audio.com:3478',
    'stun.aeta.com:3478',
    'stun.alltel.com.au:3478',
    'stun.altar.com.pl:3478',
    'stun.annatel.net:3478',
    'stun.antisip.com:3478',
    'stun.arbuz.ru:3478',
    'stun.avigora.com:3478',
    'stun.avigora.fr:3478',
    'stun.awa-shima.com:3478',
    'stun.awt.be:3478',
    'stun.b2b2c.ca:3478',
    'stun.bahnhof.net:3478',
    'stun.barracuda.com:3478',
    'stun.bluesip.net:3478',
    'stun.bmwgs.cz:3478',
    'stun.botonakis.com:3478',
    'stun.budgetphone.nl:3478',
    'stun.budgetsip.com:3478',
    'stun.cablenet-as.net:3478',
    'stun.callromania.ro:3478',
    'stun.callwithus.com:3478',
    'stun.cbsys.net:3478',
    'stun.chathelp.ru:3478',
    'stun.cheapvoip.com:3478',
    'stun.ciktel.com:3478',
    'stun.cloopen.com:3478',
    'stun.colouredlines.com.au:3478',
    'stun.comfi.com:3478',
    'stun.commpeak.com:3478',
    'stun.comtube.com:3478',
    'stun.comtube.ru:3478',
    'stun.cope.es:3478',
    'stun.counterpath.com:3478',
    'stun.counterpath.net:3478',
    'stun.cryptonit.net:3478',
    'stun.darioflaccovio.it:3478',
    'stun.datamanagement.it:3478',
    'stun.dcalling.de:3478',
    'stun.decanet.fr:3478',
    'stun.demos.ru:3478',
    'stun.develz.org:3478',
    'stun.dingaling.ca:3478',
    'stun.doublerobotics.com:3478',
    'stun.drogon.net:3478',
    'stun.duocom.es:3478',
    'stun.dus.net:3478',
    'stun.e-fon.ch:3478',
    'stun.easybell.de:3478',
    'stun.easycall.pl:3478',
    'stun.easyvoip.com:3478',
    'stun.efficace-factory.com:3478',
    'stun.einsundeins.com:3478',
    'stun.einsundeins.de:3478',
    'stun.ekiga.net:3478',
    'stun.epygi.com:3478',
    'stun.etoilediese.fr:3478',
    'stun.eyeball.com:3478',
    'stun.faktortel.com.au:3478',
    'stun.freecall.com:3478',
    'stun.freeswitch.org:3478',
    'stun.freevoipdeal.com:3478',
    'stun.fuzemeeting.com:3478',
    'stun.gmx.de:3478',
    'stun.gmx.net:3478',
    'stun.gradwell.com:3478',
    'stun.halonet.pl:3478',
    'stun.hellonanu.com:3478',
    'stun.hoiio.com:3478',
    'stun.hosteurope.de:3478',
    'stun.ideasip.com:3478',
    'stun.imesh.com:3478',
    'stun.infra.net:3478',
    'stun.internetcalls.com:3478',
    'stun.intervoip.com:3478',
    'stun.ipcomms.net:3478',
    'stun.ipfire.org:3478',
    'stun.ippi.fr:3478',
    'stun.ipshka.com:3478',
    'stun.iptel.org:3478',
    'stun.irian.at:3478',
    'stun.it1.hr:3478',
    'stun.ivao.aero:3478',
    'stun.jappix.com:3478',
    'stun.jumblo.com:3478',
    'stun.justvoip.com:3478',
    'stun.kanet.ru:3478',
    'stun.kiwilink.co.nz:3478',
    'stun.kundenserver.de:3478',
    'stun.l.google.com:19302',
    'stun.linea7.net:3478',
    'stun.linphone.org:3478',
    'stun.liveo.fr:3478',
    'stun.lowratevoip.com:3478',
    'stun.lugosoft.com:3478',
    'stun.lundimatin.fr:3478',
    'stun.magnet.ie:3478',
    'stun.manle.com:3478',
    'stun.mgn.ru:3478',
    'stun.mit.de:3478',
    'stun.mitake.com.tw:3478',
    'stun.modulus.gr:3478',
    'stun.mozcom.com:3478',
    'stun.myvoiptraffic.com:3478',
    'stun.mywatson.it:3478',
    'stun.nas.net:3478',
    'stun.neotel.co.za:3478',
    'stun.netappel.com:3478',
    'stun.netappel.fr:3478',
    'stun.netgsm.com.tr:3478',
    'stun.nfon.net:3478',
    'stun.noblogs.org:3478',
    'stun.noc.ams-ix.net:3478',
    'stun.node4.co.uk:3478',
    'stun.nonoh.net:3478',
    'stun.nottingham.ac.uk:3478',
    'stun.nova.is:3478',
    'stun.nventure.com:3478',
    'stun.on.net.mk:3478',
    'stun.ooma.com:3478',
    'stun.ooonet.ru:3478',
    'stun.oriontelekom.rs:3478',
    'stun.outland-net.de:3478',
    'stun.ozekiphone.com:3478',
    'stun.patlive.com:3478',
    'stun.personal-voip.de:3478',
    'stun.petcube.com:3478',
    'stun.phone.com:3478',
    'stun.phoneserve.com:3478',
    'stun.pjsip.org:3478',
    'stun.poivy.com:3478',
    'stun.powerpbx.org:3478',
    'stun.powervoip.com:3478',
    'stun.ppdi.com:3478',
    'stun.prizee.com:3478',
    'stun.qq.com:3478',
    'stun.qvod.com:3478',
    'stun.rackco.com:3478',
    'stun.rapidnet.de:3478',
    'stun.rb-net.com:3478',
    'stun.refint.net:3478',
    'stun.remote-learner.net:3478',
    'stun.rixtelecom.se:3478',
    'stun.rockenstein.de:3478',
    'stun.rolmail.net:3478',
    'stun.rounds.com:3478',
    'stun.rynga.com:3478',
    'stun.samsungsmartcam.com:3478',
    'stun.schlund.de:3478',
    'stun.services.mozilla.com:3478',
    'stun.sigmavoip.com:3478',
    'stun.sip.us:3478',
    'stun.sipdiscount.com:3478',
    'stun.siplogin.de:3478',
    'stun.sipnet.net:3478',
    'stun.sipnet.ru:3478',
    'stun.siportal.it:3478',
    'stun.sippeer.dk:3478',
    'stun.siptraffic.com:3478',
    'stun.skylink.ru:3478',
    'stun.sma.de:3478',
    'stun.smartvoip.com:3478',
    'stun.smsdiscount.com:3478',
    'stun.snafu.de:3478',
    'stun.softjoys.com:3478',
    'stun.solcon.nl:3478',
    'stun.solnet.ch:3478',
    'stun.sonetel.com:3478',
    'stun.sonetel.net:3478',
    'stun.sovtest.ru:3478',
    'stun.speedy.com.ar:3478',
    'stun.spokn.com:3478',
    'stun.srce.hr:3478',
    'stun.ssl7.net:3478',
    'stun.stunprotocol.org:3478',
    'stun.symform.com:3478',
    'stun.symplicity.com:3478',
    'stun.sysadminman.net:3478',
    'stun.t-online.de:3478',
    'stun.tagan.ru:3478',
    'stun.tatneft.ru:3478',
    'stun.teachercreated.com:3478',
    'stun.tel.lu:3478',
    'stun.telbo.com:3478',
    'stun.telefacil.com:3478',
    'stun.tis-dialog.ru:3478',
    'stun.tng.de:3478',
    'stun.twt.it:3478',
    'stun.u-blox.com:3478',
    'stun.ucallweconn.net:3478',
    'stun.ucsb.edu:3478',
    'stun.ucw.cz:3478',
    'stun.uls.co.za:3478',
    'stun.unseen.is:3478',
    'stun.usfamily.net:3478',
    'stun.veoh.com:3478',
    'stun.vidyo.com:3478',
    'stun.vipgroup.net:3478',
    'stun.virtual-call.com:3478',
    'stun.viva.gr:3478',
    'stun.vivox.com:3478',
    'stun.vline.com:3478',
    'stun.vo.lu:3478',
    'stun.vodafone.ro:3478',
    'stun.voicetrading.com:3478',
    'stun.voip.aebc.com:3478',
    'stun.voip.blackberry.com:3478',
    'stun.voip.eutelia.it:3478',
    'stun.voiparound.com:3478',
    'stun.voipblast.com:3478',
    'stun.voipbuster.com:3478',
    'stun.voipbusterpro.com:3478',
    'stun.voipcheap.co.uk:3478',
    'stun.voipcheap.com:3478',
    'stun.voipfibre.com:3478',
    'stun.voipgain.com:3478',
    'stun.voipgate.com:3478',
    'stun.voipinfocenter.com:3478',
    'stun.voipplanet.nl:3478',
    'stun.voippro.com:3478',
    'stun.voipraider.com:3478',
    'stun.voipstunt.com:3478',
    'stun.voipwise.com:3478',
    'stun.voipzoom.com:3478',
    'stun.vopium.com:3478',
    'stun.voxgratia.org:3478',
    'stun.voxox.com:3478',
    'stun.voys.nl:3478',
    'stun.voztele.com:3478',
    'stun.vyke.com:3478',
    'stun.webcalldirect.com:3478',
    'stun.whoi.edu:3478',
    'stun.wifirst.net:3478',
    'stun.wwdl.net:3478',
    'stun.xs4all.nl:3478',
    'stun.xtratelecom.es:3478',
    'stun.yesss.at:3478',
    'stun.zadarma.com:3478',
    'stun.zadv.com:3478',
    'stun.zoiper.com:3478',
    'stun1.faktortel.com.au:3478',
    'stun1.l.google.com:19302',
    'stun1.voiceeclipse.net:3478',
    'stun2.l.google.com:19302',
    'stun3.l.google.com:19302',
    'stun4.l.google.com:19302',
    'stunserver.org:3478',
    'stun.nextcloud.com:443',
    'stun.flashdance.cx:3478',
]

const STUN_SERVER_OPTIONS = STUN_SERVER_LIST.map(v => ({value: v, label: v}))

// 分组标题（与 EasyTier 保持一致）
const SectionTitle = ({children}: { children: React.ReactNode }) => (
    <div style={{display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 8px'}}>
        <div style={{width: 3, height: 14, background: '#1677ff', borderRadius: 2, flexShrink: 0}}/>
        <span style={{fontSize: 12, fontWeight: 600, color: '#595959', letterSpacing: '0.02em'}}>{children}</span>
        <div style={{flex: 1, height: 1, background: '#f0f0f0'}}/>
    </div>
)

const Stun: React.FC = () => {
    const {t} = useTranslation()
    const [data, setData] = useState<any[]>([])
    const [loading, setLoading] = useState(false)
    const [modalOpen, setModalOpen] = useState(false)
    const [detailOpen, setDetailOpen] = useState(false)
    const [detailRecord, setDetailRecord] = useState<any>(null)
    const [editRecord, setEditRecord] = useState<any>(null)
    const [callbackTasks, setCallbackTasks] = useState<any[]>([])
    const [form] = Form.useForm()
    const [forwardMode, setForwardMode] = useState<string>('proxy')
    // 'none' | 'upnp' | 'natmap'
    const [natHelper, setNatHelper] = useState<string>('none')

    const fetchData = async () => {
        setLoading(true)
        try {
            const res: any = await stunApi.list()
            setData(res.data || [])
        } finally {
            setLoading(false)
        }
    }

    const fetchCallbackTasks = async () => {
        try {
            const res: any = await callbackTaskApi.list()
            setCallbackTasks(res.data || [])
        } catch { /* 忽略错误 */
        }
    }

    useEffect(() => {
        fetchData();
        fetchCallbackTasks()
    }, [])

    const handleCreate = () => {
        setEditRecord(null)
        form.resetFields()
        form.setFieldsValue({
            enable: true,
            stun_server: 'stun.miwifi.com:3478',
            forward_mode: 'proxy',
            target_protocol: 'tcp',
            nat_helper: 'none',
            natmap_keepalive: 30,
            disable_validation: false,
        })
        setForwardMode('proxy')
        setNatHelper('none')
        setModalOpen(true)
    }

    const handleEdit = (record: any) => {
        setEditRecord(record)
        // 兼容旧数据：将 use_upnp/use_natmap 转换为 nat_helper
        const natHelperVal = record.use_natmap ? 'natmap' : record.use_upnp ? 'upnp' : 'none'
        form.setFieldsValue({...record, nat_helper: natHelperVal})
        setForwardMode(record.forward_mode || 'proxy')
        setNatHelper(natHelperVal)
        setModalOpen(true)
    }

    const handleSubmit = async () => {
        const values = await form.validateFields()
        if (editRecord) {
            await stunApi.update(editRecord.id, values)
        } else {
            await stunApi.create(values)
        }
        message.success(t('common.success'))
        setModalOpen(false)
        fetchData()
    }

    const handleToggle = async (record: any, checked: boolean) => {
        await stunApi.update(record.id, {...record, enable: checked})
        checked ? await stunApi.start(record.id) : await stunApi.stop(record.id)
        fetchData()
    }

    // 根据 callback_task_id 查找任务名称
    const getCallbackTaskName = (id: number) => {
        if (!id) return null
        const task = callbackTasks.find(t => t.id === id)
        return task ? task.name : `#${id}`
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
            title: t('stun.stunServer'), dataIndex: 'stun_server',
            render: (v: string) => <Text code style={{fontSize: 12}}>{v}</Text>,
        },
        {
            title: t('stun.currentIP'),
            render: (_: any, r: any) => r.current_ip
                ? <Text code style={{color: '#52c41a'}}>{r.current_ip}:{r.current_port}</Text>
                : <Text type="secondary">-</Text>,
        },
        {
            title: 'STUN状态', dataIndex: 'stun_status', width: 100,
            render: (v: string, r: any) => {
                if (r.status !== 'running') return <Tag color="default">-</Tag>
                if (v === 'penetrating') return <Tag color="success">穿透中</Tag>
                if (v === 'timeout') return <Tag color="warning">超时</Tag>
                if (v === 'failed') return <Tag color="error">失败</Tag>
                return <Tag color="processing">检测中</Tag>
            },
        },
        {
            title: t('stun.natType'), dataIndex: 'nat_type',
            render: (v: string) => v ? <Tag color="blue">{v}</Tag> : '-',
        },
        {
            title: '模式/选项', width: 160,
            render: (_: any, r: any) => (
                <Space size={4} wrap>
                    {r.forward_mode === 'direct'
                        ? <Tag color="orange" style={{fontSize: 11}}>直接转发</Tag>
                        : <Tag color="blue" style={{fontSize: 11}}>本机代理</Tag>}
                    {r.target_protocol &&
                        <Tag color="geekblue" style={{fontSize: 11}}>{r.target_protocol?.toUpperCase()}</Tag>}
                    {r.use_upnp && <Tag color="purple" style={{fontSize: 11}}>UPnP</Tag>}
                    {r.use_natmap && <Tag color="orange" style={{fontSize: 11}}>NATMAP</Tag>}
                    {r.callback_task_id ? (
                        <Tag color="cyan" style={{fontSize: 11}}>
                            回调: {getCallbackTaskName(r.callback_task_id)}
                        </Tag>
                    ) : null}
                </Space>
            ),
        },
        {
            title: t('common.action'), width: 180,
            render: (_: any, r: any) => (
                <Space size={4}>
                    {r.status === 'running'
                        ? <Tooltip title={t('common.stop')}><Button size="small" icon={<StopOutlined/>}
                                                                    onClick={async () => {
                                                                        await stunApi.stop(r.id);
                                                                        fetchData()
                                                                    }}/></Tooltip>
                        : <Tooltip title={t('common.start')}><Button size="small" type="primary"
                                                                     icon={<PlayCircleOutlined/>} onClick={async () => {
                            await stunApi.start(r.id);
                            fetchData()
                        }}/></Tooltip>
                    }
                    <Tooltip title="详情">
                        <Button size="small" icon={<InfoCircleOutlined/>} onClick={() => {
                            setDetailRecord(r);
                            setDetailOpen(true)
                        }}/>
                    </Tooltip>
                    <Tooltip title={t('common.edit')}>
                        <Button size="small" icon={<EditOutlined/>} onClick={() => handleEdit(r)}/>
                    </Tooltip>
                    <Popconfirm title={t('common.deleteConfirm')} onConfirm={async () => {
                        await stunApi.delete(r.id);
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

    // ===== Tab 1: 基本配置（含转发目标） =====
    const tabBasic = (
        <>
            <Row gutter={16}>
                <Col span={18}>
                    <Form.Item name="name" label={t('common.name')} rules={[{required: true, message: '请填写名称'}]}>
                        <Input placeholder="规则名称" style={{width: '100%'}}/>
                    </Form.Item>
                </Col>
                <Col span={6}>
                    <Form.Item name="enable" label={t('common.enable')} valuePropName="checked">
                        <Switch/>
                    </Form.Item>
                </Col>
            </Row>

            <Row gutter={16}>
                <Col span={14}>
                    <Form.Item name="stun_server"
                        label={t('stun.stunServer')}
                        extra={<span style={{fontSize: 11}}>可从列表选择或直接输入，格式：<code>host:port</code></span>}
                    >
                        <AutoComplete
                            options={STUN_SERVER_OPTIONS}
                            placeholder="stun.miwifi.com:3478"
                            style={{width: '100%'}}
                            filterOption={(inputValue, option) =>
                                option!.value.toLowerCase().includes(inputValue.toLowerCase())
                            }
                            allowClear
                        />
                    </Form.Item>
                </Col>
                <Col span={10}>
                    <Form.Item
                        name="disable_validation"
                        label="禁用有效性检测"
                        valuePropName="checked"
                        tooltip="勾选后跳过 NAT 类型检测，仅获取 STUN 映射地址，适用于检测失败但实际可穿透的场景"
                    >
                        <Switch/>
                    </Form.Item>
                </Col>
            </Row>

            <SectionTitle>转发模式</SectionTitle>
            <Row gutter={16}>
                <Col span={forwardMode === 'proxy' ? 14 : 24}>
                    <Form.Item
                        name="forward_mode"
                        label={t('stun.forwardMode')}
                        extra={
                            <span style={{fontSize: 11}}>
                                {forwardMode === 'proxy'
                                    ? '本机代理：外网 → 本地端口 → 目标端口'
                                    : '直接转发：外网 → 目标端口'}
                            </span>
                        }
                    >
                        <Select style={{width: '100%'}}>
                            <Option value="proxy">
                                <Space>
                                    <Tag color="blue" style={{margin: 0}}>本机代理</Tag>
                                    <Text type="secondary" style={{fontSize: 12}}>外网 → 本机端口 → 目标</Text>
                                </Space>
                            </Option>
                            <Option value="direct">
                                <Space>
                                    <Tag color="orange" style={{margin: 0}}>直接转发</Tag>
                                    <Text type="secondary" style={{fontSize: 12}}>外网 → UPnP/NAT → 目标</Text>
                                </Space>
                            </Option>
                        </Select>
                    </Form.Item>
                </Col>
                {forwardMode === 'proxy' && (
                    <Col span={10}>
                        <Form.Item
                            name="listen_port"
                            label={t('stun.listenPort')}
                            extra={<span style={{fontSize: 11}}>不填则随机分配</span>}
                        >
                            <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="留空随机分配"/>
                        </Form.Item>
                    </Col>
                )}
            </Row>

            <SectionTitle>转发目标</SectionTitle>
            <Row gutter={16}>
                <Col span={14}>
                    <Form.Item
                        name="target_address"
                        label={t('stun.targetAddress')}
                        extra={<span style={{fontSize: 11}}>穿透成功后将流量转发到此地址</span>}
                    >
                        <Input placeholder="目标 IP / 域名" style={{width: '100%'}}/>
                    </Form.Item>
                </Col>
                <Col span={6}>
                    <Form.Item name="target_port" label={t('stun.targetPort')}>
                        <InputNumber min={1} max={65535} style={{width: '100%'}} placeholder="端口"/>
                    </Form.Item>
                </Col>
                <Col span={4}>
                    <Form.Item name="target_protocol" label={t('stun.protocol')}>
                        <Select style={{width: '100%'}}>
                            <Option value="tcp">TCP</Option>
                            <Option value="udp">UDP</Option>
                        </Select>
                    </Form.Item>
                </Col>
            </Row>


        </>
    )

    // ===== Tab 3: 高级选项 =====
    const tabAdvanced = (
        <>
            <SectionTitle>NAT 穿透辅助</SectionTitle>
            {forwardMode === 'direct' && (
                <div style={{
                    marginBottom: 8,
                    padding: '6px 10px',
                    background: '#fff7e6',
                    border: '1px solid #ffd591',
                    borderRadius: 6,
                    fontSize: 12,
                    color: '#d46b08'
                }}>
                    直接转发模式下，必须启用 UPnP 或 NATMAP 其中之一
                </div>
            )}
            <Form.Item
                name="nat_helper"
                style={{marginBottom: 8}}
                rules={[
                    {
                        validator: (_, value) => {
                            if (forwardMode === 'direct' && (!value || value === 'none')) {
                                return Promise.reject('直接转发模式下必须选择 UPnP 或 NATMAP')
                            }
                            return Promise.resolve()
                        },
                    },
                ]}
            >
                <Radio.Group>
                    <Radio value="none" disabled={forwardMode === 'direct'}>不使用</Radio>
                    <Radio value="upnp">{t('stun.useUpnp')}</Radio>
                    <Radio value="natmap">{t('stun.useNatmap')}</Radio>
                </Radio.Group>
            </Form.Item>

            {/* UPnP 详细配置 */}
            {natHelper === 'upnp' && (
                <>
                    <SectionTitle>UPnP 配置</SectionTitle>
                    <Row gutter={16}>
                        <Col span={12}>
                            <Form.Item
                                name="upnp_server_ip"
                                label={t('stun.upnpServerIP')}
                                extra={<span style={{fontSize: 11}}>留空自动发现路由器 UPnP 服务</span>}
                            >
                                <Input placeholder="192.168.1.1（可选）" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={12}>
                            <Form.Item
                                name="upnp_external_ip"
                                label={t('stun.upnpExternalIP')}
                                extra={<span style={{fontSize: 11}}>指定外部 IP，留空自动获取</span>}
                            >
                                <Input placeholder="外部 IP（可选）" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                    </Row>
                </>
            )}

            {/* NATMAP 详细配置 */}
            {natHelper === 'natmap' && (
                <>
                    <SectionTitle>NATMAP 配置</SectionTitle>
                    <Row gutter={16}>
                        <Col span={16}>
                            <Form.Item
                                name="natmap_server_addr"
                                label={t('stun.natmapServerAddr')}
                                extra={<span
                                    style={{fontSize: 11}}>NATMAP 服务器地址，格式：<code>host:port</code></span>}
                            >
                                <Input placeholder="stun.miwifi.com:3478" style={{width: '100%'}}/>
                            </Form.Item>
                        </Col>
                        <Col span={8}>
                            <Form.Item
                                name="natmap_keepalive"
                                label={t('stun.natmapKeepalive')}
                                extra={<span style={{fontSize: 11}}>保活间隔（秒）</span>}
                            >
                                <InputNumber min={5} max={300} style={{width: '100%'}} placeholder="30"/>
                            </Form.Item>
                        </Col>
                    </Row>
                </>
            )}
            <SectionTitle>回调任务</SectionTitle>
            <Form.Item
                name="callback_task_id"
                label="触发回调"
                extra={<span style={{fontSize: 11}}>IP 变化时触发指定回调任务，留空不触发</span>}
            >
                <Select
                    allowClear
                    placeholder="选择回调任务（可选）"
                    style={{width: '100%'}}
                    options={callbackTasks.map(task => ({
                        label: (
                            <Space size={6}>
                                <span>{task.name}</span>
                                {task.remark && <Text type="secondary" style={{fontSize: 11}}>{task.remark}</Text>}
                            </Space>
                        ),
                        value: task.id,
                    }))}
                    optionFilterProp="label"
                    showSearch
                    filterOption={(input, option) =>
                        String(option?.value ?? '').toLowerCase().includes(input.toLowerCase()) ||
                        callbackTasks.find(t => t.id === option?.value)?.name?.toLowerCase().includes(input.toLowerCase())
                    }
                />
            </Form.Item>

            <Form.Item name="remark" label={t('common.remark')}>
                <Input.TextArea rows={2} placeholder="备注（可选）" style={{width: '100%'}}/>
            </Form.Item>

        </>
    )

    return (
        <div>
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16}}>
                <Typography.Title level={4} style={{margin: 0}}>{t('stun.title')}</Typography.Title>
                <Button type="primary" icon={<PlusOutlined/>} onClick={handleCreate}>{t('common.create')}</Button>
            </div>

            <Table
                dataSource={data} columns={columns} rowKey="id" loading={loading}
                size="middle" style={{background: '#fff', borderRadius: 8}}
                pagination={{pageSize: 20, showSizeChanger: true}}
            />

            {/* 编辑弹窗 */}
            <Modal
                title={editRecord ? t('common.edit') : t('common.create')}
                open={modalOpen} onOk={handleSubmit} onCancel={() => setModalOpen(false)}
                width={580} destroyOnHidden
                styles={{body: {padding: '4px 24px 0'}}}
            >
                <Form form={form} layout="vertical" style={{paddingTop: 4}}
                      onValuesChange={(changed) => {
                          if ('forward_mode' in changed) setForwardMode(changed.forward_mode)
                          if ('nat_helper' in changed) setNatHelper(changed.nat_helper)
                      }}
                >
                    <Tabs
                        size="small"
                        items={[
                            {key: 'basic', label: <span><SettingOutlined/> 基本配置</span>, children: tabBasic},
                            {
                                key: 'advanced',
                                label: <span><InfoCircleOutlined/> 高级选项</span>,
                                children: tabAdvanced
                            },
                        ]}
                    />
                </Form>
            </Modal>

            {/* 详情弹窗 */}
            <Modal
                title="STUN 穿透详情"
                open={detailOpen} onCancel={() => setDetailOpen(false)} footer={null} width={480}
            >
                {detailRecord && (
                    <div style={{padding: '8px 0'}}>
                        <Card size="small"
                              style={{marginBottom: 12, background: '#f6ffed', border: '1px solid #b7eb8f'}}>
                            <Row gutter={16}>
                                <Col span={12}>
                                    <Text type="secondary">当前公网 IP</Text>
                                    <div><Text strong style={{color: '#52c41a'}}>{detailRecord.current_ip || '-'}</Text>
                                    </div>
                                </Col>
                                <Col span={12}>
                                    <Text type="secondary">当前端口</Text>
                                    <div><Text strong
                                               style={{color: '#52c41a'}}>{detailRecord.current_port || '-'}</Text>
                                    </div>
                                </Col>
                            </Row>
                        </Card>
                        <Row gutter={[16, 8]}>
                            <Col span={12}>
                                <Text type="secondary">NAT 类型</Text>
                                <div><Tag color="blue">{detailRecord.nat_type || '未知'}</Tag></div>
                            </Col>
                            <Col span={12}>
                                <Text type="secondary">STUN 服务器</Text>
                                <div><Text code style={{fontSize: 12}}>{detailRecord.stun_server}</Text></div>
                            </Col>
                            <Col span={12}>
                                <Text type="secondary">转发模式</Text>
                                <div>
                                    {detailRecord.forward_mode === 'direct'
                                        ? <Tag color="orange">直接转发</Tag>
                                        : <Tag color="blue">本机代理</Tag>}
                                </div>
                            </Col>
                            <Col span={12}>
                                <Text type="secondary">协议</Text>
                                <div><Tag color="geekblue">{(detailRecord.target_protocol || 'tcp').toUpperCase()}</Tag>
                                </div>
                            </Col>
                            {detailRecord.forward_mode === 'proxy' && detailRecord.listen_port ? (
                                <Col span={12}>
                                    <Text type="secondary">本机监听端口</Text>
                                    <div><Text code>{detailRecord.listen_port}</Text></div>
                                </Col>
                            ) : null}
                            <Col span={12}>
                                <Text type="secondary">NAT 辅助</Text>
                                <div>
                                    {detailRecord.use_natmap
                                        ? <Tag color="orange">NATMAP{detailRecord.natmap_server_addr ? ` (${detailRecord.natmap_server_addr})` : ''}</Tag>
                                        : detailRecord.use_upnp
                                            ? <Tag color="purple">UPnP{detailRecord.upnp_server_ip ? ` (${detailRecord.upnp_server_ip})` : ''}</Tag>
                                            : <Tag color="default">不使用</Tag>}
                                </div>
                            </Col>
                            {detailRecord.callback_task_id ? (
                                <Col span={24}>
                                    <Text type="secondary">回调任务</Text>
                                    <div><Tag color="cyan">{getCallbackTaskName(detailRecord.callback_task_id)}</Tag>
                                    </div>
                                </Col>
                            ) : null}
                            {detailRecord.remark && (
                                <Col span={24}>
                                    <Text type="secondary">备注</Text>
                                    <div><Text style={{fontSize: 12}}>{detailRecord.remark}</Text></div>
                                </Col>
                            )}
                            {detailRecord.last_error && (
                                <Col span={24}>
                                    <Text type="secondary">最后错误</Text>
                                    <div><Text type="danger" style={{fontSize: 12}}>{detailRecord.last_error}</Text>
                                    </div>
                                </Col>
                            )}
                        </Row>
                    </div>
                )}
            </Modal>
        </div>
    )
}

export default Stun

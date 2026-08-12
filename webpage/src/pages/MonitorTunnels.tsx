import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Switch,
  Select,
  App,
  Tag,
  Tooltip,
  Divider,
  Row,
  Col,
  Statistic,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ApiOutlined,
  LinkOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { monitorApi } from '../api';
import { useTranslation } from 'react-i18next';

const { Option } = Select;
const { TextArea } = Input;

interface TunnelBinding {
  id: number;
  server_id: number;
  tunnel_type: string;
  tunnel_id: number;
  auto_config: boolean;
  tunnel_status: string;
  remark: string;
  server_name?: string;
  tunnel_name?: string;
  tunnel_config?: any;
}

interface Server {
  id: number;
  name: string;
  display_name: string;
  is_online: boolean;
}

interface TunnelConfig {
  id: number;
  name: string;
  enable: boolean;
  type: string;
  [key: string]: any;
}

const MonitorTunnels: React.FC = () => {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [bindings, setBindings] = useState<TunnelBinding[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [tunnelConfigs, setTunnelConfigs] = useState<Record<string, TunnelConfig[]>>({
    frp: [],
    nps: [],
    easytier: [],
    cftunnel: [],
    wireguard: [],
  });
  const [modalVisible, setModalVisible] = useState(false);
  const [editingBinding, setEditingBinding] = useState<TunnelBinding | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    setLoading(true);
    try {
      // monitorApi 的响应拦截器已解包 data,运行时返回裸数组,此处仅修正 TS 类型
      const [bindingsRes, serversRes, frpRes, npsRes, easytierRes, cftunnelRes, wireguardRes] = await Promise.all([
        monitorApi.listTunnelBindings() as unknown as TunnelBinding[],
        monitorApi.listServers() as unknown as Server[],
        fetch('/api/v1/frpc').then(res => res.json()).catch(() => ({ data: [] })),
        fetch('/api/v1/nps/client').then(res => res.json()).catch(() => ({ data: [] })),
        fetch('/api/v1/easytier/client').then(res => res.json()).catch(() => ({ data: [] })),
        fetch('/api/v1/cftunnel').then(res => res.json()).catch(() => ({ data: [] })),
        fetch('/api/v1/wireguard').then(res => res.json()).catch(() => ({ data: [] })),
      ]);

      // fetch 接口返回 {code, data} 包装,统一解包
      const unwrap = (res: any) => res?.data ?? res ?? [];
      const frpList: TunnelConfig[] = unwrap(frpRes);
      const npsList: TunnelConfig[] = unwrap(npsRes);
      const easytierList: TunnelConfig[] = unwrap(easytierRes);
      const cftunnelList: TunnelConfig[] = unwrap(cftunnelRes);
      const wireguardList: TunnelConfig[] = unwrap(wireguardRes);

      // 关联服务器名称和隧道配置
      const enrichedBindings = bindingsRes.map((binding: TunnelBinding) => {
        const server = serversRes.find((s: Server) => s.id === binding.server_id);
        let tunnelConfig = null;
        let tunnelName = '';
        
        switch (binding.tunnel_type) {
          case 'frp':
            tunnelConfig = frpList.find((t: TunnelConfig) => t.id === binding.tunnel_id);
            break;
          case 'nps':
            tunnelConfig = npsList.find((t: TunnelConfig) => t.id === binding.tunnel_id);
            break;
          case 'easytier':
            tunnelConfig = easytierList.find((t: TunnelConfig) => t.id === binding.tunnel_id);
            break;
          case 'cftunnel':
            tunnelConfig = cftunnelList.find((t: TunnelConfig) => t.id === binding.tunnel_id);
            break;
          case 'wireguard':
            tunnelConfig = wireguardList.find((t: TunnelConfig) => t.id === binding.tunnel_id);
            break;
        }
        
        tunnelName = tunnelConfig?.name || `${binding.tunnel_type.toUpperCase()}-${binding.tunnel_id}`;
        
        return {
          ...binding,
          server_name: server?.display_name || server?.name,
          tunnel_name: tunnelName,
          tunnel_config: tunnelConfig,
        };
      });
      
      setBindings(enrichedBindings);
      setServers(serversRes);
      setTunnelConfigs({
        frp: frpList,
        nps: npsList,
        easytier: easytierList,
        cftunnel: cftunnelList,
        wireguard: wireguardList,
      });
    } catch (error) {
      message.error(t('monitor.tunnels.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingBinding(null);
    form.resetFields();
    form.setFieldsValue({
      tunnel_type: 'frp',
      auto_config: true,
    });
    setModalVisible(true);
  };

  const handleEdit = (record: TunnelBinding) => {
    setEditingBinding(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = (id: number) => {
    Modal.confirm({
      title: t('monitor.tunnels.confirmDelete'),
      onOk: async () => {
        try {
          await monitorApi.deleteTunnelBinding(id);
          message.success(t('common.deleteSuccess'));
          fetchData();
        } catch (error) {
          message.error(t('common.deleteFailed'));
        }
      },
    });
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      if (editingBinding) {
        await monitorApi.updateTunnelBinding(editingBinding.id, values);
        message.success(t('common.updateSuccess'));
      } else {
        await monitorApi.createTunnelBinding(values);
        message.success(t('common.createSuccess'));
      }
      
      setModalVisible(false);
      fetchData();
    } catch (error) {
      message.error(t('common.operationFailed'));
    }
  };

  const handleSyncStatus = async (bindingId: number) => {
    try {
      setLoading(true);
      await monitorApi.syncTunnelStatus(bindingId);
      message.success(t('monitor.tunnels.syncSuccess'));
      fetchData();
    } catch (error) {
      message.error(t('monitor.tunnels.syncFailed'));
    } finally {
      setLoading(false);
    }
  };

  const getTunnelTypeIcon = (type: string) => {
    return <ApiOutlined />;
  };

  const getTunnelTypeColor = (type: string) => {
    const colorMap: Record<string, string> = {
      frp: 'blue',
      nps: 'green',
      easytier: 'purple',
      cftunnel: 'orange',
      wireguard: 'cyan',
    };
    return colorMap[type] || 'default';
  };

  const getStatusColor = (status: string) => {
    const colorMap: Record<string, string> = {
      connected: 'success',
      disconnected: 'error',
      unknown: 'default',
    };
    return colorMap[status] || 'default';
  };

  const columns = [
    {
      title: t('monitor.tunnels.serverName'),
      dataIndex: 'server_name',
      key: 'server_name',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: t('monitor.tunnels.tunnelType'),
      dataIndex: 'tunnel_type',
      key: 'tunnel_type',
      width: 120,
      render: (type: string) => (
        <Space>
          {getTunnelTypeIcon(type)}
          <Tag color={getTunnelTypeColor(type)}>{type.toUpperCase()}</Tag>
        </Space>
      ),
    },
    {
      title: t('monitor.tunnels.tunnelName'),
      dataIndex: 'tunnel_name',
      key: 'tunnel_name',
      render: (text: string, record: TunnelBinding) => (
        <Space>
          <LinkOutlined />
          <a href={`/${record.tunnel_type}?id=${record.tunnel_id}`} target="_blank" rel="noopener noreferrer">
            {text}
          </a>
        </Space>
      ),
    },
    {
      title: t('monitor.tunnels.autoConfig'),
      dataIndex: 'auto_config',
      key: 'auto_config',
      width: 100,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? t('common.enabled') : t('common.disabled')}
        </Tag>
      ),
    },
    {
      title: t('monitor.tunnels.status'),
      dataIndex: 'tunnel_status',
      key: 'tunnel_status',
      width: 120,
      render: (status: string) => {
        const statusMap: Record<string, { label: string; icon: React.ReactNode }> = {
          connected: { label: t('monitor.tunnels.connected'), icon: <CheckCircleOutlined /> },
          disconnected: { label: t('monitor.tunnels.disconnected'), icon: <CloseCircleOutlined /> },
          unknown: { label: t('monitor.tunnels.unknown'), icon: <CloseCircleOutlined /> },
        };
        const info = statusMap[status] || statusMap.unknown;
        return (
          <Tag color={getStatusColor(status)} icon={info.icon}>
            {info.label}
          </Tag>
        );
      },
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 220,
      render: (_: any, record: TunnelBinding) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<SyncOutlined />}
            onClick={() => handleSyncStatus(record.id)}
          >
            {t('monitor.tunnels.syncStatus')}
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          />
          <Button
            type="link"
            size="small"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record.id)}
          />
        </Space>
      ),
    },
  ];

  const stats = {
    total: bindings.length,
    connected: bindings.filter(b => b.tunnel_status === 'connected').length,
    disconnected: bindings.filter(b => b.tunnel_status === 'disconnected').length,
    autoConfig: bindings.filter(b => b.auto_config).length,
  };

  return (
    <div style={{ padding: '24px' }}>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.tunnels.totalTunnels')}
              value={stats.total}
              prefix={<ApiOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.tunnels.connected')}
              value={stats.connected}
              valueStyle={{ color: '#3f8600' }}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.tunnels.disconnected')}
              value={stats.disconnected}
              valueStyle={{ color: '#cf1322' }}
              prefix={<CloseCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.tunnels.autoConfig')}
              value={stats.autoConfig}
              prefix={<SyncOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Card
        title={
          <Space>
            <ApiOutlined />
            {t('monitor.tunnels.title')}
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            {t('monitor.tunnels.addBinding')}
          </Button>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <Tag color="blue">{t('monitor.tunnels.description')}</Tag>
        </div>

        <Table
          loading={loading}
          dataSource={bindings}
          columns={columns}
          rowKey="id"
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingBinding ? t('monitor.tunnels.editBinding') : t('monitor.tunnels.addBinding')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="server_id"
            label={t('monitor.tunnels.selectServer')}
            rules={[{ required: true }]}
          >
            <Select
              placeholder={t('monitor.tunnels.selectServerPlaceholder')}
              showSearch
              filterOption={(input, option) => {
                const text = typeof option?.children === 'string' ? option.children : String(option?.value ?? '');
                return text.toLowerCase().includes(input.toLowerCase());
              }}
            >
              {servers.map((server) => (
                <Option key={server.id} value={server.id}>
                  {server.display_name || server.name}
                  {server.is_online && <Tag color="success" style={{ marginLeft: 8 }}>在线</Tag>}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="tunnel_type"
            label={t('monitor.tunnels.tunnelType')}
            rules={[{ required: true }]}
          >
            <Select onChange={() => form.resetFields(['tunnel_id'])}>
              <Option value="frp">
                <Space>
                  <Tag color="blue">FRP</Tag>
                  <span>Fast Reverse Proxy</span>
                </Space>
              </Option>
              <Option value="nps">
                <Space>
                  <Tag color="green">NPS</Tag>
                  <span>NPS Server</span>
                </Space>
              </Option>
              <Option value="easytier">
                <Space>
                  <Tag color="purple">EasyTier</Tag>
                  <span>P2P VPN</span>
                </Space>
              </Option>
              <Option value="cftunnel">
                <Space>
                  <Tag color="orange">Cloudflare Tunnel</Tag>
                  <span>Cloudflared</span>
                </Space>
              </Option>
              <Option value="wireguard">
                <Space>
                  <Tag color="cyan">WireGuard</Tag>
                  <span>Modern VPN</span>
                </Space>
              </Option>
            </Select>
          </Form.Item>

          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.tunnel_type !== cur.tunnel_type}>
            {({ getFieldValue }) => {
              const tunnelType = getFieldValue('tunnel_type');
              const configs = tunnelConfigs[tunnelType] || [];
              
              return (
                <Form.Item
                  name="tunnel_id"
                  label={t('monitor.tunnels.selectTunnel')}
                  rules={[{ required: true }]}
                >
                  <Select
                    placeholder={t('monitor.tunnels.selectTunnelPlaceholder')}
                    showSearch
                    filterOption={(input, option) => {
                      const text = typeof option?.children === 'string' ? option.children : String(option?.value ?? '');
                      return text.toLowerCase().includes(input.toLowerCase());
                    }}
                  >
                    {configs.map((config: TunnelConfig) => (
                      <Option key={config.id} value={config.id}>
                        {config.name}
                        {config.enable && <Tag color="success" style={{ marginLeft: 8 }}>启用</Tag>}
                      </Option>
                    ))}
                  </Select>
                </Form.Item>
              );
            }}
          </Form.Item>

          <Form.Item
            name="auto_config"
            label={t('monitor.tunnels.autoConfig')}
            valuePropName="checked"
            extra={t('monitor.tunnels.autoConfigTip')}
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name="remark"
            label={t('common.remark')}
          >
            <TextArea rows={3} placeholder={t('monitor.tunnels.remarkPlaceholder')} />
          </Form.Item>

          <Divider />

          <div style={{ padding: '12px', background: '#f5f5f5', borderRadius: 4 }}>
            <p style={{ margin: 0, fontSize: 12, color: '#666' }}>
              💡 {t('monitor.tunnels.tip1')}
            </p>
            <p style={{ margin: '8px 0 0 0', fontSize: 12, color: '#666' }}>
              💡 {t('monitor.tunnels.tip2')}
            </p>
            <p style={{ margin: '8px 0 0 0', fontSize: 12, color: '#666' }}>
              💡 {t('monitor.tunnels.tip3')}
            </p>
          </div>
        </Form>
      </Modal>
    </div>
  );
};

export default MonitorTunnels;

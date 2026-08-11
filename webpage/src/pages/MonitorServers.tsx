import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Table, Tag, Statistic, Space, Button, Modal, Form, Input, Select, Switch, App } from 'antd';
import { CloudServerOutlined, CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined, PlusOutlined } from '@ant-design/icons';
import { monitorApi } from '../api';

const { Option } = Select;

interface MonitorServer {
  id: number;
  name: string;
  display_name: string;
  enable: boolean;
  group_name: string;
  tags: string;
  access_type: string;
  is_online: boolean;
  os: string;
  arch: string;
  hostname: string;
  last_heartbeat: string;
  remark: string;
}

interface MonitorMetric {
  cpu_usage: number;
  mem_usage: number;
  disk_usage: number;
  net_sent: number;
  net_recv: number;
  process_count: number;
}

const MonitorServers: React.FC = () => {
  const { message, modal } = App.useApp();
  const [servers, setServers] = useState<MonitorServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingServer, setEditingServer] = useState<MonitorServer | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    fetchServers();
    const interval = setInterval(fetchServers, 30000); // 每30秒刷新
    return () => clearInterval(interval);
  }, []);

  const fetchServers = async () => {
    setLoading(true);
    try {
      const response = await monitorApi.listServers();
      setServers(response.data || []);
    } catch (error) {
      message.error('获取服务器列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingServer(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (server: MonitorServer) => {
    setEditingServer(server);
    form.setFieldsValue(server);
    setModalVisible(true);
  };

  const handleDelete = async (id: number) => {
    modal.confirm({
      title: '确认删除',
      content: '确定要删除这个服务器吗？这将删除所有相关的监控数据。',
      onOk: async () => {
        try {
          await monitorApi.deleteServer(id);
          message.success('删除成功');
          fetchServers();
        } catch (error) {
          message.error('删除失败');
        }
      },
    });
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      
      if (editingServer) {
        await monitorApi.updateServer(editingServer.id, values);
        message.success('更新成功');
      } else {
        await monitorApi.createServer(values);
        message.success('创建成功');
      }
      
      setModalVisible(false);
      fetchServers();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns = [
    {
      title: '状态',
      dataIndex: 'is_online',
      key: 'is_online',
      width: 80,
      render: (online: boolean) => (
        <Tag icon={online ? <CheckCircleOutlined /> : <CloseCircleOutlined />} color={online ? 'success' : 'error'}>
          {online ? '在线' : '离线'}
        </Tag>
      ),
    },
    {
      title: '服务器名称',
      dataIndex: 'display_name',
      key: 'display_name',
      render: (text: string, record: MonitorServer) => (
        <Space direction="vertical" size={0}>
          <strong>{text || record.name}</strong>
          <span style={{ fontSize: '12px', color: '#999' }}>{record.hostname}</span>
        </Space>
      ),
    },
    {
      title: '分组',
      dataIndex: 'group_name',
      key: 'group_name',
      width: 120,
    },
    {
      title: '接入方式',
      dataIndex: 'access_type',
      key: 'access_type',
      width: 100,
      render: (type: string) => {
        const typeMap: Record<string, { text: string; color: string }> = {
          agent: { text: 'Agent', color: 'blue' },
          ssh: { text: 'SSH', color: 'green' },
          http: { text: 'HTTP', color: 'orange' },
        };
        const config = typeMap[type] || { text: type, color: 'default' };
        return <Tag color={config.color}>{config.text}</Tag>;
      },
    },
    {
      title: '系统信息',
      key: 'system',
      width: 150,
      render: (_: any, record: MonitorServer) => (
        <span style={{ fontSize: '12px' }}>
          {record.os} / {record.arch}
        </span>
      ),
    },
    {
      title: '最后心跳',
      dataIndex: 'last_heartbeat',
      key: 'last_heartbeat',
      width: 180,
      render: (time: string) => time ? new Date(time).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_: any, record: MonitorServer) => (
        <Space>
          <Button size="small" onClick={() => handleEdit(record)}>编辑</Button>
          <Button size="small" danger onClick={() => handleDelete(record.id)}>删除</Button>
        </Space>
      ),
    },
  ];

  // 统计数据
  const stats = {
    total: servers.length,
    online: servers.filter(s => s.is_online).length,
    offline: servers.filter(s => !s.is_online).length,
  };

  return (
    <div style={{ padding: '24px' }}>
      <Row gutter={[16, 16]} style={{ marginBottom: '24px' }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="服务器总数"
              value={stats.total}
              prefix={<CloudServerOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="在线"
              value={stats.online}
              valueStyle={{ color: '#3f8600' }}
              prefix={<CheckCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="离线"
              value={stats.offline}
              valueStyle={{ color: '#cf1322' }}
              prefix={<CloseCircleOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="在线率"
              value={stats.total > 0 ? ((stats.online / stats.total) * 100).toFixed(1) : 0}
              suffix="%"
            />
          </Card>
        </Col>
      </Row>

      <Card
        title="服务器列表"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={fetchServers}>刷新</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>添加服务器</Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={servers}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingServer ? '编辑服务器' : '添加服务器'}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="服务器名称" rules={[{ required: true }]}>
            <Input placeholder="服务器标识名称" />
          </Form.Item>
          
          <Form.Item name="display_name" label="显示名称">
            <Input placeholder="用于显示的友好名称" />
          </Form.Item>
          
          <Form.Item name="group_name" label="分组">
            <Input placeholder="例如：生产环境、测试环境" />
          </Form.Item>
          
          <Form.Item name="access_type" label="接入方式" rules={[{ required: true }]}>
            <Select>
              <Option value="agent">Agent 主动上报</Option>
              <Option value="ssh">SSH 被动采集</Option>
              <Option value="http">HTTP 探测</Option>
            </Select>
          </Form.Item>
          
          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.access_type !== curr.access_type}>
            {({ getFieldValue }) => {
              const accessType = getFieldValue('access_type');
              
              if (accessType === 'agent') {
                return (
                  <>
                    <Form.Item name="agent_addr" label="Agent 地址" rules={[{ required: true }]}>
                      <Input placeholder="例如：127.0.0.1:50051" />
                    </Form.Item>
                    <Form.Item name="agent_token" label="认证 Token">
                      <Input.Password placeholder="Agent 认证密钥" />
                    </Form.Item>
                  </>
                );
              }
              
              if (accessType === 'ssh') {
                return (
                  <>
                    <Form.Item name="ssh_addr" label="SSH 地址" rules={[{ required: true }]}>
                      <Input placeholder="例如：192.168.1.100:22" />
                    </Form.Item>
                    <Form.Item name="ssh_user" label="SSH 用户名" rules={[{ required: true }]}>
                      <Input placeholder="SSH 登录用户名" />
                    </Form.Item>
                    <Form.Item name="ssh_password" label="SSH 密码">
                      <Input.Password placeholder="SSH 登录密码" />
                    </Form.Item>
                  </>
                );
              }
              
              if (accessType === 'http') {
                return (
                  <Form.Item name="http_probe_url" label="HTTP 探测地址" rules={[{ required: true }]}>
                    <Input placeholder="例如：http://example.com/health" />
                  </Form.Item>
                );
              }
              
              return null;
            }}
          </Form.Item>
          
          <Form.Item name="enable" label="启用监控" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
          
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} placeholder="服务器备注信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MonitorServers;

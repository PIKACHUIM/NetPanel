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
  Tag,
  Tooltip,
  Divider,
  App,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SyncOutlined,
  LinkOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import { monitorApi } from '../api';
import { useTranslation } from 'react-i18next';

const { Option } = Select;

interface DDNSBinding {
  id: number;
  server_id: number;
  ddns_task_id: number;
  ip_type: string;
  auto_update: boolean;
  last_trigger_time?: string;
  server_name?: string;
  ddns_task_name?: string;
  current_ip?: string;
}

interface Server {
  id: number;
  name: string;
  display_name: string;
  is_online: boolean;
}

interface DDNSTask {
  id: number;
  name: string;
  enable: boolean;
  provider: string;
  domains: string;
  current_ip: string;
}

const MonitorDDNS: React.FC = () => {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [bindings, setBindings] = useState<DDNSBinding[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [ddnsTasks, setDdnsTasks] = useState<DDNSTask[]>([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingBinding, setEditingBinding] = useState<DDNSBinding | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    setLoading(true);
    try {
      // monitorApi 的响应拦截器已解包 data,运行时返回裸数组,此处仅修正 TS 类型
      const [bindingsRes, serversRes, tasksRes] = await Promise.all([
        monitorApi.listDDNSBindings() as unknown as DDNSBinding[],
        monitorApi.listServers() as unknown as Server[],
        fetch('/api/v1/ddns').then(res => res.json()),
      ]);

      // fetch 接口返回 {code, data} 包装,统一解包
      const ddnsTasks: DDNSTask[] = tasksRes?.data ?? tasksRes ?? [];

      // 关联服务器名称和DDNS任务名称
      const enrichedBindings = bindingsRes.map((binding: DDNSBinding) => {
        const server = serversRes.find((s: Server) => s.id === binding.server_id);
        const task = ddnsTasks.find((t: DDNSTask) => t.id === binding.ddns_task_id);
        return {
          ...binding,
          server_name: server?.display_name || server?.name,
          ddns_task_name: task?.name,
          current_ip: task?.current_ip,
        };
      });
      
      setBindings(enrichedBindings);
      setServers(serversRes);
      setDdnsTasks(ddnsTasks);
    } catch (error) {
      message.error(t('monitor.ddns.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingBinding(null);
    form.resetFields();
    form.setFieldsValue({
      ip_type: 'IPv4',
      auto_update: true,
    });
    setModalVisible(true);
  };

  const handleEdit = (record: DDNSBinding) => {
    setEditingBinding(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = (id: number) => {
    modal.confirm({
      title: t('monitor.ddns.confirmDelete'),
      onOk: async () => {
        try {
          await monitorApi.deleteDDNSBinding(id);
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
        await monitorApi.updateDDNSBinding(editingBinding.id, values);
        message.success(t('common.updateSuccess'));
      } else {
        await monitorApi.createDDNSBinding(values);
        message.success(t('common.createSuccess'));
      }
      
      setModalVisible(false);
      fetchData();
    } catch (error) {
      message.error(t('common.operationFailed'));
    }
  };

  const handleTriggerUpdate = async (bindingId: number) => {
    try {
      setLoading(true);
      await monitorApi.triggerDDNSUpdate(bindingId);
      message.success(t('monitor.ddns.triggerSuccess'));
      fetchData();
    } catch (error) {
      message.error(t('monitor.ddns.triggerFailed'));
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: t('monitor.ddns.serverName'),
      dataIndex: 'server_name',
      key: 'server_name',
      render: (text: string, record: DDNSBinding) => (
        <Space>
          <Tag color="blue">{text}</Tag>
          {record.current_ip && (
            <Tooltip title={t('monitor.ddns.currentIP')}>
              <Tag color="green">{record.current_ip}</Tag>
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: t('monitor.ddns.ddnsTask'),
      dataIndex: 'ddns_task_name',
      key: 'ddns_task_name',
      render: (text: string, record: DDNSBinding) => (
        <Space>
          <LinkOutlined />
          <a href={`/ddns?id=${record.ddns_task_id}`} target="_blank" rel="noopener noreferrer">
            {text}
          </a>
        </Space>
      ),
    },
    {
      title: t('monitor.ddns.ipType'),
      dataIndex: 'ip_type',
      key: 'ip_type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'IPv4' ? 'blue' : 'purple'}>{type}</Tag>
      ),
    },
    {
      title: t('monitor.ddns.autoUpdate'),
      dataIndex: 'auto_update',
      key: 'auto_update',
      width: 100,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? t('common.enabled') : t('common.disabled')}
        </Tag>
      ),
    },
    {
      title: t('monitor.ddns.lastTrigger'),
      dataIndex: 'last_trigger_time',
      key: 'last_trigger_time',
      width: 180,
      render: (time: string) => time || '-',
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 200,
      render: (_: any, record: DDNSBinding) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<SyncOutlined />}
            onClick={() => handleTriggerUpdate(record.id)}
          >
            {t('monitor.ddns.triggerUpdate')}
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

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <HistoryOutlined />
            {t('monitor.ddns.title')}
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            {t('monitor.ddns.addBinding')}
          </Button>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <Tag color="blue">{t('monitor.ddns.description')}</Tag>
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
        title={editingBinding ? t('monitor.ddns.editBinding') : t('monitor.ddns.addBinding')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="server_id"
            label={t('monitor.ddns.selectServer')}
            rules={[{ required: true }]}
          >
            <Select
              placeholder={t('monitor.ddns.selectServerPlaceholder')}
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
            name="ddns_task_id"
            label={t('monitor.ddns.selectDDNSTask')}
            rules={[{ required: true }]}
          >
            <Select
              placeholder={t('monitor.ddns.selectDDNSTaskPlaceholder')}
              showSearch
              filterOption={(input, option) => {
                const text = typeof option?.children === 'string' ? option.children : String(option?.value ?? '');
                return text.toLowerCase().includes(input.toLowerCase());
              }}
            >
              {ddnsTasks.map((task) => (
                <Option key={task.id} value={task.id}>
                  {task.name}
                  <Tag color="blue" style={{ marginLeft: 8 }}>{task.provider}</Tag>
                  {task.enable && <Tag color="success">启用</Tag>}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="ip_type"
            label={t('monitor.ddns.ipType')}
            rules={[{ required: true }]}
          >
            <Select>
              <Option value="IPv4">IPv4</Option>
              <Option value="IPv6">IPv6</Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="auto_update"
            label={t('monitor.ddns.autoUpdate')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Divider />

          <div style={{ padding: '12px', background: '#f5f5f5', borderRadius: 4 }}>
            <p style={{ margin: 0, fontSize: 12, color: '#666' }}>
              💡 {t('monitor.ddns.tip1')}
            </p>
            <p style={{ margin: '8px 0 0 0', fontSize: 12, color: '#666' }}>
              💡 {t('monitor.ddns.tip2')}
            </p>
          </div>
        </Form>
      </Modal>
    </div>
  );
};

export default MonitorDDNS;

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
  Row,
  Col,
  Statistic,
  Divider,
  Tabs,
  App,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  BellOutlined,
  MailOutlined,
  RobotOutlined,
  GlobalOutlined,
  WechatOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { monitorApi } from '../api';
import { useTranslation } from 'react-i18next';

const { Option } = Select;
const { TextArea } = Input;

interface NotificationChannel {
  id: number;
  name: string;
  type: string;
  config: string;
  enabled: boolean;
  created_at: string;
}

interface CallbackAccount {
  id: number;
  name: string;
  type: string;
  config: string;
}

const MonitorNotifications: React.FC = () => {
  const { t } = useTranslation();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [callbackAccounts, setCallbackAccounts] = useState<CallbackAccount[]>([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [testModalVisible, setTestModalVisible] = useState(false);
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null);
  const [form] = Form.useForm();
  const [testForm] = Form.useForm();

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    setLoading(true);
    try {
      // 获取通知渠道和已有的回调账号
      // monitorApi 的响应拦截器已解包 data,运行时返回裸数组,此处仅修正 TS 类型
      const [channelsRes, accountsRes] = await Promise.all([
        monitorApi.listNotifications() as unknown as NotificationChannel[],
        fetch('/api/v1/callback/accounts').then(res => res.json()),
      ]);

      // fetch 接口返回 {code, data} 包装,统一解包
      const callbackList: CallbackAccount[] = accountsRes?.data ?? accountsRes ?? [];

      setChannels(channelsRes);
      setCallbackAccounts(callbackList);
    } catch (error) {
      message.error(t('monitor.notifications.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = () => {
    setEditingChannel(null);
    form.resetFields();
    form.setFieldsValue({ enabled: true, type: 'webhook' });
    setModalVisible(true);
  };

  const handleEdit = (record: NotificationChannel) => {
    setEditingChannel(record);
    const config = JSON.parse(record.config || '{}');
    form.setFieldsValue({
      ...record,
      ...config,
    });
    setModalVisible(true);
  };

  const handleDelete = (id: number) => {
    modal.confirm({
      title: t('monitor.notifications.confirmDelete'),
      onOk: async () => {
        try {
          await monitorApi.deleteNotification(id);
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
      const { name, type, enabled, callback_account_id, ...configFields } = values;
      
      const payload = {
        name,
        type,
        enabled,
        callback_account_id,
        config: JSON.stringify(configFields),
      };
      
      if (editingChannel) {
        await monitorApi.updateNotification(editingChannel.id, payload);
        message.success(t('common.updateSuccess'));
      } else {
        await monitorApi.createNotification(payload);
        message.success(t('common.createSuccess'));
      }
      
      setModalVisible(false);
      fetchData();
    } catch (error) {
      message.error(t('common.operationFailed'));
    }
  };

  const handleTest = (channel: NotificationChannel) => {
    testForm.resetFields();
    testForm.setFieldsValue({
      channel_id: channel.id,
      title: '测试通知',
      content: '这是一条测试通知消息，用于验证通知渠道是否配置正确。',
    });
    setTestModalVisible(true);
  };

  const handleSendTest = async () => {
    try {
      const values = await testForm.validateFields();
      await monitorApi.sendTestNotification(values);
      message.success(t('monitor.notifications.testSent'));
      setTestModalVisible(false);
    } catch (error) {
      message.error(t('monitor.notifications.testFailed'));
    }
  };

  const getChannelIcon = (type: string) => {
    const icons: Record<string, React.ReactNode> = {
      webhook: <GlobalOutlined />,
      email: <MailOutlined />,
      wechat_work: <WechatOutlined />,
      dingtalk: <RobotOutlined />,
      telegram: <SendOutlined />,
      discord: <RobotOutlined />,
      qq_bot: <RobotOutlined />,
      wxpusher: <WechatOutlined />,
    };
    return icons[type] || <BellOutlined />;
  };

  const renderConfigFields = (type: string) => {
    switch (type) {
      case 'webhook':
        return (
          <>
            <Form.Item
              name="callback_account_id"
              label={t('monitor.notifications.selectCallbackAccount')}
              extra={t('monitor.notifications.selectCallbackAccountTip')}
            >
              <Select
                placeholder={t('monitor.notifications.selectCallbackAccountPlaceholder')}
                allowClear
                showSearch
                filterOption={(input, option) => {
                  const text = typeof option?.children === 'string' ? option.children : String(option?.value ?? '');
                  return text.toLowerCase().includes(input.toLowerCase());
                }}
              >
                {callbackAccounts
                  .filter(acc => acc.type === 'webhook')
                  .map((acc) => (
                    <Option key={acc.id} value={acc.id}>
                      {acc.name}
                    </Option>
                  ))}
              </Select>
            </Form.Item>
            <Form.Item
              name="webhook_url"
              label={t('monitor.notifications.webhookUrl')}
              rules={[{ required: true, type: 'url' }]}
            >
              <Input placeholder="https://example.com/webhook" />
            </Form.Item>
            <Form.Item name="webhook_method" label={t('monitor.notifications.method')}>
              <Select>
                <Option value="POST">POST</Option>
                <Option value="GET">GET</Option>
              </Select>
            </Form.Item>
            <Form.Item name="webhook_headers" label={t('monitor.notifications.headers')}>
              <TextArea
                rows={3}
                placeholder='{"Authorization": "Bearer token"}'
              />
            </Form.Item>
          </>
        );
      
      case 'email':
        return (
          <>
            <Form.Item
              name="smtp_host"
              label={t('monitor.notifications.smtpHost')}
              rules={[{ required: true }]}
            >
              <Input placeholder="smtp.example.com" />
            </Form.Item>
            <Form.Item
              name="smtp_port"
              label={t('monitor.notifications.smtpPort')}
              rules={[{ required: true }]}
            >
              <Input placeholder="587" type="number" />
            </Form.Item>
            <Form.Item
              name="smtp_user"
              label={t('monitor.notifications.smtpUser')}
              rules={[{ required: true }]}
            >
              <Input placeholder="user@example.com" />
            </Form.Item>
            <Form.Item
              name="smtp_password"
              label={t('monitor.notifications.smtpPassword')}
              rules={[{ required: true }]}
            >
              <Input.Password />
            </Form.Item>
            <Form.Item
              name="smtp_from"
              label={t('monitor.notifications.fromEmail')}
              rules={[{ required: true, type: 'email' }]}
            >
              <Input placeholder="noreply@example.com" />
            </Form.Item>
            <Form.Item
              name="to_emails"
              label={t('monitor.notifications.toEmails')}
              rules={[{ required: true }]}
            >
              <Input placeholder="admin@example.com, user@example.com" />
            </Form.Item>
          </>
        );
      
      case 'wechat_work':
        return (
          <>
            <Form.Item
              name="webhook_url"
              label={t('monitor.notifications.webhookUrl')}
              rules={[{ required: true }]}
            >
              <Input placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" />
            </Form.Item>
          </>
        );
      
      case 'dingtalk':
        return (
          <>
            <Form.Item
              name="webhook_url"
              label={t('monitor.notifications.webhookUrl')}
              rules={[{ required: true }]}
            >
              <Input placeholder="https://oapi.dingtalk.com/robot/send?access_token=xxx" />
            </Form.Item>
            <Form.Item name="secret" label={t('monitor.notifications.secret')}>
              <Input.Password placeholder="SEC..." />
            </Form.Item>
          </>
        );
      
      case 'telegram':
        return (
          <>
            <Form.Item
              name="bot_token"
              label={t('monitor.notifications.botToken')}
              rules={[{ required: true }]}
            >
              <Input.Password placeholder="123456:ABC-DEF..." />
            </Form.Item>
            <Form.Item
              name="chat_id"
              label={t('monitor.notifications.chatId')}
              rules={[{ required: true }]}
            >
              <Input placeholder="-1001234567890" />
            </Form.Item>
          </>
        );
      
      case 'discord':
        return (
          <>
            <Form.Item
              name="webhook_url"
              label={t('monitor.notifications.webhookUrl')}
              rules={[{ required: true }]}
            >
              <Input placeholder="https://discord.com/api/webhooks/..." />
            </Form.Item>
          </>
        );
      
      case 'qq_bot':
        return (
          <>
            <Form.Item
              name="api_url"
              label={t('monitor.notifications.apiUrl')}
              rules={[{ required: true }]}
            >
              <Input placeholder="http://localhost:5700" />
            </Form.Item>
            <Form.Item
              name="qq_number"
              label={t('monitor.notifications.qqNumber')}
              rules={[{ required: true }]}
            >
              <Input placeholder="123456789" />
            </Form.Item>
          </>
        );
      
      case 'wxpusher':
        return (
          <>
            <Form.Item
              name="app_token"
              label={t('monitor.notifications.appToken')}
              rules={[{ required: true }]}
            >
              <Input.Password placeholder="AT_xxx" />
            </Form.Item>
            <Form.Item
              name="uids"
              label={t('monitor.notifications.uids')}
              rules={[{ required: true }]}
            >
              <Input placeholder="UID_xxx, UID_yyy" />
            </Form.Item>
          </>
        );
      
      default:
        return null;
    }
  };

  const columns = [
    {
      title: t('monitor.notifications.channelName'),
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: NotificationChannel) => (
        <Space>
          {getChannelIcon(record.type)}
          <span>{text}</span>
        </Space>
      ),
    },
    {
      title: t('monitor.notifications.channelType'),
      dataIndex: 'type',
      key: 'type',
      width: 150,
      render: (type: string) => {
        const typeMap: Record<string, { label: string; color: string }> = {
          webhook: { label: 'Webhook', color: 'blue' },
          email: { label: t('monitor.notifications.email'), color: 'green' },
          wechat_work: { label: t('monitor.notifications.wechatWork'), color: 'green' },
          dingtalk: { label: t('monitor.notifications.dingtalk'), color: 'blue' },
          telegram: { label: 'Telegram', color: 'cyan' },
          discord: { label: 'Discord', color: 'purple' },
          qq_bot: { label: t('monitor.notifications.qqBot'), color: 'blue' },
          wxpusher: { label: 'WxPusher', color: 'green' },
        };
        const info = typeMap[type] || { label: type, color: 'default' };
        return <Tag color={info.color}>{info.label}</Tag>;
      },
    },
    {
      title: t('common.status'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 100,
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {enabled ? t('common.enabled') : t('common.disabled')}
        </Tag>
      ),
    },
    {
      title: t('common.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 200,
      render: (_: any, record: NotificationChannel) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<SendOutlined />}
            onClick={() => handleTest(record)}
          >
            {t('monitor.notifications.test')}
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
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.notifications.totalChannels')}
              value={channels.length}
              prefix={<BellOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.notifications.enabledChannels')}
              value={channels.filter(c => c.enabled).length}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title={t('monitor.notifications.callbackAccounts')}
              value={callbackAccounts.length}
              prefix={<GlobalOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Card
        title={
          <Space>
            <BellOutlined />
            {t('monitor.notifications.title')}
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            {t('monitor.notifications.addChannel')}
          </Button>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <Tag color="blue">{t('monitor.notifications.description')}</Tag>
        </div>

        <Table
          loading={loading}
          dataSource={channels}
          columns={columns}
          rowKey="id"
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={editingChannel ? t('monitor.notifications.editChannel') : t('monitor.notifications.addChannel')}
        open={modalVisible}
        onOk={handleSubmit}
        onCancel={() => setModalVisible(false)}
        width={700}
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label={t('monitor.notifications.channelName')}
            rules={[{ required: true }]}
          >
            <Input placeholder={t('monitor.notifications.channelNamePlaceholder')} />
          </Form.Item>

          <Form.Item
            name="type"
            label={t('monitor.notifications.channelType')}
            rules={[{ required: true }]}
          >
            <Select onChange={() => form.resetFields(['webhook_url', 'smtp_host'])}>
              <Option value="webhook">Webhook</Option>
              <Option value="email">{t('monitor.notifications.email')}</Option>
              <Option value="wechat_work">{t('monitor.notifications.wechatWork')}</Option>
              <Option value="dingtalk">{t('monitor.notifications.dingtalk')}</Option>
              <Option value="telegram">Telegram</Option>
              <Option value="discord">Discord</Option>
              <Option value="qq_bot">{t('monitor.notifications.qqBot')}</Option>
              <Option value="wxpusher">WxPusher</Option>
            </Select>
          </Form.Item>

          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.type !== cur.type}>
            {({ getFieldValue }) => renderConfigFields(getFieldValue('type'))}
          </Form.Item>

          <Form.Item name="enabled" label={t('common.status')} valuePropName="checked">
            <Switch />
          </Form.Item>

          <Divider />

          <div style={{ padding: '12px', background: '#f5f5f5', borderRadius: 4 }}>
            <p style={{ margin: 0, fontSize: 12, color: '#666' }}>
              💡 {t('monitor.notifications.tip1')}
            </p>
            <p style={{ margin: '8px 0 0 0', fontSize: 12, color: '#666' }}>
              💡 {t('monitor.notifications.tip2')}
            </p>
          </div>
        </Form>
      </Modal>

      <Modal
        title={t('monitor.notifications.sendTest')}
        open={testModalVisible}
        onOk={handleSendTest}
        onCancel={() => setTestModalVisible(false)}
      >
        <Form form={testForm} layout="vertical">
          <Form.Item name="channel_id" hidden>
            <Input />
          </Form.Item>
          <Form.Item
            name="title"
            label={t('monitor.notifications.testTitle')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="content"
            label={t('monitor.notifications.testContent')}
            rules={[{ required: true }]}
          >
            <TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default MonitorNotifications;

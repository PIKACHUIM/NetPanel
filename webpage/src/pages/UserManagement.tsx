import React, { useState, useEffect, useCallback } from 'react'
import {
  Card, Table, Button, Space, Tag, Switch, Modal, Form,
  Input, message, Popconfirm, Typography, Badge, Tooltip,
  Avatar, Row, Col,
} from 'antd'
import {
  UserAddOutlined, EditOutlined, DeleteOutlined,
  ReloadOutlined, UserOutlined, CrownOutlined,
  LockOutlined, MailOutlined, TeamOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { adminApi } from '../api'
import { useAppStore } from '../store/appStore'

const { Text } = Typography

interface UserItem {
  id: number
  username: string
  email: string
  enable: boolean
  is_admin: boolean
  remark: string
  created_at: string
  updated_at: string
}

const UserManagement: React.FC = () => {
  const { username: currentUsername } = useAppStore()
  const [loading, setLoading] = useState(false)
  const [users, setUsers] = useState<UserItem[]>([])
  const [modalOpen, setModalOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<UserItem | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const isCurrentAdmin = currentUsername === 'admin'

  const fetchUsers = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adminApi.listUsers()
      setUsers(res.data || [])
    } catch (e: any) {
      message.error(e?.response?.data?.message || '获取用户列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  const openCreate = () => {
    setEditingUser(null)
    form.resetFields()
    form.setFieldsValue({ enable: true, is_admin: false })
    setModalOpen(true)
  }

  const openEdit = (user: UserItem) => {
    setEditingUser(user)
    form.setFieldsValue({
      email: user.email,
      enable: user.enable,
      is_admin: user.is_admin,
      remark: user.remark,
      password: '',
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    setSubmitting(true)
    try {
      if (editingUser) {
        // 编辑：不传空密码
        const payload: any = {
          email: values.email,
          enable: values.enable,
          is_admin: values.is_admin,
          remark: values.remark,
        }
        if (values.password) payload.password = values.password
        await adminApi.updateUser(editingUser.id, payload)
        message.success('更新成功')
      } else {
        await adminApi.createUser(values)
        message.success('创建成功')
      }
      setModalOpen(false)
      fetchUsers()
    } catch (e: any) {
      message.error(e?.response?.data?.message || '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await adminApi.deleteUser(id)
      message.success('删除成功')
      fetchUsers()
    } catch (e: any) {
      message.error(e?.response?.data?.message || '删除失败')
    }
  }

  const handleToggleEnable = async (user: UserItem, checked: boolean) => {
    try {
      await adminApi.updateUser(user.id, { enable: checked })
      message.success(checked ? '已启用' : '已禁用')
      fetchUsers()
    } catch (e: any) {
      message.error(e?.response?.data?.message || '操作失败')
    }
  }

  const columns: ColumnsType<UserItem> = [
    {
      title: '用户',
      key: 'user',
      width: 200,
      render: (_, record) => (
        <Space>
          <Avatar
            size={32}
            style={{
              background: record.is_admin
                ? 'linear-gradient(135deg, #f5a623, #f76b1c)'
                : 'linear-gradient(135deg, #1677ff, #0958d9)',
              flexShrink: 0,
            }}
            icon={record.is_admin ? <CrownOutlined /> : <UserOutlined />}
          />
          <div>
            <div style={{ fontWeight: 600, fontSize: 13 }}>
              {record.username}
              {record.username === 'admin' && (
                <Tag color="gold" style={{ marginLeft: 6, fontSize: 10 }}>内置</Tag>
              )}
            </div>
            {record.email && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                <MailOutlined style={{ marginRight: 3 }} />{record.email}
              </Text>
            )}
          </div>
        </Space>
      ),
    },
    {
      title: '角色',
      dataIndex: 'is_admin',
      key: 'is_admin',
      width: 100,
      render: (v: boolean) => (
        <Tag
          color={v ? 'gold' : 'default'}
          icon={v ? <CrownOutlined /> : <UserOutlined />}
        >
          {v ? '管理员' : '普通用户'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enable',
      key: 'enable',
      width: 90,
      render: (v: boolean, record) => (
        <Switch
          checked={v}
          size="small"
          disabled={record.username === 'admin'}
          onChange={(checked) => handleToggleEnable(record, checked)}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    {
      title: '备注',
      dataIndex: 'remark',
      key: 'remark',
      ellipsis: true,
      render: (v: string) => v || <Text type="secondary">-</Text>,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (v: string) => (
        <Text style={{ fontSize: 12 }}>{dayjs(v).format('YYYY-MM-DD HH:mm')}</Text>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) => (
        <Space size={4}>
          <Tooltip title="编辑">
            <Button
              type="text"
              size="small"
              icon={<EditOutlined />}
              onClick={() => openEdit(record)}
            />
          </Tooltip>
          {record.username !== 'admin' && (
            <Popconfirm
              title="确定要删除该用户吗？"
              onConfirm={() => handleDelete(record.id)}
              okText="删除"
              okButtonProps={{ danger: true }}
              cancelText="取消"
            >
              <Tooltip title="删除">
                <Button type="text" size="small" danger icon={<DeleteOutlined />} />
              </Tooltip>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Card
        title={
          <Space>
            <TeamOutlined style={{ color: '#1677ff' }} />
            <span>用户管理</span>
            <Badge count={users.length} style={{ backgroundColor: '#1677ff' }} />
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} size="small" onClick={fetchUsers}>
              刷新
            </Button>
            <Button type="primary" icon={<UserAddOutlined />} onClick={openCreate}>
              新建用户
            </Button>
          </Space>
        }
        style={{ borderRadius: 8 }}
      >
        {/* 说明提示 */}
        <div style={{
          marginBottom: 12,
          padding: '8px 12px',
          background: 'rgba(22,119,255,0.06)',
          borderRadius: 6,
          border: '1px solid rgba(22,119,255,0.15)',
          fontSize: 12,
          color: '#666',
        }}>
          <UserOutlined style={{ marginRight: 6, color: '#1677ff' }} />
          admin 为内置超级管理员，不可删除、不可禁用、不可取消管理员权限。
          {!isCurrentAdmin && ' 只有 admin 可以修改用户的管理员权限。'}
        </div>

        <Table<UserItem>
          columns={columns}
          dataSource={users}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={{ pageSize: 20, showTotal: (t) => `共 ${t} 个用户` }}
        />
      </Card>

      {/* 新建/编辑弹窗 */}
      <Modal
        title={
          <Space>
            {editingUser ? <EditOutlined /> : <UserAddOutlined />}
            {editingUser ? `编辑用户：${editingUser.username}` : '新建用户'}
          </Space>
        }
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        confirmLoading={submitting}
        okText={editingUser ? '保存' : '创建'}
        cancelText="取消"
        width={480}
        destroyOnClose
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          {!editingUser && (
            <Form.Item
              name="username"
              label="用户名"
              rules={[
                { required: true, message: '请输入用户名' },
                { min: 2, max: 50, message: '用户名长度 2-50 位' },
                { pattern: /^[a-zA-Z0-9_-]+$/, message: '只允许字母、数字、下划线、连字符' },
              ]}
            >
              <Input prefix={<UserOutlined />} placeholder="请输入用户名" />
            </Form.Item>
          )}

          <Form.Item
            name="password"
            label={editingUser ? '新密码（留空不修改）' : '密码'}
            rules={editingUser ? [
              { min: 6, message: '密码至少6位', warningOnly: false },
            ] : [
              { required: true, message: '请输入密码' },
              { min: 6, message: '密码至少6位' },
            ]}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder={editingUser ? '留空则不修改密码' : '请输入密码（至少6位）'}
            />
          </Form.Item>

          <Form.Item name="email" label="邮箱">
            <Input prefix={<MailOutlined />} placeholder="请输入邮箱（可选）" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="enable" label="启用状态" valuePropName="checked">
                <Switch
                  checkedChildren="启用"
                  unCheckedChildren="禁用"
                  disabled={editingUser?.username === 'admin'}
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="is_admin" label="管理员权限" valuePropName="checked">
                <Switch
                  checkedChildren="管理员"
                  unCheckedChildren="普通用户"
                  disabled={editingUser?.username === 'admin' || !isCurrentAdmin}
                />
              </Form.Item>
            </Col>
          </Row>

          {!isCurrentAdmin && (
            <div style={{
              fontSize: 12, color: '#999',
              marginTop: -8, marginBottom: 8,
            }}>
              * 只有 admin 可以修改管理员权限
            </div>
          )}

          <Form.Item name="remark" label="备注">
            <Input.TextArea placeholder="备注信息（可选）" rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default UserManagement

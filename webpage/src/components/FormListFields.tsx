/**
 * 通用 Form.List 子组件
 * 从 EasytierClient.tsx 提取的公共组件
 */
import React from 'react'
import { Button, Form, Input, InputNumber, Row, Col, Select } from 'antd'
import { PlusOutlined, MinusCircleOutlined } from '@ant-design/icons'

export const PROTOCOL_OPTIONS = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'WS', value: 'ws' },
  { label: 'WSS', value: 'wss' },
  { label: 'WG', value: 'wg' },
  { label: 'QUIC', value: 'quic' },
]

export const SimpleList = ({
  fieldName, placeholder, addText
}: { fieldName: string; placeholder: string; addText: string }) => (
  <Form.List name={fieldName}>
    {(fields, { add, remove }) => (
      <>
        {fields.map(({ key, name, ...rest }) => (
          <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
            <Col flex="auto">
              <Form.Item {...rest} name={[name, 'value']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写' }]}>
                <Input placeholder={placeholder} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col flex="none" style={{ display: 'flex', alignItems: 'center' }}>
              <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
            </Col>
          </Row>
        ))}
        <Button type="dashed" onClick={() => add({ value: '' })} icon={<PlusOutlined />} block>{addText}</Button>
      </>
    )}
  </Form.List>
)

export const AddrList = ({
  fieldName, addText, defaultPort
}: { fieldName: string; addText: string; defaultPort?: string }) => {
  return (
    <Form.List name={fieldName}>
      {(fields, { add, remove }) => (
        <>
          {fields.map(({ key, name, ...rest }) => (
            <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
              <Col span={5}>
                <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                  <Select options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={13}>
                <Form.Item {...rest} name={[name, 'host']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写地址' }]}>
                  <Input placeholder="IP 或域名" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={5}>
                <Form.Item {...rest} name={[name, 'port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '端口' }]}>
                  <Input placeholder="端口" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={1} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
              </Col>
            </Row>
          ))}
          <Button type="dashed" onClick={() => add({ proto: 'tcp', host: '', port: defaultPort || '' })} icon={<PlusOutlined />} block>{addText}</Button>
        </>
      )}
    </Form.List>
  )
}

export const ProtoPortList = ({
  fieldName, addText, minOne, defaultPort
}: { fieldName: string; addText: string; minOne?: boolean; defaultPort?: string }) => (
  <Form.List
    name={fieldName}
    rules={minOne ? [{
      validator: async (_, items) => {
        if (!items || items.length === 0) throw new Error('至少添加一个')
      }
    }] : undefined}
  >
    {(fields, { add, remove }, { errors }) => (
      <>
        {fields.map(({ key, name, ...rest }) => (
          <Row key={key} gutter={8} align="middle" style={{ marginBottom: 8 }}>
            <Col span={7}>
              <Form.Item {...rest} name={[name, 'proto']} style={{ marginBottom: 0 }}>
                <Select options={PROTOCOL_OPTIONS} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={15}>
              <Form.Item {...rest} name={[name, 'port']} style={{ marginBottom: 0 }} rules={[{ required: true, message: '请填写端口' }]}>
                <Input placeholder={defaultPort || '11010'} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={2} style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {(!minOne || fields.length > 1) && (
                <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#ff4d4f', fontSize: 16 }} />
              )}
            </Col>
          </Row>
        ))}
        {minOne && <Form.ErrorList errors={errors} />}
        <Button type="dashed" onClick={() => add({ proto: 'tcp', port: defaultPort || '' })} icon={<PlusOutlined />} block>{addText}</Button>
      </>
    )}
  </Form.List>
)

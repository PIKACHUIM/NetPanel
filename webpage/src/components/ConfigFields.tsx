/**
 * 声明式配置表单字段
 *
 * 背景：EasytierClient（172 个 Form.Item）、FrpClient（140 个）等页面里，
 * 大量 Form.Item 形如「<Col span><Form.Item name label extra rules><Input/></Form.Item></Col>」，
 * 只是控件类型与文案不同，属于复制粘贴式样板。
 *
 * 本模块把这些样板收敛为字段声明 + 渲染器，同时保留 escape hatch：
 * 需要自定义 children（Form.List、联动渲染等）时用 `children` 字段直接透传，
 * 不强行抽象。
 *
 * 设计约束：
 *   - 纯展示，不含任何数据获取或业务判断
 *   - 渲染结构与手写 JSX 一致（同样的 Row/Col/Form.Item 层级），UI 零变化
 *   - 支持 Form.useWatch 做联动显示（when），但不引入新依赖
 */
import React from 'react'
import { Checkbox, Col, Form, Input, InputNumber, Row, Select, Switch, Typography } from 'antd'
import type { FormItemProps } from 'antd'

const { Text } = Typography

/** 字段控件类型。 */
export type FieldType =
  | 'text'
  | 'password'
  | 'textarea'
  | 'number'
  | 'select'
  | 'switch'
  | 'checkbox'

export interface FieldOption {
  label: React.ReactNode
  value: string | number
}

/** 单个字段声明。 */
export interface FieldSpec {
  /** 字段名，对应 Form.Item 的 name */
  name: string
  /** 控件类型，默认 'text' */
  type?: FieldType
  /** 标签文本 */
  label?: React.ReactNode
  /** 占位符（text/password/number/select 有效） */
  placeholder?: string
  /** 栅格宽度，默认 12（合计 24） */
  span?: number
  /** 辅助说明（渲染在 label 下方的 extra） */
  hint?: React.ReactNode
  /** 校验规则，直接透传给 antd */
  rules?: FormItemProps['rules']
  /** 下拉选项（select 有效） */
  options?: FieldOption[]
  /** 数字输入边界 */
  min?: number
  max?: number
  /** 允许清空（select/input 有效） */
  allowClear?: boolean
  /** textarea 行数 */
  rows?: number
  /** Form.Item 的 marginBottom 覆盖 */
  style?: React.CSSProperties
  /**
   * 自定义渲染内容。提供时忽略 type/options 等控件参数，
   * 用于 Form.List、条件渲染等无法声明式表达的字段。
   */
  children?: React.ReactNode
  /**
   * 联动显示条件。返回 false 时该字段不渲染。
   * 依赖 Form.useWatch 订阅的字段变化。
   */
  when?: (watch: Record<string, any>) => boolean
}

const fullWidth: React.CSSProperties = { width: '100%' }

/** 渲染单个字段的控件部分。 */
function renderControl(spec: FieldSpec): React.ReactNode {
  if (spec.children) return spec.children

  switch (spec.type) {
    case 'password':
      return <Input.Password placeholder={spec.placeholder} style={fullWidth} />
    case 'textarea':
      return <Input.TextArea rows={spec.rows ?? 2} placeholder={spec.placeholder} style={fullWidth} />
    case 'number':
      return (
        <InputNumber
          min={spec.min}
          max={spec.max}
          placeholder={spec.placeholder}
          style={fullWidth}
        />
      )
    case 'select':
      return (
        <Select
          allowClear={spec.allowClear}
          placeholder={spec.placeholder}
          options={spec.options}
          style={fullWidth}
        />
      )
    case 'switch':
      return <Switch />
    case 'checkbox':
      return <Checkbox>{spec.label}</Checkbox>
    default:
      return <Input placeholder={spec.placeholder} style={fullWidth} />
  }
}

/** isCheckLike 判断该字段是否应使用 valuePropName="checked"。 */
const isCheckLike = (t?: FieldType) => t === 'switch' || t === 'checkbox'

/**
 * FieldRow 把字段声明渲染成一行栅格。
 *
 * 组件本身不感知 Form——它渲染的 Form.Item 会自动向上找到最近的 <Form>，
 * 因此可与页面现有的 Form.Item 自由混用。
 */
const FieldRow: React.FC<{ fields: FieldSpec[] }> = ({ fields }) => {
  // useWatch 需在 Form 内调用；无 Form 上下文时 antd 会告警，
  // 故用 try 语义规避——这里直接调用，页面均在 Form 内使用。
  const form = Form.useFormInstance()
  const watch = Form.useWatch([], form) as Record<string, any> | undefined

  return (
    <Row gutter={16}>
      {fields
        .filter((f) => !f.when || f.when(watch ?? {}))
        .map((f) => (
          <Col key={f.name} span={f.span ?? 12}>
            <Form.Item
              name={f.name}
              label={isCheckLike(f.type) ? undefined : f.label}
              valuePropName={isCheckLike(f.type) ? 'checked' : undefined}
              rules={f.rules}
              extra={f.hint ? <span style={{ fontSize: 11 }}>{f.hint}</span> : undefined}
              style={f.style}
            >
              {renderControl(f)}
            </Form.Item>
          </Col>
        ))}
    </Row>
  )
}

/**
 * CheckboxGrid 渲染布尔开关网格。
 *
 * 对应 EasytierClient 中 22 处「<Col span={12}><Form.Item valuePropName=checked
 * style={{marginBottom:4}}><Checkbox>…</Checkbox></Form.Item></Col>」的重复结构。
 */
const CheckboxGrid: React.FC<{
  /** 复选框声明，label 即显示文本 */
  items: Array<Pick<FieldSpec, 'name' | 'label'> & { hint?: React.ReactNode }>
  /** 每行列数，栅格 24 内的占比，默认 12（两列） */
  span?: number
  /** 行间距，默认 [0, 4] */
  gutter?: [number, number]
}> = ({ items, span = 12, gutter = [0, 4] }) => (
  <Row gutter={gutter}>
    {items.map((it) => (
      <Col key={it.name} span={span}>
        <Form.Item name={it.name} valuePropName="checked" style={{ marginBottom: 4 }}>
          <Checkbox>
            {it.label}
            {it.hint && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                {it.hint}
              </Text>
            )}
          </Checkbox>
        </Form.Item>
      </Col>
    ))}
  </Row>
)

export { FieldRow, CheckboxGrid }

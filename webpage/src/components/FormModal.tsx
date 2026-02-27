import React from 'react'
import { Modal, Button, Space, theme as antTheme } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { useAppStore } from '../store/appStore'

interface FormModalProps {
  /** 模态框是否可见 */
  open: boolean
  /** 标题文字 */
  title: string
  /** 标题图标 */
  icon?: React.ReactNode
  /** 是否为编辑模式（影响确认按钮文字） */
  isEdit?: boolean
  /** 确认按钮加载状态 */
  confirmLoading?: boolean
  /** 模态框宽度 */
  width?: number
  /** 确认回调 */
  onOk: () => void
  /** 取消回调 */
  onCancel: () => void
  /** 子内容 */
  children: React.ReactNode
  /** 额外的底部内容（左侧） */
  footerExtra?: React.ReactNode
  /** 确认按钮文字（覆盖默认） */
  okText?: string
  /** 取消按钮文字（覆盖默认） */
  cancelText?: string
}

/**
 * 通用表单模态框组件
 * 参考 OpenIDCS-Client 设计风格，提供精致的模态框 UI
 * 支持 light / dark / glass-light / glass-dark 四种主题
 */
const FormModal: React.FC<FormModalProps> = ({
  open,
  title,
  icon,
  isEdit = false,
  confirmLoading = false,
  width = 580,
  onOk,
  onCancel,
  children,
  footerExtra,
  okText,
  cancelText,
}) => {
  const { theme } = useAppStore()
  const { token } = antTheme.useToken()
  const isGlass = theme === 'glass-light' || theme === 'glass-dark'
  const isDark = theme === 'dark' || theme === 'glass-dark'

  // 根据主题计算模态框背景
  const modalBg = isGlass
    ? isDark
      ? 'rgba(15, 20, 35, 0.82)'
      : 'rgba(255, 255, 255, 0.72)'
    : isDark
    ? token.colorBgElevated
    : token.colorBgContainer

  const headerBg = isDark
    ? 'rgba(255,255,255,0.025)'
    : 'rgba(0,0,0,0.018)'

  const footerBg = isDark
    ? 'rgba(255,255,255,0.025)'
    : 'rgba(0,0,0,0.018)'

  const borderColor = isGlass
    ? isDark
      ? 'rgba(255,255,255,0.1)'
      : 'rgba(0,0,0,0.08)'
    : token.colorBorderSecondary

  return (
    <Modal
      open={open}
      onCancel={onCancel}
      width={width}
      destroyOnClose
      footer={null}
      closable={false}
      centered
      styles={{
        content: {
          padding: 0,
          borderRadius: 16,
          overflow: 'hidden',
          background: modalBg,
          ...(isGlass ? {
            backdropFilter: 'blur(32px) saturate(180%)',
            WebkitBackdropFilter: 'blur(32px) saturate(180%)',
            border: `1px solid ${borderColor}`,
            boxShadow: isDark
              ? '0 24px 64px rgba(0,0,0,0.6), 0 0 0 1px rgba(255,255,255,0.05)'
              : '0 24px 64px rgba(0,0,0,0.18), 0 0 0 1px rgba(255,255,255,0.6)',
          } : {
            border: `1px solid ${borderColor}`,
            boxShadow: isDark
              ? '0 16px 48px rgba(0,0,0,0.4)'
              : '0 16px 48px rgba(0,0,0,0.12)',
          }),
        },
        mask: {
          backdropFilter: isGlass ? 'blur(6px)' : undefined,
          background: isGlass
            ? isDark ? 'rgba(0,0,0,0.55)' : 'rgba(0,0,0,0.3)'
            : undefined,
        },
      }}
    >
      {/* ── 头部 ── */}
      <div style={{
        padding: '18px 22px 16px',
        borderBottom: `1px solid ${borderColor}`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        background: headerBg,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {/* 彩色图标块 */}
          {icon && (
            <div style={{
              width: 36, height: 36,
              borderRadius: 10,
              background: isEdit
                ? 'linear-gradient(135deg, rgba(250,173,20,0.18), rgba(250,173,20,0.08))'
                : 'linear-gradient(135deg, rgba(22,119,255,0.18), rgba(22,119,255,0.08))',
              border: isEdit
                ? '1px solid rgba(250,173,20,0.3)'
                : '1px solid rgba(22,119,255,0.25)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              color: isEdit ? '#faad14' : '#1677ff',
              fontSize: 16,
              flexShrink: 0,
            }}>
              {icon}
            </div>
          )}
          {/* 标题文字 */}
          <div>
            <div style={{
              fontSize: 15,
              fontWeight: 600,
              color: token.colorText,
              lineHeight: 1.35,
              letterSpacing: '-0.01em',
            }}>
              {title}
            </div>
            <div style={{
              fontSize: 12,
              color: token.colorTextTertiary,
              marginTop: 2,
              lineHeight: 1.3,
            }}>
              {isEdit ? '修改现有配置项' : '填写信息以创建新配置'}
            </div>
          </div>
        </div>

        {/* 关闭按钮 */}
        <button
          onClick={onCancel}
          style={{
            width: 30, height: 30,
            borderRadius: 8,
            border: `1px solid ${borderColor}`,
            background: 'transparent',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer',
            color: token.colorTextTertiary,
            fontSize: 13,
            transition: 'all 0.15s ease',
            flexShrink: 0,
            outline: 'none',
          }}
          onMouseEnter={e => {
            e.currentTarget.style.background = isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.06)'
            e.currentTarget.style.color = token.colorText
            e.currentTarget.style.borderColor = isDark ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.15)'
          }}
          onMouseLeave={e => {
            e.currentTarget.style.background = 'transparent'
            e.currentTarget.style.color = token.colorTextTertiary
            e.currentTarget.style.borderColor = borderColor
          }}
        >
          <CloseOutlined />
        </button>
      </div>

      {/* ── 内容区 ── */}
      <div style={{
        padding: '20px 22px',
        maxHeight: 'calc(80vh - 148px)',
        overflowY: 'auto',
        overflowX: 'hidden',
      }}>
        {children}
      </div>

      {/* ── 底部 ── */}
      <div style={{
        padding: '13px 22px',
        borderTop: `1px solid ${borderColor}`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        background: footerBg,
        gap: 8,
      }}>
        <div style={{ flex: 1, minWidth: 0 }}>{footerExtra}</div>
        <Space size={8}>
          <Button
            onClick={onCancel}
            style={{
              borderRadius: 8,
              minWidth: 76,
              height: 34,
              fontSize: 13,
              borderColor: borderColor,
            }}
          >
            {cancelText ?? '取消'}
          </Button>
          <Button
            type="primary"
            onClick={onOk}
            loading={confirmLoading}
            style={{
              borderRadius: 8,
              minWidth: 96,
              height: 34,
              fontSize: 13,
              fontWeight: 500,
              background: isEdit
                ? 'linear-gradient(135deg, #faad14, #fa8c16)'
                : 'linear-gradient(135deg, #1677ff, #0958d9)',
              border: 'none',
              boxShadow: isEdit
                ? '0 2px 10px rgba(250,173,20,0.35)'
                : '0 2px 10px rgba(22,119,255,0.35)',
            }}
          >
            {okText ?? (isEdit ? '保存修改' : '立即创建')}
          </Button>
        </Space>
      </div>
    </Modal>
  )
}

export default FormModal

// ─────────────────────────────────────────────
// FormSection：表单分组标题
// ─────────────────────────────────────────────

type SectionColor = 'blue' | 'purple' | 'green' | 'orange' | 'red' | 'cyan'

const sectionColorMap: Record<SectionColor, { bg: string; border: string; text: string }> = {
  blue:   { bg: 'rgba(22,119,255,0.12)',  border: 'rgba(22,119,255,0.25)',  text: '#1677ff' },
  purple: { bg: 'rgba(114,46,209,0.12)',  border: 'rgba(114,46,209,0.25)',  text: '#722ed1' },
  green:  { bg: 'rgba(82,196,26,0.12)',   border: 'rgba(82,196,26,0.25)',   text: '#52c41a' },
  orange: { bg: 'rgba(250,140,22,0.12)',  border: 'rgba(250,140,22,0.25)',  text: '#fa8c16' },
  red:    { bg: 'rgba(255,77,79,0.12)',   border: 'rgba(255,77,79,0.25)',   text: '#ff4d4f' },
  cyan:   { bg: 'rgba(19,194,194,0.12)',  border: 'rgba(19,194,194,0.25)',  text: '#13c2c2' },
}

export const FormSection: React.FC<{
  title: string
  icon?: React.ReactNode
  color?: SectionColor
  children: React.ReactNode
}> = ({ title, icon, color = 'blue', children }) => {
  const { token } = antTheme.useToken()
  const { theme } = useAppStore()
  const isDark = theme === 'dark' || theme === 'glass-dark'
  const c = sectionColorMap[color]

  return (
    <div style={{ marginBottom: 22 }}>
      {/* 分组标题行 */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        marginBottom: 16,
      }}>
        {/* 彩色图标块 */}
        {icon && (
          <div style={{
            width: 30, height: 30,
            borderRadius: 8,
            background: isDark
              ? c.bg.replace('0.12', '0.18')
              : c.bg,
            border: `1px solid ${c.border}`,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: c.text,
            fontSize: 14,
            flexShrink: 0,
          }}>
            {icon}
          </div>
        )}
        {/* 标题文字 */}
        <span style={{
          fontSize: 13,
          fontWeight: 600,
          color: token.colorText,
          letterSpacing: '0.02em',
        }}>
          {title}
        </span>
        {/* 右侧分隔线 */}
        <div style={{
          flex: 1,
          height: 1,
          background: `linear-gradient(to right, ${token.colorBorderSecondary}, transparent)`,
          marginLeft: 4,
        }} />
      </div>

      {/* 内容 */}
      <div style={{ paddingLeft: icon ? 0 : 0 }}>
        {children}
      </div>
    </div>
  )
}

/**
 * 表单分组标题
 *
 * 此前该组件在 9 个页面中各有一份逐字副本（仅缩进风格不同），
 * 本 PR 收敛为单一实现。样式与原实现完全一致，UI 无变化。
 */
import React from 'react'

const SectionTitle = ({ children }: { children: React.ReactNode }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '12px 0 8px' }}>
    <div style={{ width: 3, height: 14, background: '#0071e3', borderRadius: 2, flexShrink: 0 }} />
    <span style={{ fontSize: 12, fontWeight: 600, color: '#595959', letterSpacing: '0.02em' }}>
      {children}
    </span>
    <div style={{ flex: 1, height: 1, background: '#f0f0f0' }} />
  </div>
)

export default SectionTitle

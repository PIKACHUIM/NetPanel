import React, { useEffect, useState } from 'react'
import { Input, Select, InputNumber, Space, Tag, Tooltip, Typography, TimePicker } from 'antd'
import { ClockCircleOutlined, EditOutlined, CheckCircleOutlined, WarningOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import dayjs, { Dayjs } from 'dayjs'

const { Text } = Typography

// ── 预设选项 ──────────────────────────────────────────────────────────────────

type PresetKey =
  | 'every_minute'
  | 'every_n_minutes'
  | 'every_hour'
  | 'every_n_hours'
  | 'every_day'
  | 'every_week'
  | 'every_month'
  | 'fixed_daily'    // 每天固定时间
  | 'fixed_weekly'   // 每周固定时间
  | 'fixed_monthly'  // 每月固定时间
  | 'custom'

// 预设 key 列表（label/description 在组件内通过 t() 获取）
const PRESET_KEYS: PresetKey[] = [
  'every_minute',
  'every_n_minutes',
  'every_hour',
  'every_n_hours',
  'every_day',
  'every_week',
  'every_month',
  'fixed_daily',
  'fixed_weekly',
  'fixed_monthly',
  'custom',
]

const WEEK_OPTIONS = [
  { value: 1, label: '周一' },
  { value: 2, label: '周二' },
  { value: 3, label: '周三' },
  { value: 4, label: '周四' },
  { value: 5, label: '周五' },
  { value: 6, label: '周六' },
  { value: 0, label: '周日' },
]

// ── 解析表达式 → 推断预设 ─────────────────────────────────────────────────────

interface InferResult {
  key: PresetKey
  n?: number
  hour?: number
  minute?: number
  dow?: number
  dom?: number
}

function inferPreset(expr: string): InferResult {
  if (!expr) return { key: 'every_minute' }
  const parts = expr.trim().split(/\s+/)
  if (parts.length !== 6) return { key: 'custom' }
  const [sec, min, hour, dom, mon, dow] = parts

  if (sec === '0' && min === '*' && hour === '*' && dom === '*' && mon === '*' && dow === '*')
    return { key: 'every_minute' }

  const everyNMin = min.match(/^\*\/(\d+)$/)
  if (sec === '0' && everyNMin && hour === '*' && dom === '*' && mon === '*' && dow === '*')
    return { key: 'every_n_minutes', n: parseInt(everyNMin[1]) }

  if (sec === '0' && min === '0' && hour === '*' && dom === '*' && mon === '*' && dow === '*')
    return { key: 'every_hour' }

  const everyNHour = hour.match(/^\*\/(\d+)$/)
  if (sec === '0' && min === '0' && everyNHour && dom === '*' && mon === '*' && dow === '*')
    return { key: 'every_n_hours', n: parseInt(everyNHour[1]) }

  if (sec === '0' && min === '0' && hour === '0' && dom === '*' && mon === '*' && dow === '*')
    return { key: 'every_day' }

  if (sec === '0' && min === '0' && hour === '0' && dom === '*' && mon === '*' && dow === '1')
    return { key: 'every_week' }

  if (sec === '0' && min === '0' && hour === '0' && dom === '1' && mon === '*' && dow === '*')
    return { key: 'every_month' }

  // 固定时间：每天 HH:MM
  const fixedHour = hour.match(/^(\d+)$/)
  const fixedMin  = min.match(/^(\d+)$/)
  if (sec === '0' && fixedMin && fixedHour && dom === '*' && mon === '*' && dow === '*')
    return { key: 'fixed_daily', hour: parseInt(fixedHour[1]), minute: parseInt(fixedMin[1]) }

  // 固定时间：每周 dow HH:MM
  const fixedDow = dow.match(/^(\d+)$/)
  if (sec === '0' && fixedMin && fixedHour && dom === '*' && mon === '*' && fixedDow)
    return { key: 'fixed_weekly', hour: parseInt(fixedHour[1]), minute: parseInt(fixedMin[1]), dow: parseInt(fixedDow[1]) }

  // 固定时间：每月 dom HH:MM
  const fixedDom = dom.match(/^(\d+)$/)
  if (sec === '0' && fixedMin && fixedHour && fixedDom && mon === '*' && dow === '*')
    return { key: 'fixed_monthly', hour: parseInt(fixedHour[1]), minute: parseInt(fixedMin[1]), dom: parseInt(fixedDom[1]) }

  return { key: 'custom' }
}

// ── 构建表达式 ────────────────────────────────────────────────────────────────

interface BuildParams {
  n?: number
  hour?: number
  minute?: number
  dow?: number
  dom?: number
}

function buildExpr(key: PresetKey, params: BuildParams = {}): string {
  const { n, hour = 8, minute = 0, dow = 1, dom = 1 } = params
  switch (key) {
    case 'every_minute':    return '0 * * * * *'
    case 'every_n_minutes': return `0 */${n || 5} * * * *`
    case 'every_hour':      return '0 0 * * * *'
    case 'every_n_hours':   return `0 0 */${n || 2} * * *`
    case 'every_day':       return '0 0 0 * * *'
    case 'every_week':      return '0 0 0 * * 1'
    case 'every_month':     return '0 0 0 1 * *'
    case 'fixed_daily':     return `0 ${minute} ${hour} * * *`
    case 'fixed_weekly':    return `0 ${minute} ${hour} * * ${dow}`
    case 'fixed_monthly':   return `0 ${minute} ${hour} ${dom} * *`
    default:                return ''
  }
}

// ── 简单校验 ──────────────────────────────────────────────────────────────────

function validateExpr(expr: string): boolean {
  if (!expr) return false
  const parts = expr.trim().split(/\s+/)
  return parts.length === 6
}

// ── 主组件 ────────────────────────────────────────────────────────────────────

interface CronExprInputProps {
  value?: string
  onChange?: (val: string) => void
}

const CronExprInput: React.FC<CronExprInputProps> = ({ value = '', onChange }) => {
  const inferred = inferPreset(value)
  const [presetKey, setPresetKey] = useState<PresetKey>(inferred.key)
  const [nValue,    setNValue]    = useState<number>(inferred.n || 5)
  const [fixedTime, setFixedTime] = useState<Dayjs>(dayjs().hour(inferred.hour ?? 8).minute(inferred.minute ?? 0).second(0))
  const [fixedDow,  setFixedDow]  = useState<number>(inferred.dow ?? 1)
  const [fixedDom,  setFixedDom]  = useState<number>(inferred.dom ?? 1)
  const [rawExpr,   setRawExpr]   = useState<string>(value)

  // 当外部 value 变化时同步
  useEffect(() => {
    const inf = inferPreset(value)
    setPresetKey(inf.key)
    if (inf.n !== undefined) setNValue(inf.n)
    if (inf.hour !== undefined || inf.minute !== undefined)
      setFixedTime(dayjs().hour(inf.hour ?? 8).minute(inf.minute ?? 0).second(0))
    if (inf.dow !== undefined) setFixedDow(inf.dow)
    if (inf.dom !== undefined) setFixedDom(inf.dom)
    setRawExpr(value)
  }, [value])

  // 统一构建并触发 onChange
  const emit = (key: PresetKey, params: BuildParams) => {
    const expr = buildExpr(key, params)
    setRawExpr(expr)
    onChange?.(expr)
  }

  const currentParams = (): BuildParams => ({
    n: nValue,
    hour: fixedTime.hour(),
    minute: fixedTime.minute(),
    dow: fixedDow,
    dom: fixedDom,
  })

  const handlePresetChange = (key: PresetKey) => {
    setPresetKey(key)
    if (key === 'custom') return
    emit(key, currentParams())
  }

  const handleNChange = (n: number | null) => {
    const val = n || 1
    setNValue(val)
    emit(presetKey, { ...currentParams(), n: val })
  }

  const handleTimeChange = (time: Dayjs | null) => {
    const t = time || dayjs().hour(8).minute(0)
    setFixedTime(t)
    emit(presetKey, { ...currentParams(), hour: t.hour(), minute: t.minute() })
  }

  const handleDowChange = (dow: number) => {
    setFixedDow(dow)
    emit(presetKey, { ...currentParams(), dow })
  }

  const handleDomChange = (dom: number | null) => {
    const val = dom || 1
    setFixedDom(val)
    emit(presetKey, { ...currentParams(), dom: val })
  }

  const handleRawChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value
    setRawExpr(v)
    onChange?.(v)
    const inf = inferPreset(v)
    setPresetKey(inf.key)
    if (inf.n !== undefined) setNValue(inf.n)
    if (inf.hour !== undefined || inf.minute !== undefined)
      setFixedTime(dayjs().hour(inf.hour ?? 8).minute(inf.minute ?? 0).second(0))
    if (inf.dow !== undefined) setFixedDow(inf.dow)
    if (inf.dom !== undefined) setFixedDom(inf.dom)
  }

  const { t } = useTranslation()
  const isValid = validateExpr(rawExpr)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {/* 第一行：预设选择 + N值输入 */}
      <Space.Compact style={{ width: '100%' }}>
        <Select
          value={presetKey}
          onChange={handlePresetChange}
          style={{ flex: 1, minWidth: 120 }}
          options={PRESET_KEYS.map(key => ({
            value: key,
            label: (
              <span>
                <ClockCircleOutlined style={{ marginRight: 6, opacity: 0.6 }} />
                {t(`cron.preset.${key}`)}
              </span>
            ),
          }))}
          popupMatchSelectWidth={false}
        />
        {/* 间隔 N 分钟/小时 */}
        {(presetKey === 'every_n_minutes' || presetKey === 'every_n_hours') && (
          <InputNumber
            min={1}
            max={presetKey === 'every_n_minutes' ? 59 : 23}
            value={nValue}
            onChange={handleNChange}
            style={{ width: 80 }}
            addonAfter={presetKey === 'every_n_minutes' ? t('cron.preset.unitMin') : t('cron.preset.unitHour')}
          />
        )}

        {/* 固定时间：时间选择器 */}
        {(presetKey === 'fixed_daily' || presetKey === 'fixed_weekly' || presetKey === 'fixed_monthly') && (
          <>
            {/* 每周：星期选择 */}
            {presetKey === 'fixed_weekly' && (
              <Select
                value={fixedDow}
                onChange={handleDowChange}
                style={{ width: 80 }}
                options={WEEK_OPTIONS}
                popupMatchSelectWidth={false}
              />
            )}
            {/* 每月：日期选择 */}
            {presetKey === 'fixed_monthly' && (
              <InputNumber
                min={1}
                max={31}
                value={fixedDom}
                onChange={handleDomChange}
                style={{ width: 72 }}
                addonAfter={t('cron.preset.unitDay')}
              />
            )}
            {/* 时间选择器 */}
            <TimePicker
              value={fixedTime}
              onChange={handleTimeChange}
              format="HH:mm"
              showSecond={false}
              style={{ width: 100 }}
              allowClear={false}
            />
          </>
        )}
      </Space.Compact>

      {/* 第二行：Cron 表达式输入框 */}
      <Input
        value={rawExpr}
        onChange={handleRawChange}
        placeholder="0 */5 * * * *"
        prefix={
        <Tooltip title={t('cron.preset.exprTip')}>
            <EditOutlined style={{ color: '#8c8c8c', fontSize: 13 }} />
          </Tooltip>
        }
        suffix={
          rawExpr ? (
            isValid
              ? <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 13 }} />
              : <Tooltip title={t('cron.preset.exprInvalid')}>
                  <WarningOutlined style={{ color: '#faad14', fontSize: 13 }} />
                </Tooltip>
          ) : null
        }
        style={{
          fontFamily: 'monospace',
          fontSize: 13,
          borderColor: rawExpr && !isValid ? '#faad14' : undefined,
        }}
      />

      {/* 第三行：说明文字 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t(`cron.preset.${presetKey}_desc`)}
        </Text>
        {rawExpr && isValid && (
          <Tag
            color="blue"
            style={{ fontFamily: 'monospace', fontSize: 11, margin: 0 }}
          >
            {rawExpr}
          </Tag>
        )}
        <Text type="secondary" style={{ fontSize: 11, marginLeft: 'auto', opacity: 0.6 }}>
          {t('cron.preset.exprFormat')}
        </Text>
      </div>
    </div>
  )
}

export default CronExprInput

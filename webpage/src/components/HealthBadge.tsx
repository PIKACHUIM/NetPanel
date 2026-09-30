import {useEffect, useState} from 'react'
import {Badge, Tooltip} from 'antd'
import {systemApi} from '../api'
import {useTranslation} from 'react-i18next'

/**
 * 顶栏健康徽标：定期轮询 /system/health，
 * 绿色=全部正常，红色=存在异常（DB 读写失败或引擎心跳过期）。
 * 轮询失败（网络/后端挂）也视为异常。
 *
 * 接口契约：后端 GetHealth 返回顶层结构，request.ts 拦截器已把整个 body
 * 解包给调用方，所以 res 就是 body 本身，checks 在顶层而非 res.data.checks。
 *   {"code": 200|503, "status": "healthy"|"unhealthy", "uptime": "...", "checks": {...}}
 * checks 的值以 "ok" 开头表示该项正常，否则为失败原因。
 */
export default function HealthBadge() {
    const {t} = useTranslation('health')
    const [healthy, setHealthy] = useState<boolean | null>(null)
    const [detail, setDetail] = useState<string>(t('checking'))

    const check = async () => {
        try {
            const res: any = await systemApi.getHealth()
            // 判定优先用后端 status 字段（healthy/unhealthy），其次 code
            const ok = res?.status === 'healthy' || res?.code === 200
            setHealthy(ok)
            // checks 在顶层（不是 res.data.checks）：后端返回的是
            // {"code":..., "status":..., "uptime":..., "checks": {...}}，
            // request.ts 拦截器 return data 已把整个 body 解包给调用方
            const checks: Record<string, string> = res?.checks || {}
            const bad = Object.entries(checks).filter(([, v]) => !v.startsWith('ok'))
            if (ok) {
                setDetail(t('allOk'))
            } else if (bad.length > 0) {
                setDetail(`${t('abnormal')}${bad.map(([k, v]) => `${k}（${v}）`).join('；')}`)
            } else {
                // 后端明确返回 unhealthy 但 checks 为空：取不到明细，不假装知道原因
                setDetail(t('abnormalUnknown'))
            }
        } catch (e: any) {
            setHealthy(false)
            // 区分"后端自检不通过"（503）与"真的不可达"：前者是明确信号，
            // 后者才是网络/后端挂掉。避免把 503 误述为"网络问题"。
            const status = e?.response?.status
            setDetail(status === 503 ? t('selfCheckFailed') : t('unreachable'))
        }
    }

    useEffect(() => {
        check()
        // 后台标签页持续轮询无意义；隐藏时暂停、恢复时立即补一次，
        // 避免挂 48 小时约 2880 次无意义请求
        const timer = setInterval(check, 60_000)
        const onVisibilityChange = () => {
            if (document.hidden) {
                clearInterval(timer)
            } else {
                check()
            }
        }
        document.addEventListener('visibilitychange', onVisibilityChange)
        return () => {
            clearInterval(timer)
            document.removeEventListener('visibilitychange', onVisibilityChange)
        }
    }, [])

    if (healthy === null) return null // 首次检测完成前不渲染，避免闪烁

    return (
        <Tooltip title={detail} placement="bottom">
            <div style={{display: 'flex', alignItems: 'center', padding: '0 6px', cursor: 'default'}}>
                <Badge
                    status={healthy ? 'success' : 'error'}
                    text={<span style={{fontSize: 11, opacity: 0.75}}>{healthy ? t('normal') : t('abnormal')}</span>}
                />
            </div>
        </Tooltip>
    )
}

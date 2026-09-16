import {useEffect, useState} from 'react'
import {Badge, Tooltip} from 'antd'
import {systemApi} from '../api'

/**
 * 顶栏健康徽标：定期轮询 /system/health，
 * 绿色=全部正常，红色=存在异常（DB 读写失败或引擎心跳过期）。
 * 轮询失败（网络/后端挂）也视为异常。
 */
export default function HealthBadge() {
    const [healthy, setHealthy] = useState<boolean | null>(null)
    const [detail, setDetail] = useState<string>('检测中…')

    const check = async () => {
        try {
            const res: any = await systemApi.getHealth()
            const ok = res?.code === 200
            setHealthy(ok)
            const checks: Record<string, string> = res?.data?.checks || {}
            const bad = Object.entries(checks).filter(([, v]) => !v.startsWith('ok'))
            setDetail(ok ? '系统正常' : `异常项：${bad.map(([k]) => k).join('、') || '未知'}`)
        } catch {
            setHealthy(false)
            setDetail('健康检查不可达（后端异常或网络问题）')
        }
    }

    useEffect(() => {
        check()
        const timer = setInterval(check, 60_000)
        return () => clearInterval(timer)
    }, [])

    if (healthy === null) return null // 首次检测完成前不渲染，避免闪烁

    return (
        <Tooltip title={detail} placement="bottom">
            <div style={{display: 'flex', alignItems: 'center', padding: '0 6px', cursor: 'default'}}>
                <Badge
                    status={healthy ? 'success' : 'error'}
                    text={<span style={{fontSize: 11, opacity: 0.75}}>{healthy ? '正常' : '异常'}</span>}
                />
            </div>
        </Tooltip>
    )
}

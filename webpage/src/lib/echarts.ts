// 共享 echarts 按需注册模块。
// 所有使用图表（懒加载路由内的监控页）统一从这里引入同一个 echarts 实例，
// 集合并集覆盖各页面用到的图表/组件，避免"漏注册一个组件就少一块功能"。
// 若新增图表类型，请优先在此追加注册，而不是在页面内重复 use()。
import * as echarts from 'echarts/core'
import { LineChart, ScatterChart, EffectScatterChart } from 'echarts/charts'
import { GeoComponent, GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([LineChart, ScatterChart, EffectScatterChart, GeoComponent, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

export default echarts
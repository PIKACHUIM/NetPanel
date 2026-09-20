# 前端依赖安装指南

## 📦 必需的 npm 包

### 1. 图表库（用于监控概览和探测结果）
```bash
npm install echarts echarts-for-react tslib
```

**用途：**
- `echarts` - 强大的数据可视化库
- `echarts-for-react` - ECharts 的 React 封装
- `tslib` - `echarts-for-react` 的 ESM 入口运行时辅助库，需显式声明
   （其自身未声明此依赖，缺少时 `vite build` 会报 `Rollup failed to resolve import "tslib"`）

**使用场景：**
- 监控概览页面的世界地图
- 服务探测页面的响应时间折线图
- 监控指标的各种图表

---

### 2. 终端组件（用于远程终端）
```bash
npm install xterm xterm-addon-fit xterm-addon-web-links
```

**用途：**
- `xterm` - 专业的终端模拟器
- `xterm-addon-fit` - 终端自适应窗口大小插件
- `xterm-addon-web-links` - 终端中的链接点击支持

**使用场景：**
- 远程终端页面的 SSH 终端
- WebSocket 实时终端交互

---

## 🚀 快速安装

### 一键安装所有依赖
```bash
cd webpage
npm install echarts echarts-for-react xterm xterm-addon-fit xterm-addon-web-links
```

---

## 📝 版本要求

| 包名 | 推荐版本 | 最低版本 |
|------|---------|---------|
| echarts | ^6.1.0 | 6.0.0 |
| echarts-for-react | ^3.0.0 | 3.0.0 |
| tslib | ^2.8.1 | 2.0.0 |
| xterm | ^5.3.0 | 5.0.0 |
| xterm-addon-fit | ^0.8.0 | 0.7.0 |
| xterm-addon-web-links | ^0.9.0 | 0.8.0 |

---

## 🔍 验证安装

安装完成后，检查 `package.json` 是否包含以下依赖：

```json
{
  "dependencies": {
    "echarts": "^6.1.0",
    "echarts-for-react": "^3.0.0",
    "tslib": "^2.8.1",
    "xterm": "^5.3.0",
    "xterm-addon-fit": "^0.8.0",
    "xterm-addon-web-links": "^0.9.0"
  }
}
```

---

## 🐛 常见问题

### 问题 1：xterm 样式丢失
**原因：** 没有导入 xterm 的 CSS 文件

**解决方案：**
在 `MonitorTerminal.tsx` 中确保导入：
```typescript
import 'xterm/css/xterm.css'
```

### 问题 2：ECharts 地图不显示
**原因：** 未注册地图组件，或未向 `ReactECharts` 传入 echarts 实例

**解决方案：**
本项目已按需引入 echarts。请**不要**使用 `import * as echarts from 'echarts'`（全量引入会把按需优化的体积加回去）。应统一从共享注册模块取实例，并**必须**把该实例作为 `echarts` prop 传给 `ReactECharts`：
```typescript
// 统一从共享模块取已按需注册的 echarts 实例
import echarts from '../lib/echarts'
import ReactECharts from 'echarts-for-react/lib/core'

// 使用前注册/加载地图（需在渲染前调用）
fetch(WORLD_MAP_URL)
  .then(r => r.json())
  .then(geo => { if (!echarts.getMap('world')) echarts.registerMap('world', geo) })

// 渲染时必须传 echarts={echarts}，否则图表空白（tsc/CI 拦不住）
<ReactECharts echarts={echarts} option={option} style={{ height: '500px' }} notMerge={true} lazyUpdate={true} />
```
> 注册地图与渲染必须使用同一个 echarts 实例，二者都来自共享模块；缺少组件（如 `LegendComponent`）时在 `webpage/src/lib/echarts.ts` 中补齐，而不是在页面内重复 `echarts.use()`。

### 问题 3：TypeScript 类型错误
**原因：** 缺少类型定义

**解决方案：**
```bash
npm install --save-dev @types/echarts @types/xterm
```

---

## 📚 相关文档

- [ECharts 官方文档](https://echarts.apache.org/zh/index.html)
- [xterm.js 官方文档](https://xtermjs.org/)
- [echarts-for-react GitHub](https://github.com/hustcc/echarts-for-react)

---

## ✅ 安装清单

完成以下步骤后，前端环境即可正常运行：

- [ ] 安装 echarts 和 echarts-for-react
- [ ] 安装 xterm 及其插件
- [ ] 验证 package.json 中的依赖
- [ ] 运行 `npm run dev` 测试
- [ ] 访问监控页面确认功能正常

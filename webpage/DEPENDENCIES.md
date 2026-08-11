# 前端依赖安装指南

## 📦 必需的 npm 包

### 1. 图表库（用于监控概览和探测结果）
```bash
npm install echarts echarts-for-react
```

**用途：**
- `echarts` - 强大的数据可视化库
- `echarts-for-react` - ECharts 的 React 封装

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
| echarts | ^5.4.0 | 5.0.0 |
| echarts-for-react | ^3.0.0 | 3.0.0 |
| xterm | ^5.3.0 | 5.0.0 |
| xterm-addon-fit | ^0.8.0 | 0.7.0 |
| xterm-addon-web-links | ^0.9.0 | 0.8.0 |

---

## 🔍 验证安装

安装完成后，检查 `package.json` 是否包含以下依赖：

```json
{
  "dependencies": {
    "echarts": "^5.4.0",
    "echarts-for-react": "^3.0.0",
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
**原因：** 没有注册地图组件

**解决方案：**
在使用 ECharts 前注册地图：
```typescript
import * as echarts from 'echarts'
import 'echarts/map/js/world'
```

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

import React, { Suspense, lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Spin } from 'antd'
import MainLayout from './layouts/MainLayout'
import LoginPage from './pages/Login'
import { useAppStore } from './store/appStore'

// 懒加载页面
const Dashboard = lazy(() => import('./pages/Dashboard'))
const PortForward = lazy(() => import('./pages/PortForward'))
const Stun = lazy(() => import('./pages/Stun'))
const FrpClient = lazy(() => import('./pages/FrpClient'))
const FrpServer = lazy(() => import('./pages/FrpServer'))
const NpsServer = lazy(() => import('./pages/NpsServer'))
const NpsClient = lazy(() => import('./pages/NpsClient'))
const EasytierClient = lazy(() => import('./pages/EasytierClient'))
const EasytierServer = lazy(() => import('./pages/EasytierServer'))
const CfTunnel = lazy(() => import('./pages/CfTunnel'))
const TunService = lazy(() => import('./pages/TunService'))
const Wireguard = lazy(() => import('./pages/Wireguard'))
const Ddns = lazy(() => import('./pages/Ddns'))
const Caddy = lazy(() => import('./pages/Caddy'))
const Wol = lazy(() => import('./pages/Wol'))
const DomainAccount = lazy(() => import('./pages/DomainAccount'))
const CertAccount = lazy(() => import('./pages/CertAccount'))
const DomainCert = lazy(() => import('./pages/DomainCert'))
const DomainInfo = lazy(() => import('./pages/DomainRecord'))
const DomainRecordDetail = lazy(() => import('./pages/DomainRecordDetail'))
const Waf = lazy(() => import('./pages/Waf'))
const Firewall = lazy(() => import('./pages/Firewall'))
const Dnsmasq = lazy(() => import('./pages/Dnsmasq'))
const Cron = lazy(() => import('./pages/Cron'))
const Storage = lazy(() => import('./pages/Storage'))
const IpDb = lazy(() => import('./pages/IpDb'))
const Access = lazy(() => import('./pages/Access'))
const CallbackAccount = lazy(() => import('./pages/CallbackAccount'))
const CallbackTask = lazy(() => import('./pages/CallbackTask'))
const Settings = lazy(() => import('./pages/Settings'))
const SystemLogs = lazy(() => import('./pages/SystemLogs'))
const UserManagement = lazy(() => import('./pages/UserManagement'))
const MeshNodes = lazy(() => import('./pages/MeshNodes'))
const MeshTunnels = lazy(() => import('./pages/MeshTunnels'))
const MeshTopology = lazy(() => import('./pages/MeshTopology'))
const MeshEvents = lazy(() => import('./pages/MeshEvents'))
const OAuthProviders = lazy(() => import('./pages/OAuthProviders'))
const OAuthCallback = lazy(() => import('./pages/OAuthCallback'))
const AiChat = lazy(() => import('./pages/AiChat'))
const AiAssistant = lazy(() => import('./pages/AiAssistant'))
const AiCronTask = lazy(() => import('./pages/AiCronTask'))
const AiPlugin = lazy(() => import('./pages/AiPlugin'))
const AiProvider = lazy(() => import('./pages/AiProvider'))
const MonitorDashboard = lazy(() => import('./pages/MonitorDashboard'))
const MonitorServers = lazy(() => import('./pages/MonitorServers'))
const MonitorProbes = lazy(() => import('./pages/MonitorProbes'))
const MonitorTasks = lazy(() => import('./pages/MonitorTasks'))
const MonitorAlerts = lazy(() => import('./pages/MonitorAlerts'))
const MonitorTerminal = lazy(() => import('./pages/MonitorTerminal'))
const MonitorDDNS = lazy(() => import('./pages/MonitorDDNS'))
const MonitorNotifications = lazy(() => import('./pages/MonitorNotifications'))
const MonitorTunnels = lazy(() => import('./pages/MonitorTunnels'))

const PageLoader: React.FC = () => (
  <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%', minHeight: 300 }}>
    <Spin size="large" />
  </div>
)

// 路由守卫
const PrivateRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { token } = useAppStore()
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

const App: React.FC = () => {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/oauth/callback" element={<Suspense fallback={<PageLoader />}><OAuthCallback /></Suspense>} />
      <Route
        path="/"
        element={
          <PrivateRoute>
            <MainLayout />
          </PrivateRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Suspense fallback={<PageLoader />}><Dashboard /></Suspense>} />
        <Route path="port-forward" element={<Suspense fallback={<PageLoader />}><PortForward /></Suspense>} />
        <Route path="stun" element={<Suspense fallback={<PageLoader />}><Stun /></Suspense>} />
        <Route path="frp/client" element={<Suspense fallback={<PageLoader />}><FrpClient /></Suspense>} />
        <Route path="frp/server" element={<Suspense fallback={<PageLoader />}><FrpServer /></Suspense>} />
        <Route path="nps/server" element={<Suspense fallback={<PageLoader />}><NpsServer /></Suspense>} />
        <Route path="nps/client" element={<Suspense fallback={<PageLoader />}><NpsClient /></Suspense>} />
        <Route path="easytier/client" element={<Suspense fallback={<PageLoader />}><EasytierClient /></Suspense>} />
        <Route path="easytier/server" element={<Suspense fallback={<PageLoader />}><EasytierServer /></Suspense>} />
        <Route path="cftunnel" element={<Suspense fallback={<PageLoader />}><CfTunnel /></Suspense>} />
        <Route path="tunservice" element={<Suspense fallback={<PageLoader />}><TunService /></Suspense>} />
        <Route path="wireguard" element={<Suspense fallback={<PageLoader />}><Wireguard /></Suspense>} />
        <Route path="ddns" element={<Suspense fallback={<PageLoader />}><Ddns /></Suspense>} />
        <Route path="caddy" element={<Suspense fallback={<PageLoader />}><Caddy /></Suspense>} />
        <Route path="wol" element={<Suspense fallback={<PageLoader />}><Wol /></Suspense>} />
        <Route path="domain/account" element={<Suspense fallback={<PageLoader />}><DomainAccount /></Suspense>} />
        <Route path="domain/cert-account" element={<Suspense fallback={<PageLoader />}><CertAccount /></Suspense>} />
        <Route path="domain/cert" element={<Suspense fallback={<PageLoader />}><DomainCert /></Suspense>} />
        <Route path="domain/info" element={<Suspense fallback={<PageLoader />}><DomainInfo /></Suspense>} />
        <Route path="domain/info/:domainInfoId/records" element={<Suspense fallback={<PageLoader />}><DomainRecordDetail /></Suspense>} />
        <Route path="security/waf" element={<Suspense fallback={<PageLoader />}><Waf /></Suspense>} />
        <Route path="security/firewall" element={<Suspense fallback={<PageLoader />}><Firewall /></Suspense>} />
        <Route path="dnsmasq" element={<Suspense fallback={<PageLoader />}><Dnsmasq /></Suspense>} />
        <Route path="cron" element={<Suspense fallback={<PageLoader />}><Cron /></Suspense>} />
        <Route path="storage" element={<Suspense fallback={<PageLoader />}><Storage /></Suspense>} />
        <Route path="ipdb" element={<Suspense fallback={<PageLoader />}><IpDb /></Suspense>} />
        <Route path="access" element={<Suspense fallback={<PageLoader />}><Access /></Suspense>} />
        <Route path="callback/account" element={<Suspense fallback={<PageLoader />}><CallbackAccount /></Suspense>} />
        <Route path="callback/task" element={<Suspense fallback={<PageLoader />}><CallbackTask /></Suspense>} />
        <Route path="settings" element={<Suspense fallback={<PageLoader />}><Settings /></Suspense>} />
        <Route path="admin/logs" element={<Suspense fallback={<PageLoader />}><SystemLogs /></Suspense>} />
        <Route path="admin/users" element={<Suspense fallback={<PageLoader />}><UserManagement /></Suspense>} />
        <Route path="admin/oauth-providers" element={<Suspense fallback={<PageLoader />}><OAuthProviders /></Suspense>} />
        <Route path="mesh/nodes" element={<Suspense fallback={<PageLoader />}><MeshNodes /></Suspense>} />
        <Route path="mesh/tunnels" element={<Suspense fallback={<PageLoader />}><MeshTunnels /></Suspense>} />
        <Route path="mesh/topology" element={<Suspense fallback={<PageLoader />}><MeshTopology /></Suspense>} />
        <Route path="mesh/events" element={<Suspense fallback={<PageLoader />}><MeshEvents /></Suspense>} />
        <Route path="ai/chat" element={<Suspense fallback={<PageLoader />}><AiChat /></Suspense>} />
        <Route path="ai/assistant" element={<Suspense fallback={<PageLoader />}><AiAssistant /></Suspense>} />
        <Route path="ai/cron-task" element={<Suspense fallback={<PageLoader />}><AiCronTask /></Suspense>} />
        <Route path="ai/plugin" element={<Suspense fallback={<PageLoader />}><AiPlugin /></Suspense>} />
        <Route path="ai/provider" element={<Suspense fallback={<PageLoader />}><AiProvider /></Suspense>} />
        <Route path="monitor/dashboard" element={<Suspense fallback={<PageLoader />}><MonitorDashboard /></Suspense>} />
        <Route path="monitor/servers" element={<Suspense fallback={<PageLoader />}><MonitorServers /></Suspense>} />
        <Route path="monitor/probes" element={<Suspense fallback={<PageLoader />}><MonitorProbes /></Suspense>} />
        <Route path="monitor/tasks" element={<Suspense fallback={<PageLoader />}><MonitorTasks /></Suspense>} />
        <Route path="monitor/alerts" element={<Suspense fallback={<PageLoader />}><MonitorAlerts /></Suspense>} />
        <Route path="monitor/terminal" element={<Suspense fallback={<PageLoader />}><MonitorTerminal /></Suspense>} />
        <Route path="monitor/ddns" element={<Suspense fallback={<PageLoader />}><MonitorDDNS /></Suspense>} />
        <Route path="monitor/notifications" element={<Suspense fallback={<PageLoader />}><MonitorNotifications /></Suspense>} />
        <Route path="monitor/tunnels" element={<Suspense fallback={<PageLoader />}><MonitorTunnels /></Suspense>} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

export default App

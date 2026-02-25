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
const Ddns = lazy(() => import('./pages/Ddns'))
const Caddy = lazy(() => import('./pages/Caddy'))
const Wol = lazy(() => import('./pages/Wol'))
const DomainAccount = lazy(() => import('./pages/DomainAccount'))
const DomainCert = lazy(() => import('./pages/DomainCert'))
const DomainRecord = lazy(() => import('./pages/DomainRecord'))
const Dnsmasq = lazy(() => import('./pages/Dnsmasq'))
const Cron = lazy(() => import('./pages/Cron'))
const Storage = lazy(() => import('./pages/Storage'))
const IpDb = lazy(() => import('./pages/IpDb'))
const Access = lazy(() => import('./pages/Access'))
const CallbackAccount = lazy(() => import('./pages/CallbackAccount'))
const CallbackTask = lazy(() => import('./pages/CallbackTask'))
const Settings = lazy(() => import('./pages/Settings'))

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
        <Route path="ddns" element={<Suspense fallback={<PageLoader />}><Ddns /></Suspense>} />
        <Route path="caddy" element={<Suspense fallback={<PageLoader />}><Caddy /></Suspense>} />
        <Route path="wol" element={<Suspense fallback={<PageLoader />}><Wol /></Suspense>} />
        <Route path="domain/account" element={<Suspense fallback={<PageLoader />}><DomainAccount /></Suspense>} />
        <Route path="domain/cert" element={<Suspense fallback={<PageLoader />}><DomainCert /></Suspense>} />
        <Route path="domain/record" element={<Suspense fallback={<PageLoader />}><DomainRecord /></Suspense>} />
        <Route path="dnsmasq" element={<Suspense fallback={<PageLoader />}><Dnsmasq /></Suspense>} />
        <Route path="cron" element={<Suspense fallback={<PageLoader />}><Cron /></Suspense>} />
        <Route path="storage" element={<Suspense fallback={<PageLoader />}><Storage /></Suspense>} />
        <Route path="ipdb" element={<Suspense fallback={<PageLoader />}><IpDb /></Suspense>} />
        <Route path="access" element={<Suspense fallback={<PageLoader />}><Access /></Suspense>} />
        <Route path="callback/account" element={<Suspense fallback={<PageLoader />}><CallbackAccount /></Suspense>} />
        <Route path="callback/task" element={<Suspense fallback={<PageLoader />}><CallbackTask /></Suspense>} />
        <Route path="settings" element={<Suspense fallback={<PageLoader />}><Settings /></Suspense>} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}

export default App

import { useAppStore } from '../store/appStore'

/**
 * 带鉴权头的 fetch。
 *
 * 面板接口均需要 Authorization: Bearer <token>。少数场景（例如需要流式读取、
 * 或在统一 axios 封装之外调用）会直接使用 fetch，此前漏带鉴权头，
 * 一旦后端的公开/受限路由边界发生变化就会静默失效（返回 401 而非报错）。
 * 统一走本函数可确保 token 始终被携带。
 */
export function authFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const token = useAppStore.getState().token
  const headers = new Headers(init.headers || {})
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  return fetch(input, { ...init, headers })
}

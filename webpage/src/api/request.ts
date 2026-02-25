import axios from 'axios'
import { message } from 'antd'
import { useAppStore } from '../store/appStore'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// 请求拦截器：添加 Token
request.interceptors.request.use(
  (config) => {
    const token = useAppStore.getState().token
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// 响应拦截器：统一错误处理
request.interceptors.response.use(
  (response) => {
    const data = response.data
    if (data.code !== undefined && data.code !== 0 && data.code !== 200) {
      message.error(data.message || '请求失败')
      return Promise.reject(new Error(data.message))
    }
    return data
  },
  (error) => {
    if (error.response?.status === 401) {
      useAppStore.getState().logout()
      window.location.href = '/login'
    } else {
      message.error(error.response?.data?.message || error.message || '网络错误')
    }
    return Promise.reject(error)
  }
)

export default request

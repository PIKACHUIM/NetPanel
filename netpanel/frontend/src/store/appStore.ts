import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AppState {
  token: string | null
  username: string | null
  language: 'zh' | 'en'
  theme: 'light' | 'dark'
  collapsed: boolean
  setToken: (token: string | null) => void
  setUsername: (username: string | null) => void
  setLanguage: (lang: 'zh' | 'en') => void
  setTheme: (theme: 'light' | 'dark') => void
  setCollapsed: (collapsed: boolean) => void
  logout: () => void
}

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      language: 'zh',
      theme: 'light',
      collapsed: false,
      setToken: (token) => set({ token }),
      setUsername: (username) => set({ username }),
      setLanguage: (language) => set({ language }),
      setTheme: (theme) => set({ theme }),
      setCollapsed: (collapsed) => set({ collapsed }),
      logout: () => set({ token: null, username: null }),
    }),
    {
      name: 'netpanel-store',
      partialize: (state) => ({
        token: state.token,
        username: state.username,
        language: state.language,
        theme: state.theme,
      }),
    }
  )
)

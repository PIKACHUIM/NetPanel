import { create } from 'zustand'
import { persist } from 'zustand/middleware'

// UI 模式：亮色 / 暗色
export type UIMode = 'light' | 'dark'

// 壁纸主题：none=简约无壁纸，其他为动漫壁纸
export type WallpaperKey = 'none' | 'march7' | 'firefly' | 'nahida' | 'gura' | 'umaru' | 'pikachu' | 'columbia'

// 兼容旧 ThemeMode（用于迁移）
export type ThemeMode = UIMode | 'glass-light' | 'glass-dark' | WallpaperKey

// 壁纸配置（含专属主题色）
export const wallpaperList = [
  { key: 'none' as const, name: '简约UI', icon: '🎯', color: '#1677ff' },
  { key: 'march7' as const, name: '三月七', icon: '❄️', color: '#7cc5ea', pc: '/wallpapers/march7/pc.png', mobile: '/wallpapers/march7/phone.png' },
  { key: 'firefly' as const, name: '流萤🌟', icon: '🦋', color: '#f59e0b', pc: '/wallpapers/firefly/pc.jpg', mobile: '/wallpapers/firefly/pc.jpg' },
  { key: 'nahida' as const, name: '纳西妲', icon: '🌿', color: '#4ade80', pc: '/wallpapers/nahida/pc.jpg', mobile: '/wallpapers/nahida/pc.jpg' },
  { key: 'gura' as const, name: '鲨鲨妹', icon: '🦈', color: '#38bdf8', pc: '/wallpapers/gura/pc.jpg', mobile: '/wallpapers/gura/pc.jpg' },
  { key: 'umaru' as const, name: '干物妹', icon: '🍪', color: '#fb923c', pc: '/wallpapers/umaru/pc.jpg', mobile: '/wallpapers/umaru/pc.jpg' },
  { key: 'pikachu' as const, name: '皮卡丘', icon: '🌩️', color: '#facc15', pc: '/wallpapers/pikachu/pc.jpeg', mobile: '/wallpapers/pikachu/phone.jpg' },
  { key: 'columbia' as const, name: '少女🕊️', icon: '🎵', color: '#f472b6', pc: '/wallpapers/columbia/pc.webp', mobile: '/wallpapers/columbia/pc.webp' },
]

// 获取当前壁纸的主题色
export function getWallpaperColor(wallpaper: WallpaperKey): string {
  const w = wallpaperList.find(item => item.key === wallpaper)
  return w?.color || '#1677ff'
}

// 兼容旧代码的 animeThemes 导出
export const animeThemes = wallpaperList.filter(w => w.key !== 'none') as Array<{ key: string; name: string; icon: string; pc: string; mobile: string }>

// 判断是否有壁纸
export function hasWallpaper(wallpaper: WallpaperKey): boolean {
  return wallpaper !== 'none'
}

// 兼容旧代码
export function isAnimeTheme(theme: ThemeMode): boolean {
  return wallpaperList.some(w => w.key !== 'none' && w.key === theme)
}

// 获取壁纸背景图 URL
export function getWallpaperBg(wallpaper: WallpaperKey): string | null {
  if (wallpaper === 'none') return null
  const w = wallpaperList.find(item => item.key === wallpaper)
  if (!w || !('pc' in w)) return null
  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768
  return isMobile ? w.mobile : w.pc
}

// 兼容旧代码
export function getAnimeThemeBg(theme: ThemeMode): string | null {
  return getWallpaperBg(theme as WallpaperKey)
}

interface AppState {
  token: string | null
  username: string | null
  language: 'zh' | 'en'
  // 新主题系统：UI 模式 + 壁纸 独立选择
  uiMode: UIMode
  wallpaper: WallpaperKey
  // 兼容旧代码
  theme: ThemeMode
  collapsed: boolean
  setToken: (token: string | null) => void
  setUsername: (username: string | null) => void
  setLanguage: (lang: 'zh' | 'en') => void
  setUIMode: (mode: UIMode) => void
  setWallpaper: (wp: WallpaperKey) => void
  setTheme: (theme: ThemeMode) => void
  setCollapsed: (collapsed: boolean) => void
  logout: () => void
}

export const useAppStore = create<AppState>()(
  persist(
    (set) => ({
      token: null,
      username: null,
      language: 'zh',
      uiMode: 'dark',
      wallpaper: 'none',
      theme: 'dark',
      collapsed: false,
      setToken: (token) => set({ token }),
      setUsername: (username) => set({ username }),
      setLanguage: (language) => set({ language }),
      setUIMode: (uiMode) => set({ uiMode, theme: uiMode }),
      setWallpaper: (wallpaper) => set((state) => ({
        wallpaper,
        theme: wallpaper === 'none' ? state.uiMode : wallpaper,
      })),
      setTheme: (theme) => {
        // 兼容旧调用：自动推断 uiMode 和 wallpaper
        if (theme === 'light' || theme === 'dark') {
          set({ theme, uiMode: theme, wallpaper: 'none' })
        } else if (theme === 'glass-light') {
          set({ theme: 'light', uiMode: 'light', wallpaper: 'none' })
        } else if (theme === 'glass-dark') {
          set({ theme: 'dark', uiMode: 'dark', wallpaper: 'none' })
        } else {
          // 动漫壁纸
          set((state) => ({ theme, wallpaper: theme as WallpaperKey }))
        }
      },
      setCollapsed: (collapsed) => set({ collapsed }),
      logout: () => set({ token: null, username: null }),
    }),
    {
      name: 'netpanel-store',
      partialize: (state) => ({
        token: state.token,
        username: state.username,
        language: state.language,
        uiMode: state.uiMode,
        wallpaper: state.wallpaper,
        theme: state.theme,
      }),
    }
  )
)

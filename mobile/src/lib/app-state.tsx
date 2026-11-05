import * as Location from 'expo-location'
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { DEMO_POSITION, inDataBounds } from '@/data/properties'
import type { Point } from './geometry'

const strings = {
  en: { home: 'Home', map: 'Map', mic: 'Talk', open: 'Open home', me: 'Me', youAt: "You're at", held: 'Held', years: 'years', holdTalk: 'Hold to talk', property: 'Property', owner: 'Owner', tag: 'Tag', note: 'Note', photo: 'Photo', logCall: 'Log call', zoom: 'Zoom in to see properties', near: 'Near me', demo: 'Demo position · 11 Gager St', language: 'Language', soon: 'Coming soon', contact: 'Contact owner', save: 'Save note' },
  zh: { home: '首页', map: '地图', mic: '语音', open: '开放日', me: '我的', youAt: '当前位置', held: '持有', years: '年', holdTalk: '按住说话', property: '房产', owner: '业主', tag: '标签', note: '笔记', photo: '照片', logCall: '通话记录', zoom: '放大以查看房产', near: '附近', demo: '演示位置 · Gager St 11号', language: '语言', soon: '即将推出', contact: '联系业主', save: '保存笔记' },
} as const
export type Language = keyof typeof strings
export type Key = keyof typeof strings.en
const Context = createContext<{ language: Language; setLanguage: (l: Language) => void; t: (key: Key) => string; position: Point; demo: boolean; locate: () => Promise<void> } | null>(null)
export function AppProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<Language>('en')
  const [position, setPosition] = useState<Point>(DEMO_POSITION)
  const [demo, setDemo] = useState(true)
  async function locate() {
    try {
      const permission = await Location.requestForegroundPermissionsAsync()
      if (!permission.granted) return
      const location = await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced })
      const p = { lat: location.coords.latitude, lng: location.coords.longitude }
      setDemo(!inDataBounds(p)); setPosition(inDataBounds(p) ? p : DEMO_POSITION)
    } catch { setDemo(true); setPosition(DEMO_POSITION) }
  }
  useEffect(() => { const timer = setTimeout(() => { void locate() }, 0); return () => clearTimeout(timer) }, [])
  return <Context.Provider value={{ language, setLanguage, t: key => strings[language][key], position, demo, locate }}>{children}</Context.Provider>
}
export function useApp() {
  const value = useContext(Context)
  if (!value) throw new Error('AppProvider missing')
  return value
}

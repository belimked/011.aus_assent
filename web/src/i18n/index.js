// 轻量 i18n：以简体中文原文作为 key
//   简体 zh-CN：原文
//   繁體 zh-HK：OpenCC 自动转换（香港用字，贴近粤语用户），按需懒加载
//   English  ：src/i18n/en/*.js 词典；缺失时回退原文并记录到 missing
import { ref } from 'vue'

export const LOCALES = [
  { k: 'zh-CN', l: '简体中文', short: '简' },
  { k: 'zh-HK', l: '繁體中文', short: '繁' },
  { k: 'en', l: 'English', short: 'EN' },
]
export const locale = ref('zh-CN')

const EN = Object.assign({}, ...Object.values(import.meta.glob('./en/*.js', { eager: true })).map(m => m.default))
export const missing = new Set()
let conv = null
const cache = new Map()
const fill = (s, p) => p ? s.replace(/\{(\w+)\}/g, (_, k) => p[k] ?? '') : s

/** 翻译：t('已登记 {n} 人', { n: 5 }) */
export function t(s, p) {
  if (s == null || s === '') return s
  const l = locale.value // 建立响应式依赖
  let out = s
  if (l === 'en') {
    out = EN[s]
    // 已拼好数字的文本（如 'OCR 13 行'）回退到 '{n}' 模板
    if (out === undefined) { const m = s.match(/\d+(?:\.\d+)?/); if (m && EN[s.replace(m[0], '{n}')] !== undefined) out = fill(EN[s.replace(m[0], '{n}')], { n: m[0] }) }
    if (out === undefined) { if (/[\u4e00-\u9fff]/.test(s)) missing.add(s); out = s }
  } else if (l === 'zh-HK' && conv) {
    out = cache.get(s)
    if (out === undefined) { out = conv(s); cache.set(s, out) }
  }
  return fill(out, p)
}

/** 翻译已经拼好的日志类文本：整句命中优先，否则按分隔符逐段翻译 */
export function tx(s) {
  if (!s || locale.value === 'zh-CN') return t(s)
  if (locale.value === 'zh-HK') return t(s)
  if (EN[s] !== undefined) return EN[s]
  return s.split(/(\s·\s|；|、|，)/).map(seg => {
    if (/^(\s·\s|；|、|，)$/.test(seg)) return { '；': '; ', '、': ', ', '，': ', ' }[seg] || seg
    if (EN[seg.trim()] !== undefined) return EN[seg.trim()]
    const num = seg.trim().match(/\d+(?:\.\d+)?/)
    if (num) { const k = seg.trim().replace(num[0], '{n}'); if (EN[k] !== undefined) return fill(EN[k], { n: num[0] }) }
    const m = seg.match(/^([^：:]+)[：:]\s*(.*)$/)
    if (m && EN[m[1]] !== undefined) return `${EN[m[1]]}: ${EN[m[2]] ?? m[2]}`
    if (/[\u4e00-\u9fff]/.test(seg)) missing.add(seg)
    return seg
  }).join('')
}

export async function setLocale(l) {
  if (l === 'zh-HK' && !conv) {
    const OpenCC = await import('opencc-js/cn2t')
    conv = (OpenCC.default || OpenCC).Converter({ from: 'cn', to: 'hk' })
  }
  locale.value = l
  localStorage.setItem('lang', l)
  document.documentElement.lang = l
}
export const isEn = () => locale.value === 'en'

// 调试：window.__i18nMissing() 查看英文缺失项
if (typeof window !== 'undefined') window.__i18nMissing = () => [...missing]

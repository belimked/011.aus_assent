// 驾驶舱 / 工作台指标计算
import { state, viewStatus, userById, now, startOfDay, DAY, TALK, SUBURBS, NOW0 } from './store'

export const PERIODS = [
  { k: 'today', l: '今天', days: 1 },
  { k: '7d', l: '近 7 天', days: 7 },
  { k: '30d', l: '近 30 天', days: 30 },
  { k: '90d', l: '近 90 天', days: 90 },
]
export function range(k) {
  const d = PERIODS.find(p => p.k === k).days
  const end = now(), start = new Date(startOfDay(end).getTime() - (d - 1) * DAY)
  const len = end - start
  return { start, end, prevStart: new Date(start - len), prevEnd: start, days: d }
}
const inR = (a, s, e) => a.at >= s && a.at < e

export function kpis(period, agents) {
  const r = range(period)
  const pick = (s, e) => state.acts.filter(a => inR(a, s, e) && agents.includes(a.agent))
  const cur = pick(r.start, r.end), prev = pick(r.prevStart, r.prevEnd)
  const calc = L => {
    const knocks = L.filter(a => a.type === 'knock')
    return {
      knocks: knocks.length,
      talks: knocks.filter(a => TALK.has(a.result)).length,
      captured: knocks.filter(a => a.captured).length,
      linked: knocks.filter(a => a.linked).length,
      appraisals: knocks.filter(a => a.result === '预约估价').length,
      listings: L.filter(a => a.type === 'listing').length,
      ofi: L.filter(a => a.type === 'ofi').length,
      calls: L.filter(a => a.type === 'call').length,
    }
  }
  return { cur: calc(cur), prev: calc(prev), list: cur }
}
export const delta = (c, p) => p ? Math.round((c - p) / p * 100) : (c ? 100 : 0)

export function coverage(suburb) {
  const L = state.props.filter(p => !suburb || p.suburb === suburb)
  const c = { green: 0, red: 0, grey: 0, other: 0 }
  L.forEach(p => c[viewStatus(p)]++)
  const total = L.length
  return { ...c, total, pct: total ? Math.round(c.green / total * 100) : 0 }
}
export const suburbCoverage = () => SUBURBS.map(s => ({ suburb: s, ...coverage(s) }))

export function trend(period, agents) {
  const r = range(period)
  const days = Math.max(r.days, 7)
  const out = []
  for (let i = days - 1; i >= 0; i--) {
    const s = new Date(startOfDay(now()).getTime() - i * DAY), e = new Date(s.getTime() + DAY)
    const L = state.acts.filter(a => inR(a, s, e) && agents.includes(a.agent))
    out.push({ d: s, knocks: L.filter(a => a.type === 'knock').length, talks: L.filter(a => a.type === 'knock' && TALK.has(a.result)).length, captured: L.filter(a => a.captured).length })
  }
  return out
}
export function heat(period, agents) {
  const r = range(period === 'today' ? '30d' : period)
  const m = {}
  state.acts.filter(a => a.type === 'knock' && inR(a, r.start, r.end) && agents.includes(a.agent)).forEach(a => {
    const k = a.at.getDay() + '-' + a.at.getHours(); m[k] = m[k] || { n: 0, t: 0 }; m[k].n++; if (TALK.has(a.result)) m[k].t++
  })
  return m
}
export function leaderboard(period, agents) {
  const r = range(period)
  return agents.map(id => {
    const L = state.acts.filter(a => a.agent === id && inR(a, r.start, r.end))
    const k = L.filter(a => a.type === 'knock')
    const talks = k.filter(a => TALK.has(a.result)).length
    return { id, user: userById(id), knocks: k.length, talks, rate: k.length ? Math.round(talks / k.length * 100) : 0, captured: L.filter(a => a.captured).length, appraisals: k.filter(a => a.result === '预约估价').length, listings: L.filter(a => a.type === 'listing').length, linked: k.filter(a => a.linked).length }
  }).sort((a, b) => b.knocks + b.appraisals * 10 - (a.knocks + a.appraisals * 10))
}
export function resultMix(list) {
  const m = {}
  list.filter(a => a.type === 'knock').forEach(a => m[a.result] = (m[a.result] || 0) + 1)
  return Object.entries(m).map(([name, value]) => ({ name, value })).sort((a, b) => b.value - a.value)
}
// 潜在卖家机会：未关联、持有 >=10 年、未设免打扰，近 60 天没拜访
export function opportunities(limit = 8) {
  return state.props.filter(p => viewStatus(p) === 'red' && p.tp && !p.flags.dnk && (NOW0 - p.tp.saleDate) / (365.25 * DAY) >= 10 && (!p.lastVisit || now() - p.lastVisit > 7 * DAY))
    .map(p => { const y = Math.floor((NOW0 - p.tp.saleDate) / (365.25 * DAY)), idle = p.lastVisit ? Math.min(30, (now() - p.lastVisit) / DAY) : 30; return { p, years: y, score: Math.min(99, Math.round(30 + y * 1.6 + idle * .5 + (p.tags.length ? 12 : 0))) } })
    .sort((a, b) => b.score - a.score).slice(0, limit)
}
export const fmtNum = n => n.toLocaleString('en-AU')

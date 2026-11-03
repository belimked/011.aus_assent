// 演示数据 + 全局状态（纯前端模拟，不连接 Rex）
import { reactive, computed } from 'vue'
import { t, locale } from '../i18n'

let seed = 20261103
const rnd = () => (seed = (seed * 16807) % 2147483647, (seed - 1) / 2147483646)
const pick = a => a[Math.floor(rnd() * a.length)]
const ri = (a, b) => a + Math.floor(rnd() * (b - a + 1))

const BOOT = Date.now()
export const NOW0 = new Date(2026, 10, 3, 14, 20)
export const now = () => new Date(NOW0.getTime() + (Date.now() - BOOT))
const DAY = 864e5
const startOfDay = d => new Date(d.getFullYear(), d.getMonth(), d.getDate())

/* ---------------- 字典 ---------------- */
export const STATUS = {
  green: { label: '已关联业主', short: '已关联', color: '#16a34a', bg: '#dcfce7' },
  red: { label: '未关联业主', short: '未关联', color: '#dc2626', bg: '#fee2e2' },
  grey: { label: '待地址匹配', short: '待匹配', color: '#9ca3af', bg: '#f3f4f6' },
  other: { label: '其他团队跟进', short: '他人跟进', color: '#8b5cf6', bg: '#ede9fe' },
}
export const RESULTS = ['无人应门', '留名片', '与业主交谈', '租客应门', '不感兴趣', '预约估价']
export const TALK = new Set(['与业主交谈', '租客应门', '预约估价'])
export const ACT = {
  knock: { label: '上门拜访', icon: 'House', color: '#0e7490' },
  call: { label: '电话回访', icon: 'Phone', color: '#2563eb' },
  ofi: { label: '开放参观登记', icon: 'Key', color: '#b8860b' },
  photo: { label: '拍照录入', icon: 'Camera', color: '#7c3aed' },
  listing: { label: '签约委托', icon: 'Trophy', color: '#16a34a' },
  note: { label: '添加笔记', icon: 'EditPen', color: '#64748b' },
}

export const USERS = reactive([
  { id: 'ping', name: 'Ping Han', role: 'admin', roleLabel: '管理员', team: '办公室', device: 'Web', color: '#b8860b', status: 'active', last: '在线' },
  { id: 'owen', name: 'Owen Chen', role: 'leader', roleLabel: '团队负责人', team: 'Owen 团队', device: 'iPhone 16 Pro', color: '#0e7490', status: 'active', last: '1 小时前' },
  { id: 'kevin', name: 'Kevin Zhang', role: 'agent', roleLabel: '经纪人', team: 'Owen 团队', device: 'iPhone 15', color: '#2563eb', status: 'active', last: '在线' },
  { id: 'natalie', name: 'Natalie Wong', role: 'agent', roleLabel: '经纪人', team: 'Owen 团队', device: 'iPhone 13', color: '#db2777', status: 'active', last: '12 分钟前' },
  { id: 'lily', name: 'Lily Ho', role: 'leader', roleLabel: '团队负责人', team: 'Lily 团队', device: 'iPhone 15 Pro', color: '#7c3aed', status: 'active', last: '35 分钟前' },
  { id: 'jason', name: 'Jason Wu', role: 'agent', roleLabel: '经纪人', team: 'Lily 团队', device: 'iPhone 14', color: '#ea580c', status: 'active', last: '3 分钟前' },
  { id: 'sarah', name: 'Sarah Liu', role: 'assistant', roleLabel: '助理', team: 'Lily 团队', device: 'iPhone SE', color: '#16a34a', status: 'active', last: '2 天前' },
])
export const userById = id => USERS.find(u => u.id === id)
export const initials = n => n.split(' ').map(s => s[0]).join('').slice(0, 2)
const FIELD = USERS.filter(u => u.id !== 'ping').map(u => u.id)

const NAMES = ['Mary Chen', 'David Liu', 'Grace Huang', 'Peter Nguyen', 'Linda Wang', 'James Smith', 'Karen Lee', 'Michael Brown', 'Helen Zhou', 'Tony Tran', 'Susan Taylor', 'Wei Zhang', 'Emily Wilson', 'Jack Ho', 'Anna Lin', 'Robert Jones', 'Cindy Wu', 'Daniel Kim', 'Fiona Ma', 'George Chan', 'Amy Xu', 'Kelvin Lam', 'Joyce Yip', 'Brian Cooper']
const NOTE_TXT = ['业主自住，暂时不考虑出售，明年再回访。', '和太太聊过，房产事务由先生负责。', '对本街近期成交感兴趣，已发送 CMA。', '2023 年翻新厨房，两年内可能换大房。', '无人应门，已留名片。', '投资房，租约到明年 3 月。', '问圣诞后能否上门估价。', 'Spoke to owner, kids moving out — may downsize next year.']

/* ---------------- 房产 ---------------- */
const STREETS = [
  { name: 'Mains Rd', suburb: 'Sunnybank', a: [-27.5702, 153.0566], b: [-27.5902, 153.0579], n: 26 },
  { name: 'McCullough St', suburb: 'Sunnybank', a: [-27.5756, 153.0525], b: [-27.5763, 153.0690], n: 26 },
  { name: 'Gager St', suburb: 'Sunnybank', a: [-27.5811, 153.0597], b: [-27.5816, 153.0706], n: 18 },
  { name: 'Lister St', suburb: 'Sunnybank', a: [-27.5851, 153.0587], b: [-27.5857, 153.0700], n: 18 },
  { name: 'Turton St', suburb: 'Sunnybank', a: [-27.5768, 153.0627], b: [-27.5900, 153.0634], n: 20 },
  { name: 'Dillon St', suburb: 'Sunnybank', a: [-27.5716, 153.0642], b: [-27.5719, 153.0722], n: 12 },
  { name: 'Gowan Rd', suburb: 'Sunnybank Hills', a: [-27.5935, 153.0480], b: [-27.5945, 153.0640], n: 24 },
  { name: 'Hellawell Rd', suburb: 'Sunnybank Hills', a: [-27.5980, 153.0500], b: [-27.6060, 153.0520], n: 18 },
  { name: 'Pinelands Rd', suburb: 'Sunnybank Hills', a: [-27.5990, 153.0560], b: [-27.6000, 153.0680], n: 18 },
  { name: 'Warrigal Rd', suburb: 'Runcorn', a: [-27.5960, 153.0740], b: [-27.5990, 153.0880], n: 20 },
  { name: 'Gaynesford St', suburb: 'Runcorn', a: [-27.5880, 153.0760], b: [-27.5900, 153.0860], n: 14 },
  { name: 'Padstow Rd', suburb: 'Eight Mile Plains', a: [-27.5790, 153.0820], b: [-27.5850, 153.0960], n: 16 },
]
export const SUBURBS = [...new Set(STREETS.map(s => s.suburb))]
export const STREET_NAMES = STREETS.map(s => s.name)

let pid = 1
function mkProp(s, num, lat, lng, unit) {
  const x = rnd()
  const status = x < .3 ? 'green' : x < .86 ? 'red' : x < .93 ? 'grey' : 'other'
  const type = unit ? 'Unit' : rnd() < .14 ? 'Townhouse' : 'House'
  const beds = type === 'Unit' ? ri(1, 3) : ri(3, 5)
  const saleYear = ri(1993, 2025)
  const base = type === 'Unit' ? 520 : type === 'Townhouse' ? 760 : 1150
  const est = Math.round((base + rnd() * base * .5) / 10) * 10
  const p = {
    id: pid++, num: String(num), unit, street: s.name, suburb: s.suburb, lat, lng, status, type, beds,
    baths: Math.max(1, beds - ri(1, 2)), cars: ri(1, 2), land: type === 'House' ? ri(405, 810) : null,
    estLow: est, estHigh: est + ri(60, 140),
    rexId: status === 'grey' ? null : 40000 + pid * 13,
    lot: `Lot ${ri(1, 120)} on RP${ri(80000, 99999)}`,
    tp: status !== 'grey' && rnd() < .74 ? { owner: pick(NAMES), saleDate: new Date(saleYear, ri(0, 11), ri(1, 27)), price: Math.round(est * (0.35 + (saleYear - 1993) / 32 * .6)) * 1000 } : null,
    contacts: [], notes: [], lastVisit: null, lastResult: null, agent: null,
    flags: { dnc: false, dnk: rnd() < .035, junk: rnd() < .05 }, flagLog: [], tags: [],
  }
  p.addr = (unit ? unit + '/' : '') + p.num + ' ' + p.street
  if (status === 'green' || status === 'other') {
    p.agent = status === 'other' ? 'other' : pick(FIELD)
    p.contacts.push({ name: p.tp && rnd() < .7 ? p.tp.owner : pick(NAMES), role: rnd() < .8 ? '业主' : '房东', mobile: '04' + ri(10, 99) + ' ' + ri(100, 999) + ' ' + ri(100, 999), email: null, dnc: rnd() < .05 })
  }
  if (status === 'green' || (status === 'red' && rnd() < .45)) {
    for (let i = 0, k = ri(1, 3); i < k; i++) p.notes.push({ date: new Date(NOW0 - ri(20, 400) * DAY), author: pick(FIELD), text: pick(NOTE_TXT) })
    p.notes.sort((a, b) => b.date - a.date)
  }
  if (rnd() < .15) p.tags.push(pick(['潜在卖家', '投资客', '换房需求', '验房报告']))
  return p
}
const props = []
STREETS.forEach(s => {
  const dy = s.b[0] - s.a[0], dx = s.b[1] - s.a[1], L = Math.hypot(dx, dy), off = 0.00022
  for (let i = 0; i < s.n; i++) {
    const t = (i + 0.5) / s.n, side = i % 2
    const lat = s.a[0] + dy * t + (side ? -1 : 1) * (dx / L) * off
    const lng = s.a[1] + dx * t + (side ? 1 : -1) * (dy / L) * off
    props.push(mkProp(s, 2 * Math.floor(i / 2) + side + 1 + (i > 10 ? 2 : 0), lat, lng))
  }
})
// 15 Gager St 单元楼
for (let u = 1; u <= 6; u++) props.push(mkProp(STREETS[2], 15, -27.58085 + u * 0.00002, 153.0655, u))
const demo = props.find(p => p.street === 'Gager St' && !p.unit && p.status === 'red')
Object.assign(demo, { tp: { owner: 'Mary Chen', saleDate: new Date(2009, 4, 18), price: 412000 }, tags: ['潜在卖家'] })
demo.flags.dnk = false

/* ---------------- 活动流水（近 90 天） ---------------- */
const acts = []
let aid = 1
function pushAct(a) { a.id = aid++; acts.push(a); return a }
const knockable = props.filter(p => p.status !== 'grey')
for (let d = 89; d >= 0; d--) {
  const day = startOfDay(new Date(NOW0 - d * DAY)), wd = day.getDay(), weekend = wd === 0 || wd === 6
  FIELD.forEach(ag => {
    const lazy = ag === 'sarah' ? .4 : ag === 'owen' || ag === 'lily' ? .7 : 1
    const n = Math.round((weekend ? ri(4, 11) : ri(1, 6)) * lazy * (0.8 + (90 - d) / 90 * .4))
    for (let i = 0; i < n; i++) {
      const h = weekend ? ri(9, 16) : pick([10, 15, 16, 16, 17, 17, 18, 18, 19])
      const at = new Date(day.getTime() + h * 36e5 + ri(0, 59) * 6e4)
      if (at > NOW0) continue
      const r = rnd()
      const result = r < .4 ? '无人应门' : r < .58 ? '留名片' : r < .8 ? '与业主交谈' : r < .88 ? '租客应门' : r < .95 ? '不感兴趣' : '预约估价'
      const p = pick(knockable)
      const captured = TALK.has(result) && rnd() < .82
      pushAct({ type: 'knock', at, agent: ag, propId: p.id, result, captured, linked: captured && result !== '租客应门' && rnd() < .2 })
      if (result === '预约估价' && rnd() < .3 && d > 5) pushAct({ type: 'listing', at: new Date(at.getTime() + ri(3, 12) * DAY), agent: ag, propId: p.id, result: '签约委托' })
    }
    for (let i = 0, k = weekend ? 0 : ri(0, 4); i < k; i++) {
      const at = new Date(day.getTime() + ri(9, 18) * 36e5 + ri(0, 59) * 6e4)
      if (at <= NOW0) pushAct({ type: 'call', at, agent: ag, propId: pick(knockable).id, result: pick(['已接通', '未接通', '已接通', '约定回电']) })
    }
    if (wd === 6 && ag !== 'sarah') for (let i = 0, k = ri(4, 12); i < k; i++) {
      const at = new Date(day.getTime() + ri(9, 13) * 36e5 + ri(0, 59) * 6e4)
      if (at <= NOW0) pushAct({ type: 'ofi', at, agent: ag, propId: null, result: pick(['首次到访', '首次到访', '回头客']), captured: true })
    }
    if (rnd() < .06) pushAct({ type: 'photo', at: new Date(day.getTime() + 20 * 36e5), agent: ag, propId: null, result: `OCR ${ri(5, 14)} 行`, captured: true })
  })
}
acts.sort((a, b) => a.at - b.at)
acts.filter(a => a.at > NOW0).forEach(a => { a.at = new Date(NOW0 - ri(1, 5) * 36e5) })
acts.sort((a, b) => a.at - b.at)
acts.forEach(a => { if (a.type === 'knock') { const p = props[a.propId - 1]; if (p && (!p.lastVisit || a.at > p.lastVisit)) { p.lastVisit = a.at; p.lastResult = a.result; p.lastAgent = a.agent } } })

/* ---------------- 开放参观 ---------------- */
const LISTINGS = [
  { id: 'L1', addr: '12 Pinelands Rd', suburb: 'Sunnybank Hills', beds: 4, baths: 2, cars: 2, guide: '$1.28M – $1.36M', agent: 'jason', hue: 200 },
  { id: 'L2', addr: '7/88 Mains Rd', suburb: 'Sunnybank', beds: 3, baths: 2, cars: 1, guide: '$690k+', agent: 'kevin', hue: 30 },
  { id: 'L3', addr: '41 Gowan Rd', suburb: 'Sunnybank Hills', beds: 5, baths: 3, cars: 2, guide: '竞拍 · 11月22日', agent: 'owen', hue: 150 },
  { id: 'L4', addr: '23 Gaynesford St', suburb: 'Runcorn', beds: 4, baths: 2, cars: 2, guide: '$1.05M+', agent: 'natalie', hue: 280 },
  { id: 'L5', addr: '5 Dillon St', suburb: 'Sunnybank', beds: 3, baths: 1, cars: 1, guide: '$899k – $949k', agent: 'lily', hue: 0 },
]
const at = (d, h, m) => new Date(NOW0.getFullYear(), NOW0.getMonth(), NOW0.getDate() + d, h, m)
const VISITORS = NAMES.slice(4).map((n, i) => ({ name: n, mobile: '04' + (12 + i) + ' 555 0' + String(10 + i).padStart(2, '0'), email: n.split(' ')[0].toLowerCase() + '@mail.com', visits: i % 3 === 0 ? 2 : 1 }))
VISITORS[0].mobile = '0412 555 010'
let vid = 1
const mkAtt = (n, base) => Array.from({ length: n }, (_, i) => { const v = pick(VISITORS); return { id: vid++, ...v, at: new Date(base.getTime() + ri(0, 28) * 6e4), returning: v.visits > 1, report: rnd() < .35, seller: rnd() < .18, consent: rnd() < .7, follow: false } })
const opens = [
  { id: 'O1', listing: 'L1', start: at(-3, 10, 0), end: at(-3, 10, 30) },
  { id: 'O2', listing: 'L3', start: at(-3, 11, 0), end: at(-3, 11, 30) },
  { id: 'O3', listing: 'L4', start: at(-3, 12, 15), end: at(-3, 12, 45) },
  { id: 'O4', listing: 'L2', start: at(0, 14, 0), end: at(0, 14, 45) },
  { id: 'O5', listing: 'L5', start: at(2, 17, 30), end: at(2, 18, 0) },
  { id: 'O6', listing: 'L1', start: at(4, 10, 0), end: at(4, 10, 30) },
  { id: 'O7', listing: 'L3', start: at(4, 11, 0), end: at(4, 11, 30) },
  { id: 'O8', listing: 'L4', start: at(4, 12, 15), end: at(4, 12, 45) },
].map(o => ({ ...o, attendees: o.start < NOW0 ? mkAtt(o.id === 'O4' ? 5 : ri(8, 19), o.start) : [] }))

/* ---------------- 跟进任务 ---------------- */
const TASK_T = [['call', '回电讨论估价'], ['appraisal', '上门估价'], ['report', '发送验房报告'], ['call', '跟进出售意向'], ['visit', '再次上门拜访'], ['call', 'OFI 回访']]
const tasks = Array.from({ length: 28 }, (_, i) => {
  const p = pick(props.filter(x => x.contacts.length)), [type, title] = pick(TASK_T), dd = ri(-4, 9)
  return { id: i + 1, type, title, propId: p.id, contact: p.contacts[0].name, mobile: p.contacts[0].mobile, due: new Date(startOfDay(NOW0).getTime() + dd * DAY + ri(9, 18) * 36e5), agent: pick(FIELD), done: dd < -2 && rnd() < .5, source: pick(['语音录入', 'OFI 登记', '手动创建', '拍照录入']) }
})
tasks.push({ id: 99, type: 'appraisal', title: '圣诞后上门估价（业主主动提出）', propId: demo.id, contact: 'Mary Chen', mobile: '0423 118 902', due: new Date(NOW0.getTime() + 2 * 36e5), agent: 'kevin', done: false, source: '语音录入' })

/* ---------------- 审核队列 ---------------- */
const rv = []
props.filter(p => p.status === 'grey').forEach((p, i) => rv.push({ id: 'R' + p.id, kind: 'address', propId: p.id, rexAddr: (p.addr.replace(' St', ' Street').replace(' Rd', ' Road')) + (i % 2 ? '' : `, ${p.suburb} QLD`), cand: `${p.lot} · ${p.addr}`, score: 68 + (p.id * 7) % 30, reason: ['缺少单元号', '拼写差异', '街道类型不同', '门牌合并'][i % 4], at: new Date(NOW0 - ri(1, 72) * 36e5) }))
;[['业主姓名', 'Mary Chan', 'Mary Chen', '“…the owner is Mary 陈, 她说圣诞之后可以来估价…”', 62], ['手机号', '0423 118 902', '0423 118 920', '“…number is oh-four-two-three, one-one-eight, nine-zero-two…”', 71], ['关系类型', '租客', '业主', '“…佢话自己系租嘅，但系业主系佢阿哥…”', 54], ['跟进日期', '12月27日', '1月6日', '“…after the holidays, maybe early Jan…”', 58]].forEach(([f, v, alt, tr, c], i) => rv.push({ id: 'A' + i, kind: 'ai', propId: props[20 + i * 17].id, agent: FIELD[i % FIELD.length], field: f, value: v, alt, transcript: tr, conf: c, at: new Date(NOW0 - ri(1, 30) * 36e5) }))
;[['17A Turton St', '门口发现新门牌（原 17 号已分割）'], ['2/44 Lister St', 'Rex 无此单元']].forEach(([a, why], i) => rv.push({ id: 'N' + i, kind: 'new', addr: a, why, agent: pick(FIELD), at: new Date(NOW0 - ri(2, 40) * 36e5) }))
rv.push({ id: 'B1', kind: 'ocr', title: 'OFI 纸质登记表 · 41 Gowan Rd', agent: 'owen', at: new Date(NOW0 - 50 * 36e5), rows: [
  { name: 'Joyce Yip', mobile: '0433 210 778', email: 'joyce.yip@mail.com', interest: '买家', unsure: [] },
  { name: 'Kelvin Lam', mobile: '0411 9?? 302', email: '', interest: '买家', unsure: ['mobile'] },
  { name: 'B. Cooper', mobile: '0402 113 554', email: 'bcooper@outlook.com', interest: '潜在卖家', unsure: ['name'] },
  { name: 'Wendy Tse', mobile: '0450 772 019', email: 'wendy.t@gmail.com', interest: '买家', unsure: [] },
  { name: 'Ah Ming', mobile: '0416 380 4l2', email: '', interest: '验房报告', unsure: ['mobile', 'name'] },
] })
rv.push({ id: 'S1', kind: 'scope', addr: '9 Lister St', why: 'Lily 团队记录中存在同名业主 “Karen Lee”，Kevin 无权查看', agent: 'kevin', at: new Date(NOW0 - 5 * 36e5) })

/* ---------------- 全局状态 ---------------- */
export const state = reactive({
  me: 'ping', props, acts, opens, listings: LISTINGS, tasks, reviews: rv, visitors: VISITORS,
  audit: [], notifications: [], live: true, lastSync: now(), quota: 1180, cockpit: false,
  exports: [
    { at: new Date(NOW0 - 6 * DAY), who: 'Ping Han', scope: 'Sunnybank · 未关联 · 214 行', fields: 8, reason: '周末拜访规划' },
    { at: new Date(NOW0 - 13 * DAY), who: 'Ping Han', scope: 'Runcorn · 全部 · 96 行', fields: 6, reason: '团队会议' },
  ],
  settings: { ownerRoles: ['业主', '房东', '共同业主'], otherMode: 'neutral', retention: 30, cacheHours: 24, tags: ['潜在卖家', '投资客', '换房需求', '验房报告', '首次置业'], lists: ['Sunnybank 月报', 'OFI 回访', '新盘通知'], anomaly: 80, watermark: true, provider: 'Pricefinder' },
})
export const me = computed(() => userById(state.me))
export const propById = id => state.props.find(p => p.id === id)
export const listingById = id => state.listings.find(l => l.id === id)

// 当前角色下，房产对我的显示状态（封闭式数据库：别的团队的关联显示为紫色）
export function viewStatus(p) {
  if (p.status !== 'green' || me.value.role === 'admin') return p.status
  const owner = userById(p.agent)
  return owner && owner.team === me.value.team ? 'green' : 'other'
}
export const scopeAgents = computed(() => {
  const u = me.value
  if (u.role === 'admin') return FIELD
  if (u.role === 'leader') return USERS.filter(x => x.team === u.team && x.id !== 'ping').map(x => x.id)
  return [u.id]
})

/* ---------------- 审计 / 通知 ---------------- */
export function audit(action, target, detail, who) {
  state.audit.unshift({ id: Math.random().toString(36).slice(2), at: now(), who: who || me.value.name, action, target, detail })
}
export function notify(title, body, type = "info", link, p) {
  // title/body 存简体原文作为翻译 key，p 为插值参数，渲染时 t(body, p)
  state.notifications.unshift({ id: Math.random().toString(36).slice(2), at: now(), title, body, p, type, link, read: false })
}
;[...acts].slice(-40).forEach(a => { const p = a.propId && propById(a.propId); audit(ACT[a.type].label, p ? p.addr : '—', a.result + (a.captured ? ' · 已写入 Rex' : ''), userById(a.agent).name); state.audit[0].at = a.at })
audit('地址关联', '7 Mains Rd', 'Rex #48213 ↔ Lot 7 RP84512', 'Ping Han'); state.audit[0].at = new Date(NOW0 - 26 * 36e5)
audit('导出', 'Sunnybank · 未关联', '214 行 · 仅 Rex 字段', 'Ping Han'); state.audit[0].at = new Date(NOW0 - 6 * DAY)
state.audit.sort((a, b) => b.at - a.at)
notify('异常访问提醒', 'Jason Wu 过去 1 小时查看了 86 个房产详情（阈值 80）', 'warning', '/team')
notify("审核队列有新项目", "{n} 项待处理，其中 AI 不确定字段 4 项", "info", "/review", { n: rv.length })
notify("今天 14:00 开放参观", "{addr} · {who} 负责", "success", "/ofi", { addr: "7/88 Mains Rd", who: "Kevin Zhang" })

/* ---------------- 操作 ---------------- */
export function recordResult(p, result, extra = {}) {
  const prev = { lastVisit: p.lastVisit, lastResult: p.lastResult, lastAgent: p.lastAgent }
  const a = pushAct({ type: 'knock', at: now(), agent: state.me === 'ping' ? 'kevin' : state.me, propId: p.id, result, captured: !!extra.captured, linked: !!extra.linked })
  Object.assign(p, { lastVisit: a.at, lastResult: result, lastAgent: a.agent })
  audit('拜访结果', p.addr, result)
  return () => { state.acts.splice(state.acts.indexOf(a), 1); Object.assign(p, prev); audit('撤销', p.addr, '撤销拜访结果：' + result) }
}
export function addNote(p, text) {
  const n = { date: now(), author: state.me, text }
  p.notes.unshift(n); audit('添加笔记', p.addr, text.slice(0, 40))
  return () => { p.notes.splice(p.notes.indexOf(n), 1) }
}
export function setFlag(p, key, val, reason) {
  p.flags[key] = val
  p.flagLog.unshift({ at: now(), who: me.value.name, key, val, reason })
  audit(val ? '设置免打扰' : '取消免打扰', p.addr, `${FLAG_LABEL[key]}${reason ? ' · ' + reason : ''}`)
}
export const FLAG_LABEL = { dnc: '勿致电', dnk: '勿上门', junk: '勿投广告' }
export function linkOwner(p, c) {
  const prev = { status: p.status, agent: p.agent, contacts: [...p.contacts] }
  p.status = 'green'; p.agent = state.me === 'ping' ? 'kevin' : state.me
  p.contacts.unshift({ name: c.name, role: c.role || '业主', mobile: c.mobile, email: c.email, dnc: false })
  audit('关联业主', p.addr, `${c.name} · ${c.role || '业主'}`)
  return () => Object.assign(p, prev)
}
export function addTask(t) {
  const task = { id: Date.now(), done: false, agent: state.me === 'ping' ? 'kevin' : state.me, source: '手动创建', ...t }
  state.tasks.push(task); audit('创建跟进', propById(t.propId)?.addr || '—', t.title)
  return task
}
export function ofiRegister(o, v) {
  const a = { id: vid++, at: now(), follow: false, ...v }
  o.attendees.unshift(a)
  pushAct({ type: 'ofi', at: a.at, agent: listingById(o.listing).agent, propId: null, result: v.returning ? '回头客' : '首次到访', captured: true })
  audit('OFI 登记', listingById(o.listing).addr, `${v.name}${v.report ? ' · 验房报告' : ''}${v.seller ? ' · 潜在卖家' : ''}`)
  return a
}

/* ---------------- 实时模拟 ---------------- */
let timer
export function startLive() {
  clearInterval(timer)
  timer = setInterval(() => {
    state.lastSync = now()
    state.quota = Math.min(3600, state.quota + ri(3, 14))
    if (!state.live || rnd() > .55) return
    const ag = pick(FIELD.filter(id => userById(id).status === 'active'))
    const p = pick(knockable), r = rnd()
    const result = r < .45 ? '无人应门' : r < .62 ? '留名片' : r < .85 ? '与业主交谈' : r < .93 ? '租客应门' : '预约估价'
    const captured = TALK.has(result) && rnd() < .8
    const a = pushAct({ type: 'knock', at: now(), agent: ag, propId: p.id, result, captured, linked: captured && rnd() < .25, fresh: true })
    Object.assign(p, { lastVisit: a.at, lastResult: result, lastAgent: ag })
    if (a.linked && p.status === 'red') { p.status = 'green'; p.agent = ag; p.contacts.push({ name: p.tp?.owner || pick(NAMES), role: '业主', mobile: '04' + ri(10, 99) + ' ' + ri(100, 999) + ' ' + ri(100, 999), dnc: false }) }
    audit(captured ? '语音录入' : '拜访结果', p.addr, result + (captured ? ' · 已写入 Rex' : ''), userById(ag).name)
    if (result === '预约估价') notify("新的估价预约 🎉", "{who} 在 {addr} 约到了上门估价", "success", "/map?p=" + p.id, { who: userById(ag).name, addr: p.addr })
  }, 4000)
}

/* ---------------- 工具 ---------------- */
const intl = () => ({ 'zh-CN': 'zh-CN', 'zh-HK': 'zh-HK', en: 'en-AU' })[locale.value]
export const fmtDate = d => d ? d.toLocaleDateString(intl(), { year: 'numeric', month: 'short', day: 'numeric' }) : '—'
export const fmtMD = d => d ? (locale.value === 'en' ? d.toLocaleDateString('en-AU', { day: 'numeric', month: 'short' }) : `${d.getMonth() + 1}月${d.getDate()}日`) : '—'
export const fmtTime = d => d ? d.toTimeString().slice(0, 5) : '—'
const WEEK_ZH = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
// WEEK 为响应式代理：WEEK[i] 按当前语言返回
export const WEEK = new Proxy(WEEK_ZH, { get: (a, k) => (typeof k === 'string' && /^\d$/.test(k)) ? (locale.value === 'en' ? ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'][k] : t(a[k])) : a[k] })
export function relTime(d) {
  if (!d) return t('从未')
  const s = (now() - d) / 1000
  if (s < 0) { const m = -s / 60; return m < 60 ? t('{n} 分钟后', { n: Math.round(m) }) : m < 1440 ? t('{n} 小时后', { n: Math.round(m / 60) }) : t('{n} 天后', { n: Math.round(m / 1440) }) }
  if (s < 60) return t('刚刚')
  if (s < 3600) return t('{n} 分钟前', { n: Math.floor(s / 60) })
  if (s < 86400) return t('{n} 小时前', { n: Math.floor(s / 3600) })
  if (s < 86400 * 30) return t('{n} 天前', { n: Math.floor(s / 86400) })
  return fmtDate(d)
}
export const money = v => v >= 1e6 ? `$${(v / 1e6).toFixed(2)}M` : `$${Math.round(v / 1000)}k`
export const yearsHeld = p => p.tp ? Math.floor((NOW0 - p.tp.saleDate) / (365.25 * DAY)) : null
export { startOfDay, DAY }

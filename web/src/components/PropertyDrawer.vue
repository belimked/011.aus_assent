<template>
  <el-drawer :model-value="!!p" size="480px" :with-header="false" @close="ui.drawerId = null" class="pdrawer">
    <template v-if="p">
      <!-- 房源头图 -->
      <div class="hero" :style="{ background: heroBg }">
        <svg class="house" viewBox="0 0 200 120"><path :d="houseShape" fill="rgba(255,255,255,.92)" /><rect x="88" y="78" width="22" height="34" rx="2" :fill="accent" opacity=".85" /><rect x="52" y="70" width="22" height="18" rx="2" fill="#cfe3f5" /><rect x="126" y="70" width="22" height="18" rx="2" fill="#cfe3f5" /></svg>
        <div class="hero-top">
          <StatusPill :status="vs" />
          <el-tag v-if="p.flags.dnk" type="danger" effect="dark" size="small" round>{{ t('勿上门') }}</el-tag>
          <el-tag v-if="p.flags.dnc" type="danger" effect="dark" size="small" round>{{ t('勿致电') }}</el-tag>
          <el-tag v-for="tag in p.tags" :key="tag" effect="dark" size="small" round color="#c9a227" style="border:0;color:#1b1300">{{ t(tag) }}</el-tag>
          <div class="grow" />
          <el-button circle size="small" :icon="Close" @click="ui.drawerId = null" />
        </div>
        <div class="hero-bot">
          <div class="addr serif">{{ p.addr }}</div>
          <div class="sub">{{ p.suburb }} QLD 4109 · {{ typeLabel }} · {{ p.lot }}</div>
        </div>
      </div>

      <div class="body">
        <!-- 房屋规格 -->
        <div class="specs">
          <div><el-icon><House /></el-icon><b>{{ p.beds }}</b> {{ t('卧') }}</div>
          <div><el-icon><Watermelon /></el-icon><b>{{ p.baths }}</b> {{ t('卫') }}</div>
          <div><el-icon><Van /></el-icon><b>{{ p.cars }}</b> {{ t('车位') }}</div>
          <div v-if="p.land"><el-icon><Crop /></el-icon><b>{{ p.land }}</b> m²</div>
          <div class="grow" />
          <div class="est"><div class="small muted">{{ t('估价区间') }}</div><b>{{ money(p.estLow * 1000) }} – {{ money(p.estHigh * 1000) }}</b></div>
        </div>

        <!-- 快捷结果 -->
        <section>
          <div class="sh"><h4>{{ t('记录本次拜访') }}</h4><span class="small muted">{{ p.lastVisit ? t('上次：{result} · {time} · {agent}', { result: t(p.lastResult), time: relTime(p.lastVisit), agent: userById(p.lastAgent)?.name || '' }) : t('尚未拜访过') }}</span></div>
          <el-alert v-if="p.flags.dnk" type="error" :closable="false" show-icon :title="t('此房产标记为「勿上门」，请勿敲门')" style="margin-bottom:8px" />
          <div class="quick">
            <button v-for="r in RESULTS" :key="r" class="qb" :disabled="p.flags.dnk" @click="quick(r)">{{ t(r) }}</button>
          </div>
          <el-button class="gold-btn talk" size="large" :icon="Microphone" :disabled="p.flags.dnk" @click="openCapture(p.id)">{{ t('语音录入（中/英/粤混说）') }}</el-button>
        </section>

        <!-- Rex 联系人 -->
        <section>
          <div class="sh"><h4>{{ t('Rex 联系人') }}</h4><div class="grow" /><el-button link type="primary" size="small" :icon="TopRight" @click="ElMessage.info(t('演示：将在 Rex 中打开房产 #{id}', { id: p.rexId }))">{{ t('在 Rex 中打开') }}</el-button></div>
          <template v-if="vs === 'green'">
            <div v-for="c in p.contacts" :key="c.name" class="contact">
              <div class="avatar" style="width:38px;height:38px;background:#0f2a44;font-size:14px">{{ c.name.split(' ').map(s => s[0]).join('') }}</div>
              <div class="grow">
                <div><b>{{ c.name }}</b> <el-tag size="small" effect="plain">{{ t(c.role) }}</el-tag> <el-tag v-if="c.dnc" size="small" type="danger">{{ t('勿致电') }}</el-tag></div>
                <div class="small muted">{{ c.mobile }}<span v-if="c.email"> · {{ c.email }}</span></div>
              </div>
              <el-tooltip :content="c.dnc || p.flags.dnc ? t('该联系人设置了勿致电') : t('拨打 {mobile}', { mobile: c.mobile })"><el-button circle :icon="Phone" :disabled="c.dnc || p.flags.dnc" @click="call(c)" /></el-tooltip>
            </div>
            <div class="small muted" style="margin-top:4px">{{ t('跟进人：') }}<Avatar :id="p.agent" :size="18" /> {{ userById(p.agent)?.name }}</div>
          </template>
          <div v-else-if="vs === 'other'" class="empty-box"><el-icon><Lock /></el-icon> {{ t('此房产由其他团队跟进。按 Rex 封闭式数据库规则，联系人不可见。') }}<el-button link type="primary" @click="ElMessage.success(t('已向 Ping Han 发送协作请求'))">{{ t('申请协作') }}</el-button></div>
          <div v-else-if="vs === 'grey'" class="empty-box"><el-icon><WarningFilled /></el-icon> {{ t('地址尚未与 Rex 匹配，已进入管理员审核队列。') }}</div>
          <div v-else class="empty-box">{{ t('尚无已关联业主。通过语音录入或下方第三方信息一键关联。') }}</div>
        </section>

        <!-- 第三方 -->
        <section v-if="p.tp">
          <div class="tp">
            <div class="row"><el-icon color="#b8860b"><InfoFilled /></el-icon><b>{{ t('第三方业主信息') }}</b><div class="grow" /><span class="small muted">{{ t('来源 {provider} · 不导出', { provider: state.settings.provider }) }}</span></div>
            <div class="tpg">
              <div><div class="small muted">{{ t('业主姓名') }}</div><b>{{ p.tp.owner }}</b></div>
              <div><div class="small muted">{{ t('最近成交') }}</div><b>{{ fmtDate(p.tp.saleDate) }}</b></div>
              <div><div class="small muted">{{ t('持有') }}</div><b :style="{ color: yearsHeld(p) >= 10 ? '#b45309' : '' }">{{ t('持有 {n} 年', { n: yearsHeld(p) }) }}</b></div>
            </div>
            <div v-if="yearsHeld(p) >= 10" class="small hint">💡 {{ t('持有超过 10 年，出售可能性较高，建议优先拜访') }}</div>
            <el-button v-if="vs === 'red'" size="small" plain style="margin-top:8px" :icon="Link" @click="quickLink">{{ t('确认为业主并在 Rex 建档') }}</el-button>
          </div>
        </section>

        <!-- 笔记 -->
        <section>
          <div class="sh"><h4>{{ t('最近笔记') }}</h4><span class="small muted">{{ t('显示最近 3 条') }}</span></div>
          <el-timeline v-if="p.notes.length && vs !== 'other'" class="tl">
            <el-timeline-item v-for="(n, i) in p.notes.slice(0, 3)" :key="i" :timestamp="fmtDate(n.date) + ' · ' + (userById(n.author)?.name || n.author)" placement="top" :color="i === 0 ? '#c9a227' : ''">{{ tx(n.text) }}</el-timeline-item>
          </el-timeline>
          <div v-else class="small muted" style="margin-bottom:8px">{{ t('暂无笔记') }}</div>
          <div class="row">
            <el-input v-model="note" :placeholder="t('写一条笔记，回车保存…')" @keydown.enter="saveNote" />
            <el-button :disabled="!note.trim()" @click="saveNote">{{ t('保存') }}</el-button>
          </div>
        </section>

        <!-- 跟进 -->
        <section>
          <div class="sh"><h4>{{ t('跟进提醒') }}</h4></div>
          <div v-for="task in tasks" :key="task.id" class="task">
            <el-checkbox v-model="task.done" @change="v => v && ElMessage.success(t('已完成：{title}', { title: t(task.title) }))" />
            <div class="grow"><div :style="{ textDecoration: task.done ? 'line-through' : '' }">{{ t(task.title) }}</div><div class="small muted">{{ fmtMD(task.due) }} {{ fmtTime(task.due) }} · {{ userById(task.agent)?.name }}</div></div>
          </div>
          <div class="chips">
            <span class="small muted">{{ t('快速创建：') }}</span>
            <el-tag v-for="s in presets" :key="s.t" class="ct" effect="plain" @click="mkTask(s)">+ {{ t(s.t) }}</el-tag>
          </div>
        </section>

        <!-- 免打扰 -->
        <section>
          <div class="sh"><h4>{{ t('免打扰标记') }}</h4><span class="small muted">{{ t('同步到 Rex，所有人可见') }}</span></div>
          <div class="flags">
            <div v-for="(l, k) in FLAG_LABEL" :key="k" class="flag" :class="{ on: p.flags[k] }">
              <span>{{ t(l) }}</span>
              <el-switch :model-value="p.flags[k]" style="--el-switch-on-color:#dc2626" @change="v => toggleFlag(k, v)" />
            </div>
          </div>
          <div v-if="p.flagLog.length" class="small muted" style="margin-top:6px">{{ t('最近：') }}{{ p.flagLog[0].who }} {{ t(p.flagLog[0].val ? '设置' : '取消') }}「{{ t(FLAG_LABEL[p.flagLog[0].key]) }}」· {{ relTime(p.flagLog[0].at) }}</div>
        </section>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { computed, ref, h } from 'vue'
import { ElMessage, ElMessageBox, ElNotification, ElButton } from 'element-plus'
import { Close, Microphone, Phone, TopRight, Link } from '@element-plus/icons-vue'
import { state, me, propById, viewStatus, RESULTS, TALK, userById, relTime, fmtDate, fmtMD, fmtTime, yearsHeld, money, recordResult, addNote, setFlag, FLAG_LABEL, addTask, linkOwner, audit, now } from '../data/store'
import { ui, openCapture } from '../data/ui'
import { t, tx, locale } from '../i18n' 
import StatusPill from './StatusPill.vue'
import Avatar from './Avatar.vue'

const p = computed(() => ui.drawerId ? propById(ui.drawerId) : null)
const vs = computed(() => p.value && viewStatus(p.value))
const note = ref('')
const typeLabel = computed(() => t(({ House: '独立屋', Townhouse: '联排别墅', Unit: '公寓单元' })[p.value.type]))
const $t = t
const accent = computed(() => ({ green: '#16a34a', red: '#dc2626', grey: '#9ca3af', other: '#8b5cf6' })[vs.value])
const heroBg = computed(() => { const hue = (p.value.id * 47) % 360; return `linear-gradient(135deg, hsl(${hue} 35% 42%), hsl(${(hue + 40) % 360} 40% 26%))` })
const houseShape = computed(() => p.value.type === 'Unit' ? 'M40 112V40h120v72z M40 40h120v-8H40z' : p.value.type === 'Townhouse' ? 'M30 112V62l35-28 35 28 35-28 35 28v50z' : 'M40 112V62l60-44 60 44v50z')
const tasks = computed(() => state.tasks.filter(t => t.propId === p.value.id).sort((a, b) => a.due - b.due))
const presets = [{ t: '明天回电', d: 1, type: 'call' }, { t: '下周再访', d: 7, type: 'visit' }, { t: '一个月后估价', d: 30, type: 'appraisal' }]

function undoToast(msg, undo) {
  const n = ElNotification({
    title: msg, type: 'success', duration: 6000, position: 'bottom-right',
    message: h('div', { style: 'display:flex;align-items:center;gap:12px' }, [h('span', { class: 'small muted' }, t('已写入 Rex')), h(ElButton, { size: 'small', link: true, type: 'primary', onClick: () => { undo(); n.close(); ElMessage.info(t('已撤销')) } }, () => t('撤销'))]),
  })
}
function quick(r) {
  const undo = recordResult(p.value, r)
  undoToast(`${p.value.addr} · ${t(r)}`, undo)
  if (TALK.has(r)) setTimeout(() => ElMessageBox.confirm(t('和对方交谈过，要不要用语音把要点记下来？AI 会自动整理成字段。'), t('趁热记录'), { confirmButtonText: t('语音录入'), cancelButtonText: t('稍后'), type: 'info' }).then(() => openCapture(p.value.id)).catch(() => {}), 350)
}
function saveNote() { if (!note.value.trim()) return; const u = addNote(p.value, note.value.trim()); note.value = ''; undoToast(t('笔记已保存'), u) }
function call(c) { ElMessage.success(t('正在拨打 {name} {mobile}…（演示）', { name: c.name, mobile: c.mobile })); audit('电话回访', p.value.addr, c.name) }
async function toggleFlag(k, v) {
  try {
    const { value } = await ElMessageBox.prompt(v ? t('设置「{flag}」后，所有经纪人都会看到此标记。请注明原因：', { flag: t(FLAG_LABEL[k]) }) : t('取消「{flag}」需要记录原因（会写入审计日志）：', { flag: t(FLAG_LABEL[k]) }), t(v ? '设置免打扰' : '取消免打扰'),
      { inputPlaceholder: t(v ? '例如：业主当面要求' : '例如：业主书面同意恢复联系'), inputValidator: s => !!s?.trim() || t('请填写原因'), confirmButtonText: t('确认') })
    setFlag(p.value, k, v, value); ElMessage.success(t('已同步到 Rex'))
  } catch { }
}
function mkTask(s) {
  const t = addTask({ type: s.type, title: s.t.replace('明天', '').replace('下周', '').replace('一个月后', '') || s.t, propId: p.value.id, contact: p.value.contacts[0]?.name || p.value.tp?.owner || '业主', due: new Date(now().getTime() + s.d * 864e5) })
  t.title = s.t
  ElMessage.success($t('已创建跟进：{title}（{date}）', { title: $t(s.t), date: fmtMD(t.due) }))
}
async function quickLink() {
  await ElMessageBox.confirm(t('将在 Rex 中新建联系人「{name}」并关联为本房产的业主。确认前请当面核实身份。', { name: p.value.tp.owner }), t('关联业主'), { confirmButtonText: t('确认并写入 Rex') }).catch(() => { throw 0 })
  const undo = linkOwner(p.value, { name: p.value.tp.owner, role: '业主', mobile: '—' })
  undoToast(t('已关联业主，圆点变为绿色'), undo)
}
</script>

<style scoped>
.hero { height: 210px; position: relative; color: #fff; overflow: hidden; }
.house { position: absolute; right: -10px; bottom: -6px; width: 260px; opacity: .55; }
.hero-top { position: absolute; top: 14px; left: 18px; right: 14px; display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.hero-bot { position: absolute; left: 20px; bottom: 16px; right: 20px; text-shadow: 0 1px 8px rgba(0, 0, 0, .35); }
.addr { font-size: 28px; font-weight: 700; }
.sub { font-size: 13px; opacity: .9; margin-top: 2px; }
.body { padding: 0 20px 30px; }
.specs { display: flex; align-items: center; gap: 16px; padding: 14px 0; border-bottom: 1px solid var(--line); font-size: 13px; color: #475467; }
.specs > div { display: flex; align-items: center; gap: 4px; }
.specs b { color: var(--ink); font-size: 15px; }
.est { flex-direction: column; align-items: flex-end !important; gap: 0 !important; }
.est b { color: var(--navy); font-size: 16px; }
section { padding: 16px 0 4px; }
.sh { display: flex; align-items: baseline; gap: 10px; margin-bottom: 10px; }
.sh h4 { margin: 0; font-size: 14px; }
.quick { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.qb { min-width: 0; line-height: 1.25; overflow-wrap: anywhere; border: 1px solid var(--line); background: #fff; border-radius: 10px; padding: 9px 4px; font-size: 13px; cursor: pointer; transition: .15s; }
.qb:hover:not(:disabled) { border-color: var(--navy); background: #f2f5f9; }
.qb:active:not(:disabled) { transform: scale(.97); }
.qb:disabled { opacity: .4; cursor: not-allowed; }
.talk { width: 100%; margin-top: 10px; height: 46px; border-radius: 12px; font-size: 15px; }
.contact { display: flex; align-items: center; gap: 12px; padding: 10px 12px; border: 1px solid var(--line); border-radius: 12px; margin-bottom: 8px; }
.empty-box { background: #f7f8fa; border-radius: 10px; padding: 12px; font-size: 13px; color: var(--mut); display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }
.tp { background: var(--gold-soft); border: 1px solid #f0e2b1; border-radius: 12px; padding: 12px 14px; }
.tpg { display: grid; grid-template-columns: 1.4fr 1.2fr .8fr; gap: 8px; margin-top: 10px; }
.hint { margin-top: 8px; color: #92400e; }
.tl { padding-left: 2px; margin-bottom: 4px; }
.task { display: flex; gap: 10px; align-items: flex-start; padding: 6px 0; }
.chips { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; margin-top: 6px; }
.ct { cursor: pointer; }
.flags { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.flag { min-width: 0; gap: 4px; overflow-wrap: anywhere; display: flex; justify-content: space-between; align-items: center; border: 1px solid var(--line); border-radius: 10px; padding: 6px 10px; font-size: 13px; }
.flag.on { border-color: #fecaca; background: #fef2f2; color: #b91c1c; font-weight: 600; }
</style>
<style>.pdrawer .el-drawer__body { padding: 0; }</style>

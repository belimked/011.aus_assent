<template>
  <el-dialog :model-value="!!ui.capture" width="640px" :title="title" @close="close" top="6vh" class="capdlg" destroy-on-close>
    <template v-if="p">
      <div class="who row"><span class="dot" :style="{ background: STATUS[viewStatus(p)].color }" /><b>{{ p.addr }}</b><span class="muted small">{{ p.suburb }}</span></div>

      <!-- 步骤 1：录音 -->
      <div v-if="step === 'rec'" class="rec">
        <div class="small muted" style="margin-bottom:10px">{{ t('演示：选一段模拟口述，或者按住麦克风自己"说"（空格键也可以）') }}</div>
        <div class="samples">
          <div v-for="(s, i) in SAMPLES" :key="i" class="sample" :class="{ on: pickIdx === i }" @click="pickIdx = i">
            <div class="row"><el-tag size="small" effect="plain">{{ t(s.lang) }}</el-tag><b class="small">{{ t(s.t) }}</b></div>
            <div class="small muted q">“{{ s.text }}”</div>
          </div>
        </div>
        <div class="mic-wrap">
          <button class="mic" :class="{ on: holding }" @mousedown="hold" @mouseup="release" @mouseleave="holding && release()" @touchstart.prevent="hold" @touchend.prevent="release">
            <el-icon :size="34"><Microphone /></el-icon>
          </button>
          <div class="wave" v-if="holding"><i v-for="n in 14" :key="n" :style="{ animationDelay: n * 60 + 'ms' }" /></div>
          <div class="small muted">{{ holding ? t('正在录音 {time} · 松开结束', { time: `0:${String(sec).padStart(2, '0')}` }) : t('按住说话') }}</div>
          <div v-if="holding" class="live-tx">{{ liveText }}</div>
        </div>
      </div>

      <!-- 步骤 2：处理中 -->
      <div v-else-if="step === 'proc'" class="proc">
        <el-steps :active="procStep" finish-status="success" align-center>
          <el-step :title="t('上传录音')" :description="t('悉尼区域')" />
          <el-step :title="t('转写')" :description="t('中/英/粤')" />
          <el-step :title="t('提取字段')" :description="t('Claude · Sydney')" />
          <el-step :title="t('去重匹配')" :description="t('Rex 联系人')" />
        </el-steps>
        <div class="tx">{{ sample.text }}</div>
      </div>

      <!-- 步骤 3：确认卡片 -->
      <div v-else-if="step === 'card'" class="card-s">
        <el-alert v-if="unsure.length" type="warning" :closable="false" show-icon :title="t('有 {n} 个字段 AI 不太确定（黄色），请核对后再写入', { n: unsure.length })" />
        <div v-if="sample.dup" class="dup">
          <el-icon color="#2563eb"><User /></el-icon>
          <div class="grow small">{{ t('Rex 中已存在相似联系人') }} <b>{{ sample.dup }}</b>{{ t('（{mobile}），将合并而不是新建', { mobile: sample.f.mobile }) }}</div>
          <el-switch v-model="merge" :active-text="t('合并')" />
        </div>
        <el-form label-width="96px" class="form" size="default">
          <el-form-item v-for="f in FIELDS" :key="f.k" :label="t(f.l)" :class="{ unsure: unsure.includes(f.k) }">
            <el-select v-if="f.opts" v-model="form[f.k]" style="width:100%"><el-option v-for="o in f.opts" :key="o" :label="t(o)" :value="o" /></el-select>
            <el-date-picker v-else-if="f.k === 'follow'" v-model="form.follow" type="datetime" style="width:100%" format="YYYY-MM-DD HH:mm" :placeholder="t('无需跟进')" />
            <el-input v-else v-model="form[f.k]" :type="f.k === 'note' ? 'textarea' : 'text'" :rows="3" />
            <el-button v-if="unsure.includes(f.k)" link type="warning" size="small" @click="unsure.splice(unsure.indexOf(f.k), 1)">{{ t('没问题') }}</el-button>
          </el-form-item>
          <el-form-item :label="t('标签')">
            <el-check-tag v-for="t in state.settings.tags" :key="t" :checked="form.tags.includes(t)" class="tg" @change="c => c ? form.tags.push(t) : form.tags.splice(form.tags.indexOf(t), 1)">{{ $translate(t) }}</el-check-tag>
          </el-form-item>
          <el-form-item :label="t('邮件列表')">
            <el-checkbox v-model="form.consent">{{ t('对方口头同意接收 “Sunnybank 月报”（记录同意时间与录音）') }}</el-checkbox>
          </el-form-item>
          <el-form-item v-if="sample.flag" :label="t('免打扰')">
            <el-tag type="danger" effect="dark">{{ t('勿上门') }}</el-tag><span class="small muted" style="margin-left:8px">{{ t('从口述中识别到“不要再来”') }}</span>
          </el-form-item>
        </el-form>
        <div class="ops">
          <div class="small muted" style="margin-bottom:6px">{{ t('将对 Rex 执行以下操作：') }}</div>
          <div v-for="o in ops" :key="o" class="op"><el-icon color="#16a34a"><CircleCheck /></el-icon>{{ o }}</div>
        </div>
      </div>
    </template>
    <template #footer>
      <div v-if="step === 'rec'" class="row" style="justify-content:flex-end"><el-button @click="close">{{ t('取消') }}</el-button><el-button type="primary" @click="process">{{ t('使用所选口述') }}</el-button></div>
      <div v-else-if="step === 'card'" class="row">
        <el-button link :icon="RefreshLeft" @click="step = 'rec'">{{ t('重新录') }}</el-button>
        <div class="grow" />
        <el-button @click="close">{{ t('放弃') }}</el-button>
        <el-button class="gold-btn" :disabled="unsure.length > 0" @click="commit">{{ unsure.length ? t('还有 {n} 项待核对', { n: unsure.length }) : t('确认写入 Rex') }}</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElNotification } from 'element-plus'
import { Microphone, RefreshLeft } from '@element-plus/icons-vue'
import { state, STATUS, viewStatus, propById, recordResult, addNote, setFlag, addTask, linkOwner, audit, now, fmtMD } from '../data/store'
import { ui } from '../data/ui'
import { t, tx, locale } from '../i18n'
import enCapture from '../i18n/en/capture' 

const SAMPLES = [
  { lang: '中英混说', t: '业主在家，有出售意向', text: '业主是 Mary Chen，就是 owner 本人，她说孩子们都搬出去了，maybe downsize next year。手机 0423 118 902。圣诞之后可以来做个 appraisal。', f: { name: 'Mary Chen', role: '业主', mobile: '0423 118 902', result: '预约估价', note: '业主自住，子女已搬出，考虑明年换小房。同意圣诞后上门估价。', follow: 28 }, unsure: ['mobile'], tags: ['潜在卖家', '换房需求'], consent: true },
  { lang: '粤语 + 英文', t: '要求勿上门', text: '佢话唔好再嚟敲门喇，有咩事 email 佢就得，Peter Nguyen，peter.n@mail.com。', f: { name: 'Peter Nguyen', role: '业主', mobile: '', email: 'peter.n@mail.com', result: '与业主交谈', note: '业主要求不要再上门，可通过电邮联系。', follow: null }, unsure: ['name'], tags: [], flag: true },
  { lang: 'English', t: '租客应门', text: "Tenant answered, lease ends in March. Landlord is David Liu, she gave me his number 0412 555 010.", f: { name: 'David Liu', role: '房东', mobile: '0412 555 010', result: '租客应门', note: '租客应门，租约 3 月到期。房东 David Liu。', follow: 7 }, unsure: [], tags: ['投资客'], dup: 'David Liu' },
]
const FIELDS = [
  { k: 'name', l: '联系人' }, { k: 'role', l: '关系', opts: ['业主', '房东', '共同业主', '租客', '亲属'] }, { k: 'mobile', l: '手机' }, { k: 'email', l: '电邮' },
  { k: 'result', l: '拜访结果', opts: ['与业主交谈', '租客应门', '预约估价', '不感兴趣', '留名片'] }, { k: 'note', l: '笔记' }, { k: 'follow', l: '跟进提醒' },
]
const step = ref('rec'), pickIdx = ref(0), holding = ref(false), sec = ref(0), procStep = ref(0), merge = ref(true)
const form = reactive({}), unsure = ref([])
const p = computed(() => ui.capture && propById(ui.capture.propId))
const sample = computed(() => SAMPLES[pickIdx.value])
const title = computed(() => t(({ rec: '语音录入', proc: 'AI 正在整理…', card: '确认卡片' })[step.value]))
const $translate = t
const liveText = computed(() => sample.value.text.slice(0, Math.min(sample.value.text.length, sec.value * 12)) + '…')
let t1
watch(() => ui.capture, v => { if (v) { step.value = 'rec'; pickIdx.value = 0 } })
watch(locale, (lang, previous) => {
  if (step.value !== 'card') return
  const original = sample.value.f.note
  // Preserve notes edited by the agent; only translate untouched sample extraction.
  if (form.note === original || (previous === 'en' && form.note === enCapture[original]))
    form.note = t(original)
})

function hold() { holding.value = true; sec.value = 0; t1 = setInterval(() => sec.value++, 1000) }
function release() { clearInterval(t1); holding.value = false; if (sec.value < 1) { ElMessage.warning(t('按住时间太短，请按住说话')); return } process() }
function process() {
  step.value = 'proc'; procStep.value = 0
  const iv = setInterval(() => {
    procStep.value++
    if (procStep.value >= 4) {
      clearInterval(iv)
      const f = { ...sample.value.f }
      // 第一段口述适配当前房产的第三方业主名，演示更真实
      if (pickIdx.value === 0 && p.value.tp) f.name = p.value.tp.owner
      Object.assign(form, { name: f.name, role: f.role, mobile: f.mobile, email: f.email || '', result: f.result, note: t(f.note), follow: f.follow ? new Date(now().getTime() + f.follow * 864e5) : null, tags: [...sample.value.tags], consent: !!sample.value.consent })
      unsure.value = [...sample.value.unsure]; merge.value = true
      setTimeout(() => step.value = 'card', 250)
    }
  }, 550)
}
const ops = computed(() => {
  const o = []
  o.push(sample.value.dup && merge.value ? t('更新联系人 {name}（合并）', { name: form.name }) : t('新建联系人 {name}', { name: form.name }))
  if (['业主', '房东', '共同业主'].includes(form.role)) o.push(t('关联为本房产「{role}」→ 圆点变绿', { role: t(form.role) }))
  o.push(t('写入拜访结果「{result}」及笔记', { result: t(form.result) }))
  if (form.follow) o.push(t('创建跟进提醒 {date}', { date: fmtMD(form.follow) }))
  if (form.tags.length) o.push(t('添加标签 {tags}', { tags: form.tags.map(t).join(locale.value === 'en' ? ', ' : '、') }))
  if (form.consent) o.push(t('记录邮件列表同意'))
  if (sample.value.flag) o.push(t('设置「勿上门」'))
  return o
})
function commit() {
  const prop = p.value
  recordResult(prop, form.result, { captured: true, linked: true })
  addNote(prop, form.note)
  if (['业主', '房东', '共同业主'].includes(form.role) && prop.status !== 'green') linkOwner(prop, { name: form.name, role: form.role, mobile: form.mobile || '—', email: form.email })
  if (form.follow) addTask({ type: 'call', title: form.result === '预约估价' ? '上门估价' : '跟进回访', propId: prop.id, contact: form.name, mobile: form.mobile, due: form.follow, source: '语音录入' })
  form.tags.forEach(t => !prop.tags.includes(t) && prop.tags.push(t))
  if (sample.value.flag) setFlag(prop, 'dnk', true, '语音录入：业主要求')
  audit('语音录入', prop.addr, ops.value.join('；'))
  ElNotification({ title: t('已写入 Rex'), message: t('{addr} · {n} 项操作完成', { addr: prop.addr, n: ops.value.length }), type: 'success', position: 'bottom-right' })
  close()
}
function close() { clearInterval(t1); holding.value = false; ui.capture = null }
const key = e => { if (!ui.capture || step.value !== 'rec' || e.code !== 'Space' || e.target.tagName === 'INPUT') return; e.preventDefault(); if (e.type === 'keydown' && !holding.value && !e.repeat) hold(); if (e.type === 'keyup' && holding.value) release() }
onMounted(() => { window.addEventListener('keydown', key); window.addEventListener('keyup', key) })
onBeforeUnmount(() => { window.removeEventListener('keydown', key); window.removeEventListener('keyup', key) })
</script>

<style scoped>
.who { padding: 8px 12px; background: #f6f7f9; border-radius: 10px; margin: -6px 0 14px; }
.samples { display: grid; gap: 8px; }
.sample { border: 1.5px solid var(--line); border-radius: 12px; padding: 10px 12px; cursor: pointer; }
.sample.on { border-color: var(--gold); background: var(--gold-soft); }
.q { margin-top: 4px; line-height: 1.5; }
.mic-wrap { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 24px 0 6px; }
.mic { width: 92px; height: 92px; border-radius: 50%; border: 0; background: linear-gradient(135deg, #0f2a44, #1f4b73); color: #fff; box-shadow: 0 10px 30px rgba(15, 42, 68, .35); transition: .15s; cursor: pointer; }
.mic.on { background: linear-gradient(135deg, #c9a227, #e6c35c); color: #1b1300; transform: scale(1.1); animation: pulse 1.2s infinite; }
.wave { display: flex; gap: 3px; height: 28px; align-items: center; }
.wave i { width: 4px; height: 8px; background: var(--gold); border-radius: 3px; animation: wv .7s ease-in-out infinite alternate; }
@keyframes wv { to { height: 26px } }
.live-tx { font-size: 13px; color: #475467; max-width: 460px; text-align: center; }
.proc { padding: 18px 0 10px; }
.tx { margin-top: 24px; padding: 14px; background: #f7f8fa; border-radius: 12px; font-size: 14px; line-height: 1.7; color: #344054; }
.dup { display: flex; align-items: center; gap: 10px; margin: 10px 0 0; padding: 10px 12px; background: #eff6ff; border-radius: 10px; }
.form { margin-top: 14px; }
.form :deep(.unsure .el-input__wrapper), .form :deep(.unsure .el-select__wrapper), .form :deep(.unsure .el-textarea__inner) { background: #fef9c3; box-shadow: 0 0 0 1px #facc15 inset; }
.form :deep(.el-form-item) { margin-bottom: 12px; }
.form :deep(.el-form-item__content) { flex-wrap: nowrap; gap: 6px; }
.tg { margin-right: 6px; }
.form :deep(.el-form-item__content:has(.tg)) { flex-wrap: wrap; }
.ops { background: #f0fdf4; border-radius: 12px; padding: 10px 14px; }
.op { display: flex; gap: 6px; align-items: center; font-size: 13px; padding: 2px 0; }
</style>
<style>.capdlg { border-radius: 16px; }</style>

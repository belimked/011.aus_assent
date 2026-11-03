<template>
  <div class="page">
    <div class="page-head">
      <div><h1>{{ t('开放参观') }}</h1><div class="sub">{{ t('OFI 访客登记 · 回头客自动识别 · 验房报告与潜在卖家跟进') }}</div></div>
      <div class="sp" />
      <el-segmented v-model="when" :options="[{ label: t('今天'), value: 'today' }, { label: t('即将'), value: 'up' }, { label: t('已结束'), value: 'past' }, { label: t('全部'), value: 'all' }]" />
    </div>
    <div class="ofi">
      <div class="olist">
        <div v-for="o in opens" :key="o.id" class="oc" :class="{ on: cur?.id === o.id }" @click="curId = o.id">
          <div class="oimg" :style="{ background: `linear-gradient(135deg, hsl(${L(o).hue} 40% 45%), hsl(${L(o).hue + 40} 45% 25%))` }">
            <el-tag size="small" :type="stTag(o).t" effect="dark" round>{{ t(stTag(o).l) }}</el-tag>
          </div>
          <div class="grow">
            <div class="serif oa">{{ L(o).addr }}</div>
            <div class="small muted">{{ t(WEEK[o.start.getDay()]) }} {{ fmtMD(o.start) }} · {{ fmtTime(o.start) }}–{{ fmtTime(o.end) }}</div>
            <div class="small row" style="margin-top:4px"><Avatar :id="L(o).agent" :size="18" /> {{ userById(L(o).agent).name }} <span class="grow" /> <b>{{ o.attendees.length }}</b> {{ t('人') }}</div>
          </div>
        </div>
        <el-empty v-if="!opens.length" :description="t('没有开放参观')" :image-size="60" />
      </div>

      <div v-if="cur" class="detail">
        <div class="card hero" :style="{ background: `linear-gradient(120deg, hsl(${L(cur).hue} 40% 30%), #0f2a44)` }">
          <div class="grow">
            <div class="row"><el-tag size="small" :type="stTag(cur).t" effect="dark" round>{{ t(stTag(cur).l) }}</el-tag><span class="small" style="opacity:.8">{{ L(cur).suburb }} QLD</span></div>
            <div class="serif ha">{{ L(cur).addr }}</div>
            <div class="row hs"><span>{{ L(cur).beds }} {{ t('卧') }}</span><span>{{ L(cur).baths }} {{ t('卫') }}</span><span>{{ L(cur).cars }} {{ t('车位') }}</span><span class="gold">{{ tx(L(cur).guide) }}</span></div>
          </div>
          <div class="hstats">
            <div><b class="serif">{{ cur.attendees.length }}</b><span>{{ t('访客') }}</span></div>
            <div><b class="serif">{{ cur.attendees.filter(a => a.returning).length }}</b><span>{{ t('回头客') }}</span></div>
            <div><b class="serif">{{ cur.attendees.filter(a => a.report).length }}</b><span>{{ t('要验房报告') }}</span></div>
            <div><b class="serif gold">{{ cur.attendees.filter(a => a.seller).length }}</b><span>{{ t('潜在卖家') }}</span></div>
          </div>
        </div>

        <div class="cols">
          <!-- 登记表单 -->
          <div class="card">
            <div class="card-h"><h3>{{ t('访客登记') }}</h3><div class="sp" /><el-button size="small" :icon="Microphone" @click="voiceFill">{{ t('一句话填表') }}</el-button><el-button size="small" :icon="Camera" @click="ElMessage.info(t('拍照录入纸质登记表：识别结果会进入审核队列 → 审核队列 / OCR'))">{{ t('拍纸质表') }}</el-button></div>
            <div class="card-b">
              <el-form ref="fm" :model="f" :rules="rules" label-position="top" @submit.prevent>
                <el-form-item :label="t('手机号')" prop="mobile">
                  <el-input v-model="f.mobile" placeholder="04xx xxx xxx" @input="lookup" maxlength="14"><template #prefix>+61</template></el-input>
                </el-form-item>
                <transition name="el-zoom-in-top">
                  <div v-if="hit" class="hit"><el-icon color="#16a34a"><SuccessFilled /></el-icon><div class="grow small"><b>{{ t('回头客：{name}', { name: hit.name }) }}</b><br>{{ t('之前看过 {n} 次开放参观，信息已自动填好', { n: hit.visits }) }}</div></div>
                </transition>
                <div class="two">
                  <el-form-item :label="t('姓名')" prop="name"><el-input v-model="f.name" /></el-form-item>
                  <el-form-item :label="t('电邮')" prop="email"><el-input v-model="f.email" /></el-form-item>
                </div>
                <el-form-item :label="t('来访目的')">
                  <el-radio-group v-model="f.intent"><el-radio-button value="buy">{{ t('自住购房') }}</el-radio-button><el-radio-button value="invest">{{ t('投资') }}</el-radio-button><el-radio-button value="look">{{ t('随便看看') }}</el-radio-button></el-radio-group>
                </el-form-item>
                <div class="checks">
                  <el-checkbox v-model="f.report" border>{{ t('需要验房报告（自动发送）') }}</el-checkbox>
                  <el-checkbox v-model="f.seller" border>{{ t('附近有房想卖（潜在卖家）') }}</el-checkbox>
                  <el-checkbox v-model="f.consent" border>{{ t('同意接收新盘通知') }}</el-checkbox>
                </div>
                <div class="privacy small muted"><el-icon><InfoFilled /></el-icon> {{ t('已向访客出示隐私告知：信息仅用于本次看房跟进，可随时要求删除。') }}</div>
                <el-button class="gold-btn" size="large" style="width:100%;margin-top:12px" @click="submit">{{ t('登记访客') }} <span class="kbd" style="margin-left:8px">Enter</span></el-button>
              </el-form>
            </div>
          </div>

          <!-- 访客名单 -->
          <div class="card">
            <div class="card-h"><h3>{{ t('访客名单') }}</h3><div class="sp" />
              <el-select v-model="vf" size="small" style="width:120px" :placeholder="t('全部')"><el-option :label="t('全部')" value="" /><el-option :label="t('回头客')" value="returning" /><el-option :label="t('要报告')" value="report" /><el-option :label="t('潜在卖家')" value="seller" /><el-option :label="t('未跟进')" value="todo" /></el-select>
            </div>
            <div class="card-b vl">
              <TransitionGroup name="fade-up">
                <div v-for="a in visitors" :key="a.id" class="vi">
                  <div class="avatar" style="width:34px;height:34px;background:#334155;font-size:12px">{{ a.name.split(' ').map(s => s[0]).join('') }}</div>
                  <div class="grow">
                    <div class="row"><b>{{ a.name }}</b><el-tag v-if="a.returning" size="small" type="success" round>{{ t('回头客') }}</el-tag><el-tag v-if="a.seller" size="small" effect="dark" color="#c9a227" style="border:0;color:#1b1300" round>{{ t('潜在卖家') }}</el-tag><el-tag v-if="a.report" size="small" type="info" round>{{ t('报告') }}</el-tag></div>
                    <div class="small muted">{{ a.mobile }} · {{ fmtTime(a.at) }}</div>
                  </div>
                  <el-button v-if="!a.follow" size="small" @click="follow(a)">{{ t('跟进') }}</el-button>
                  <el-tag v-else size="small" type="success">{{ t('已建跟进') }}</el-tag>
                </div>
              </TransitionGroup>
              <el-empty v-if="!visitors.length" :description="t(cur.start > now() ? '开放参观尚未开始' : '暂无访客')" :image-size="60" />
            </div>
            <div class="vf" v-if="cur.attendees.length">
              <el-button size="small" :icon="Promotion" @click="sendReports">{{ t('发送验房报告（{n}）', { n: cur.attendees.filter(a => a.report && !a.sent).length }) }}</el-button>
              <el-button size="small" :icon="ChatLineSquare" @click="ElMessage.success(t('已生成卖方周报草稿：{n} 组访客', { n: cur.attendees.length }))">{{ t('生成卖方报告') }}</el-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Microphone, Camera, Promotion, ChatLineSquare } from '@element-plus/icons-vue'
import { state, listingById, userById, fmtTime, fmtMD, WEEK, now, startOfDay, ofiRegister, addTask, DAY, audit } from '../data/store'
import Avatar from '../components/Avatar.vue'
import { t, tx, locale } from '../i18n'

const route = useRoute()
const when = ref('today'), curId = ref(route.query.o || null), vf = ref(''), hit = ref(null), fm = ref()
const L = o => listingById(o.listing)
const sod = () => startOfDay(now()).getTime()
const opens = computed(() => state.opens.filter(o => when.value === 'all' || (when.value === 'today' ? startOfDay(o.start).getTime() === sod() : when.value === 'up' ? o.start > now() : o.end < now())).sort((a, b) => when.value === 'past' ? b.start - a.start : a.start - b.start))
watch(opens, v => { if (!v.find(o => o.id === curId.value)) curId.value = v[0]?.id }, { immediate: true })
const cur = computed(() => state.opens.find(o => o.id === curId.value))
const stTag = o => now() < o.start ? { l: '未开始', t: 'info' } : now() < o.end ? { l: '● 进行中', t: 'success' } : { l: '已结束', t: '' }
const blank = () => ({ mobile: '', name: '', email: '', intent: 'buy', report: false, seller: false, consent: true })
const f = reactive(blank())
const rules = computed(() => ({ mobile: [{ required: true, message: t('请填写手机号') }, { pattern: /^04\d{2}\s?\d{3}\s?\d{3}$/, message: t('澳洲手机号格式：04xx xxx xxx') }], name: [{ required: true, message: t('请填写姓名') }], email: [{ type: 'email', message: t('电邮格式不对') }] }))
function lookup() {
  const d = f.mobile.replace(/\s/g, '')
  if (d.length === 10) f.mobile = d.replace(/(\d{4})(\d{3})(\d{3})/, '$1 $2 $3')
  const v = state.visitors.find(v => v.mobile.replace(/\s/g, '') === d && v.visits > 1)
  hit.value = v || null
  if (v) Object.assign(f, { name: v.name, email: v.email })
}
async function submit() {
  if (cur.value.start > new Date(now().getTime() + 30 * 6e4)) return ElMessage.warning(t('这场开放参观还没开始'))
  await fm.value.validate().catch(() => { throw 0 })
  ofiRegister(cur.value, { name: f.name, mobile: f.mobile, email: f.email, returning: !!hit.value, report: f.report, seller: f.seller, consent: f.consent })
  ElMessage.success(t('已登记 {name}', { name: f.name }) + (f.report ? t('，验房报告将自动发送') : ''))
  if (f.seller) ElMessage({ type: 'warning', message: t('潜在卖家已推送给 {name}', { name: userById(L(cur.value).agent).name }), duration: 4000 })
  Object.assign(f, blank()); hit.value = null; fm.value.clearValidate()
}
function voiceFill() {
  Object.assign(f, { mobile: '0412 555 010', report: true, seller: false, intent: 'buy' }); lookup()
  ElMessage.success(t('语音识别：“0412 555 010，要份验房报告” → 已填表，并识别为回头客'))
}
const visitors = computed(() => cur.value.attendees.filter(a => !vf.value || (vf.value === 'todo' ? !a.follow : a[vf.value])))
function follow(a) { a.follow = true; addTask({ type: 'call', title: 'OFI 回访', propId: state.props[0].id, contact: a.name, mobile: a.mobile, due: new Date(now().getTime() + DAY), source: 'OFI 登记' }); ElMessage.success(t('已为 {name} 创建明天的回访', { name: a.name })) }
function sendReports() { const L2 = cur.value.attendees.filter(a => a.report && !a.sent); if (!L2.length) return ElMessage.info(t('没有待发送的报告')); L2.forEach(a => a.sent = true); audit('发送验房报告', L(cur.value).addr, L2.length + ' 份'); ElMessage.success(t('已发送 {n} 份验房报告', { n: L2.length })) }
const key = e => { if (e.key === 'Enter' && e.target.tagName === 'INPUT' && fm.value?.$el.contains(e.target)) submit().catch(() => {}) }
onMounted(() => window.addEventListener('keydown', key))
onBeforeUnmount(() => window.removeEventListener('keydown', key))
</script>

<style scoped>
.ofi { display: grid; grid-template-columns: 320px 1fr; gap: 16px; align-items: start; }
.olist { display: flex; flex-direction: column; gap: 10px; }
.oc { display: flex; gap: 12px; background: #fff; border: 1.5px solid var(--line); border-radius: 14px; padding: 10px; cursor: pointer; transition: .15s; }
.oc:hover { border-color: #c8d5e2; }
.oc.on { border-color: var(--gold); box-shadow: 0 4px 16px rgba(201, 162, 39, .18); }
.oimg { width: 78px; height: 78px; border-radius: 10px; flex: none; padding: 6px; }
.oa { font-size: 16px; font-weight: 700; }
.hero { color: #fff; display: flex; gap: 20px; padding: 20px 24px; border: 0; margin-bottom: 16px; align-items: center; }
.ha { font-size: 30px; font-weight: 700; margin: 6px 0; }
.hs { gap: 14px; font-size: 14px; opacity: .92; }
.gold { color: #e6c35c; }
.hstats { display: grid; grid-template-columns: repeat(4, minmax(66px, 1fr)); gap: 8px; text-align: center; }
.hstats b { display: block; font-size: 30px; }
.hstats span { display: block; font-size: 12px; line-height: 1.25; opacity: .75; overflow-wrap: anywhere; }
.cols { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.two { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.hit { display: flex; gap: 10px; align-items: flex-start; background: #f0fdf4; border: 1px solid #bbf7d0; border-radius: 10px; padding: 10px 12px; margin: -6px 0 12px; }
.checks { display: flex; flex-direction: column; gap: 8px; }
.checks .el-checkbox { margin: 0; width: 100%; }
.privacy { display: flex; gap: 6px; align-items: center; margin-top: 10px; }
.vl { max-height: 480px; overflow: auto; }
.vi { display: flex; align-items: center; gap: 10px; padding: 9px 4px; border-bottom: 1px solid #f3f4f6; }
.vf { display: flex; gap: 8px; padding: 0 18px 16px; }
</style>

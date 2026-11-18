import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import * as Icons from '@element-plus/icons-vue'
import 'leaflet/dist/leaflet.css'
import './styles.css'
import App from './App.vue'
import router from './router'
import { setLocale } from './i18n'

const app = createApp(App)
for (const [k, c] of Object.entries(Icons)) app.component(k, c)
app.use(ElementPlus, { locale: zhCn }).use(router)
// 先恢复上次的语言（繁体需要懒加载 OpenCC），再挂载，避免闪烁
const saved = localStorage.getItem('lang') || 'zh-CN'
setLocale(saved).catch(() => {}).finally(() => app.mount('#app'))

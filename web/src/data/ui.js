import { reactive } from 'vue'

// 跨页面共享的界面状态：房产抽屉、语音录入、全局搜索
export const ui = reactive({
  drawerId: null,
  capture: null, // { propId } 打开语音录入
  search: false,
  collapsed: false,
})
export const openProp = id => { ui.drawerId = id }
export const openCapture = propId => { ui.capture = { propId } }

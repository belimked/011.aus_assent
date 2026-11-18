<template><el-config-provider :locale="epLocale"><router-view v-if="route.meta.bare"/><div v-else class="shell"><aside class="side"><div class="brand"><b>Place Sunnybank</b><small>Rex Capture · Web 工作台</small></div><nav><router-link v-for="r in routes" :key="r.path" :to="r.path" class="ni"><el-icon><component :is="r.meta.icon"/></el-icon>{{ t(r.meta.title) }}</router-link></nav></aside><div class="main"><header class="top"><b>{{ t(route.meta.title) }}</b><div class="grow"/><el-dropdown @command="setLocale"><el-button>{{ LOCALES.find(l=>l.k===locale)?.l }}</el-button><template #dropdown><el-dropdown-menu><el-dropdown-item v-for="l in LOCALES" :key="l.k" :command="l.k">{{ l.l }}</el-dropdown-item></el-dropdown-menu></template></el-dropdown><span>{{ identity?.name }} · {{ identity?.email }}</span><el-button @click="logout">{{ t('退出登录') }}</el-button></header><main class="content"><router-view/></main></div></div></el-config-provider></template>
<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import zhHk from 'element-plus/es/locale/lang/zh-hk'
import { locale, LOCALES, setLocale, t } from './i18n'
import { routes } from './router'
import { identity, logout } from './api'
const route = useRoute()
const epLocale = computed(() => ({'zh-CN':zhCn,'zh-HK':zhHk,en})[locale.value])
</script>
<style scoped>.shell{display:flex;height:100vh}.side{width:220px;background:#0f2a44;color:white;display:flex;flex-direction:column}.brand{padding:24px 18px;display:grid;gap:5px}.brand small{color:#c9a227}nav{padding:10px;overflow:auto}.ni{display:flex;gap:12px;align-items:center;padding:11px;color:#c4cfdd;text-decoration:none;border-radius:8px}.ni:hover,.router-link-active{background:#254563;color:white}.main{flex:1;min-width:0;display:flex;flex-direction:column}.top{height:62px;background:white;border-bottom:1px solid #ddd;display:flex;align-items:center;gap:16px;padding:0 22px}.content{flex:1;overflow:auto}</style>

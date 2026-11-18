<template>
  <el-config-provider :locale="epLocale">
    <router-view v-if="route.meta.bare" />
    <div v-else class="shell" :class="{ collapsed }">
      <aside class="side">
        <div class="brand" @click="router.push('/dashboard')">
          <div class="logo">
            <svg viewBox="0 0 32 32" aria-hidden="true">
              <path d="M6 16l10-9 10 9v10h-7v-7h-6v7H6z" fill="#c9a227" />
            </svg>
          </div>
          <div v-if="!collapsed" class="bt">
            <div class="serif b1">Place Sunnybank</div>
            <div class="b2">{{ t("业主信息采集平台") }}</div>
          </div>
        </div>
        <nav>
          <template v-for="group in navGroups" :key="group.title">
            <div v-if="!collapsed" class="ng">{{ t(group.title) }}</div>
            <router-link
              v-for="item in group.items"
              :key="item.path"
              :to="item.path"
              class="ni"
              :title="t(item.meta.title)"
            >
              <el-icon :size="18"><component :is="item.meta.icon" /></el-icon>
              <span v-if="!collapsed">{{ t(item.meta.title) }}</span>
            </router-link>
          </template>
        </nav>
        <button
          class="collapse"
          :aria-label="t(collapsed ? '展开侧栏' : '收起侧栏')"
          @click="collapsed = !collapsed"
        >
          <el-icon
            ><component :is="collapsed ? 'DArrowRight' : 'DArrowLeft'"
          /></el-icon>
        </button>
      </aside>
      <div class="main">
        <header class="top">
          <div class="crumb">
            <el-icon :size="18" color="#8792a2"
              ><component :is="route.meta.icon"
            /></el-icon>
            <b>{{ t(route.meta.title) }}</b>
          </div>
          <div class="sp" />
          <el-dropdown trigger="click" @command="setLocale">
            <el-button class="lang" :icon="Switch">{{
              LOCALES.find((l) => l.k === locale)?.l
            }}</el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="language in LOCALES"
                  :key="language.k"
                  :command="language.k"
                >
                  <span class="lshort">{{ language.short }}</span>
                  {{ language.l }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-dropdown trigger="click" @command="logout">
            <div class="me">
              <Avatar :name="identity?.name" :size="32" />
              <div class="mt">
                <div class="mn">{{ identity?.name }}</div>
                <div class="small muted">{{ identity?.email }}</div>
              </div>
              <el-icon><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">{{
                  t("退出登录")
                }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </header>
        <main class="content"><router-view /></main>
      </div>
    </div>
  </el-config-provider>
</template>

<script setup>
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Switch } from "@element-plus/icons-vue";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import zhHk from "element-plus/es/locale/lang/zh-hk";
import en from "element-plus/es/locale/lang/en";
import { locale, LOCALES, setLocale, t } from "./i18n";
import { routes } from "./router";
import { identity, logout } from "./api";
import Avatar from "./components/Avatar.vue";

const route = useRoute();
const router = useRouter();
const collapsed = ref(false);
const epLocale = computed(
  () => ({ "zh-CN": zhCn, "zh-HK": zhHk, en })[locale.value],
);
const navGroups = [
  {
    title: "工作",
    items: routes.filter((item) =>
      ["/dashboard", "/map", "/properties"].includes(item.path),
    ),
  },
  {
    title: "管理",
    items: routes.filter(
      (item) => !["/dashboard", "/map", "/properties"].includes(item.path),
    ),
  },
];
watch(
  () => [locale.value, route.meta.title],
  () => {
    document.title = `${t(route.meta.title || "工作台")} · Place Sunnybank`;
  },
  { immediate: true },
);
</script>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
}
.side {
  width: 232px;
  flex: none;
  background: linear-gradient(180deg, var(--navy), #0b2135);
  color: #d6deea;
  display: flex;
  flex-direction: column;
  position: relative;
  transition: width 0.2s;
}
.collapsed .side {
  width: 68px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 16px 16px;
  cursor: pointer;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
}
.logo {
  width: 36px;
  height: 36px;
  border-radius: 9px;
  background: rgba(201, 162, 39, 0.14);
  border: 1px solid rgba(201, 162, 39, 0.45);
  display: grid;
  place-items: center;
  flex: none;
}
.logo svg {
  width: 22px;
}
.b1 {
  color: #fff;
  font-size: 16px;
  font-weight: 700;
}
.b2 {
  font-size: 11px;
  color: var(--gold-2);
  letter-spacing: 1px;
  margin-top: 1px;
}
nav {
  flex: 1;
  overflow: auto;
  padding: 8px 10px 40px;
}
.ng {
  font-size: 11px;
  letter-spacing: 1.5px;
  color: #6f87a3;
  padding: 14px 10px 6px;
}
.ni {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 9px 11px;
  border-radius: 9px;
  text-decoration: none;
  color: #c4cfdd;
  font-size: 14px;
  margin-bottom: 2px;
}
.collapsed .ni {
  justify-content: center;
}
.ni:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #fff;
}
.ni.router-link-active {
  background: linear-gradient(
    90deg,
    rgba(201, 162, 39, 0.22),
    rgba(201, 162, 39, 0.05)
  );
  color: #fff;
  box-shadow: inset 3px 0 0 var(--gold);
}
.ni.router-link-active .el-icon {
  color: var(--gold-2);
}
.collapse {
  position: absolute;
  bottom: 10px;
  right: 12px;
  background: rgba(255, 255, 255, 0.06);
  border: 0;
  color: #9fb0c4;
  border-radius: 7px;
  width: 28px;
  height: 26px;
  cursor: pointer;
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.top {
  height: 60px;
  flex: none;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 22px;
  background: #fff;
  border-bottom: 1px solid var(--line);
}
.crumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
}
.sp {
  flex: 1;
}
.me {
  display: flex;
  align-items: center;
  gap: 9px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 10px;
}
.me:hover {
  background: #f4f5f7;
}
.mn {
  font-weight: 650;
  font-size: 14px;
  line-height: 1.2;
}
.content {
  flex: 1;
  overflow: auto;
}
.lang {
  font-weight: 600;
}
.lshort {
  display: inline-grid;
  place-items: center;
  width: 26px;
  height: 20px;
  margin-right: 6px;
  border-radius: 5px;
  background: #eef2f6;
  font-size: 11px;
  font-weight: 700;
  color: var(--navy);
}
</style>

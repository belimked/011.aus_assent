import { createRouter, createWebHashHistory } from "vue-router";
import { token, identity, restore } from "./api";

export const routes = [
  {
    path: "/dashboard",
    component: () => import("./views/AdminDashboard.vue"),
    meta: { title: "工作台", icon: "Odometer" },
  },
  {
    path: "/cockpit",
    component: () => import("./views/AdminCockpit.vue"),
    meta: { title: "数据驾驶舱", icon: "DataLine" },
  },
  {
    path: "/export",
    component: () => import("./views/AdminExport.vue"),
    meta: { title: "数据导出", icon: "Download", adminOnly: true },
  },
  {
    path: "/settings",
    component: () => import("./views/AdminSettings.vue"),
    meta: { title: "系统设置", icon: "Setting", adminOnly: true },
  },
  {
    path: "/map",
    component: () => import("./views/AdminMap.vue"),
    meta: { title: "区域地图", icon: "MapLocation" },
  },
  {
    path: "/properties",
    component: () => import("./views/AdminProperties.vue"),
    meta: { title: "房产列表", icon: "OfficeBuilding" },
  },
  {
    path: "/team",
    component: () => import("./views/AdminTeam.vue"),
    meta: { title: "团队与权限", icon: "UserFilled" },
  },
  {
    path: "/review",
    component: () => import("./views/Review.vue"),
    meta: { title: "审核队列", icon: "Stamp" },
  },
  {
    path: "/tasks",
    component: () => import("./views/Tasks.vue"),
    meta: { title: "跟进任务", icon: "Calendar" },
  },
  {
    path: "/ofi",
    component: () => import("./views/Ofi.vue"),
    meta: { title: "开放参观", icon: "House" },
  },
  {
    path: "/audit",
    component: () => import("./views/AdminTables.vue"),
    props: { page: "audit" },
    meta: { title: "审计日志", icon: "Tickets" },
  },
  {
    path: "/rex",
    component: () => import("./views/AdminTables.vue"),
    props: { page: "rex" },
    meta: { title: "Rex 队列", icon: "Connection" },
  },
  {
    path: "/rex-log",
    component: () => import("./views/AdminRexLog.vue"),
    meta: { title: "Rex 调用日志", icon: "Tickets", adminOnly: true },
  },
  {
    path: "/market",
    component: () => import("./views/AdminTables.vue"),
    props: { page: "market" },
    meta: { title: "市场数据", icon: "Upload" },
  },
  {
    path: "/data",
    component: () => import("./views/AdminTables.vue"),
    props: { page: "data" },
    meta: { title: "数据浏览", icon: "Grid" },
  },
];

const router = createRouter({
  history: createWebHashHistory("/admin/"),
  routes: [
    { path: "/", redirect: "/dashboard" },
    {
      path: "/login",
      component: () => import("./views/AdminLogin.vue"),
      meta: { bare: true, title: "登录" },
    },
    ...routes,
    { path: "/:pathMatch(.*)*", redirect: "/dashboard" },
  ],
});
router.beforeEach(async (to) => {
  if (to.path === "/login") return true;
  if (!token.value || (!identity.value && !(await restore()))) return "/login";
  if (identity.value?.role === "leader" &&
    (["/audit", "/rex", "/rex-log", "/market", "/data", "/export", "/settings"].includes(to.path)))
    return "/dashboard";
  return true;
});
export default router;

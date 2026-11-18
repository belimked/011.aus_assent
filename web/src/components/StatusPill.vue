<template>
  <span class="st-pill" :style="{ background: config.bg, color: config.color }">
    <span
      class="dot"
      :style="{ background: config.color, width: '7px', height: '7px' }"
    />
    {{ t(short ? config.short : config.label) }}
  </span>
</template>

<script setup>
import { computed } from "vue";
import { t } from "../i18n";

const props = defineProps({ status: String, short: Boolean });
const statuses = {
  owned: {
    label: "有产权信息",
    short: "产权",
    color: "#16803d",
    bg: "#e9f6ed",
  },
  registered: {
    label: "已登记",
    short: "登记",
    color: "#9a7110",
    bg: "#faf2d9",
  },
  unregistered: {
    label: "未登记",
    short: "未登记",
    color: "#64748b",
    bg: "#eef1f4",
  },
  active: { label: "启用", short: "启用", color: "#16803d", bg: "#e9f6ed" },
  disabled: { label: "禁用", short: "禁用", color: "#c0392b", bg: "#fcecea" },
  yes: { label: "是", short: "是", color: "#16803d", bg: "#e9f6ed" },
  no: { label: "否", short: "否", color: "#64748b", bg: "#eef1f4" },
  pending: {
    label: "待审核",
    short: "待审核",
    color: "#9a7110",
    bg: "#faf2d9",
  },
  approved: {
    label: "已批准",
    short: "已批准",
    color: "#16803d",
    bg: "#e9f6ed",
  },
  rejected: {
    label: "已拒绝",
    short: "已拒绝",
    color: "#c0392b",
    bg: "#fcecea",
  },
  failed: { label: "失败", short: "失败", color: "#c0392b", bg: "#fcecea" },
  sent: { label: "已发送", short: "已发送", color: "#16803d", bg: "#e9f6ed" },
  waiting_auth: {
    label: "等待授权",
    short: "等待授权",
    color: "#9a7110",
    bg: "#faf2d9",
  },
  skipped: {
    label: "已跳过",
    short: "已跳过",
    color: "#64748b",
    bg: "#eef1f4",
  },
};
const config = computed(
  () =>
    statuses[props.status] || {
      label: props.status || "—",
      short: props.status || "—",
      color: "#64748b",
      bg: "#eef1f4",
    },
);
</script>

<template>
  <span class="st-pill" :style="{ background: s.bg, color: s.color }"><span class="dot" :style="{ background: s.color, width: '7px', height: '7px' }" />{{ t(short ? s.short : s.label) }}</span>
</template>
<script setup>
import { computed } from 'vue'
import { STATUS } from '../data/store'
import { t } from '../i18n'
const props = defineProps({ status: String, short: Boolean })
const s = computed(() => STATUS[props.status] || STATUS.grey)
</script>

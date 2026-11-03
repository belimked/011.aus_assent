<template>
  <span class="avatar" :style="{ width: size + 'px', height: size + 'px', fontSize: size * .38 + 'px', background: u?.color || '#94a3b8' }" :title="u?.name">{{ u ? initials(u.name) : '?' }}</span>
</template>
<script setup>
import { computed } from 'vue'
import { userById, initials } from '../data/store'
const props = defineProps({ id: String, size: { type: Number, default: 28 } })
const u = computed(() => userById(props.id))
</script>

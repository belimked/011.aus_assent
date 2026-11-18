<template>
  <span
    class="avatar"
    :style="{
      width: size + 'px',
      height: size + 'px',
      fontSize: size * 0.38 + 'px',
      background: color,
    }"
    :title="displayName"
    >{{ initials(displayName) }}</span
  >
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
  id: String,
  name: String,
  size: { type: Number, default: 28 },
});
const displayName = computed(() => props.name || "?");
const initials = (name) =>
  name
    .split(/\s+/)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
const color = computed(() => (props.name ? "#b8860b" : "#94a3b8"));
</script>

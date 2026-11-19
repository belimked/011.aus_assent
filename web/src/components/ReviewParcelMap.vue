<template>
  <div ref="element" class="review-map" :aria-label="t('候选地块地图')" />
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import L from 'leaflet';
import { osmLayer } from './tiles';
import { t } from '../i18n';

const props = defineProps({ candidate: Object });
const element = ref(null);
let map;
let marker;
let parcels = new Map();

function draw() {
  if (!map) return;
  marker?.remove();
  marker = null;
  const parcel = parcels.get(props.candidate?.LotPlan);
  if (parcel) {
    const shape = parcel.r.map(flat => {
      const ring = [];
      for (let i = 0; i < flat.length; i += 2) {
        ring.push([parcels.origin[1] + flat[i + 1] / 1e6, parcels.origin[0] + flat[i] / 1e6]);
      }
      return ring;
    });
    marker = L.polygon(shape, { color: '#ad8120', weight: 3, fillColor: '#e6c35c', fillOpacity: 0.3 }).addTo(map);
    map.fitBounds(marker.getBounds(), { padding: [40, 40], maxZoom: 18 });
  } else if (Array.isArray(props.candidate?.LL) && props.candidate.LL.length === 2) {
    marker = L.circleMarker(props.candidate.LL, {
      radius: 10, color: '#ad8120', weight: 3, fillColor: '#e6c35c', fillOpacity: 0.5,
    }).addTo(map);
    map.setView(props.candidate.LL, 17);
  }
  nextTick(() => map?.invalidateSize());
}

onMounted(async () => {
  map = L.map(element.value, { scrollWheelZoom: false }).setView([-27.58, 153.06], 13);
  osmLayer().addTo(map);
  try {
    const response = await fetch('/admin/parcels.json');
    if (response.ok) {
      const source = await response.json();
      parcels = new Map(source.parcels.map(parcel => [parcel.lp, parcel]));
      parcels.origin = source.origin;
    }
  } catch {
    // A missing public cadastral file still allows a coordinate-only marker.
  }
  draw();
});
watch(() => props.candidate, draw);
onBeforeUnmount(() => map?.remove());
</script>

<style scoped>
.review-map { height: 220px; border-radius: 10px; margin: 16px 0; background: #e9e7e0; }
</style>

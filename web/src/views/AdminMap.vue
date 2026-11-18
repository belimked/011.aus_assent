<template>
  <div class="mv">
    <aside class="panel">
      <div class="ph">
        <div class="serif heading">{{ t("区域地图") }}</div>
        <el-input
          v-model="query"
          :prefix-icon="Search"
          :placeholder="t('地址或地块编号')"
          clearable
        />
        <div class="chips">
          <button
            v-for="choice in filters"
            :key="choice.key"
            class="chip"
            :class="{ on: filter === choice.key }"
            @click="filter = choice.key"
          >
            <i v-if="choice.color" :style="{ background: choice.color }" />
            {{ t(choice.label) }} <span>{{ count(choice.key) }}</span>
          </button>
        </div>
        <div class="row small muted">
          {{ t("视野内 {n} 处地块", { n: parcels.length }) }}
          <div class="grow" />
          <span v-if="zoom < 16">{{ t("放大查看地块") }}</span>
        </div>
      </div>
      <el-scrollbar class="list">
        <div
          v-for="parcel in shownParcels"
          :key="parcel.id"
          class="li"
          @click="openParcel(parcel.id)"
        >
          <span class="dot" :style="{ background: statusColor(parcel) }" />
          <div class="grow">
            <b>{{ parcel.address }}</b>
            <div class="small muted">
              {{ parcel.suburb }} · {{ parcel.lotplan }}
            </div>
          </div>
          <el-icon class="muted"><ArrowRight /></el-icon>
        </div>
        <el-empty
          v-if="zoom >= 16 && !shownParcels.length"
          :description="t('视野内暂无地块')"
          :image-size="60"
        />
        <el-button
          v-if="shown < filteredParcels.length"
          link
          class="more"
          @click="shown += 50"
        >
          {{
            t("再显示 {n} 条", {
              n: Math.min(50, filteredParcels.length - shown),
            })
          }}
        </el-button>
        <div class="zone-section">
          <div class="row">
            <h3 class="grow">{{ t("经纪人区域") }}</h3>
            <el-button size="small" :icon="Plus" @click="startCreate">{{
              t("创建区域")
            }}</el-button>
          </div>
          <div v-for="group in groupedZones" :key="group.id" class="zone-group">
            <div class="row agent-label">
              <Avatar :name="group.name" :size="24" />{{ group.name }}
            </div>
            <div v-for="zone in group.zones" :key="zone.id" class="zone-row">
              <span
                class="zone-swatch"
                :style="{ background: agentColor(group.id) }"
              />
              <span class="grow">{{ zone.name }}</span>
              <el-button link size="small" @click="editZone(zone)">{{
                t("编辑")
              }}</el-button>
              <el-button
                link
                size="small"
                type="danger"
                @click="remove(zone)"
                >{{ t("删除") }}</el-button
              >
            </div>
          </div>
        </div>
      </el-scrollbar>
    </aside>
    <div class="mapwrap">
      <div ref="mapElement" class="map" />
      <div v-if="zoom < 16" class="zoom-hint">{{ t("放大查看地块") }}</div>
      <div class="legend card">
        <div class="small muted">{{ t("地块状态 · 当前视野") }}</div>
        <div
          v-for="item in filters.slice(1)"
          :key="item.key"
          class="row legend-row"
        >
          <span class="dot" :style="{ background: item.color }" />{{
            t(item.label)
          }}
          <span class="grow" /><b>{{ count(item.key) }}</b>
        </div>
        <div class="small muted">
          {{ t("地块边界：昆士兰州政府 CC BY 4.0") }}
        </div>
      </div>
      <div v-if="editorOpen" class="editor card">
        <div class="row">
          <h3 class="grow">{{ t(editing ? "编辑区域" : "创建区域") }}</h3>
          <el-button link :icon="Close" @click="cancel" />
        </div>
        <label>{{ t("经纪人") }}</label>
        <el-select v-model="agent" class="full">
          <el-option
            v-for="user in users"
            :key="user.id"
            :label="user.name"
            :value="user.id"
          />
        </el-select>
        <label>{{ t("区域名称") }}</label>
        <el-input v-model="form.name" :placeholder="t('区域名称')" />
        <label>{{ t("区域方式") }}</label>
        <el-select v-model="mode" class="full" @change="resetShape">
          <el-option value="polygon" :label="t('多边形')" />
          <el-option value="radius" :label="t('中心与半径')" />
          <el-option value="suburbs" :label="t('按区域')" />
        </el-select>
        <template v-if="mode === 'suburbs'">
          <label>{{ t("选择区域") }}</label>
          <el-select
            v-model="suburbs"
            multiple
            class="full"
            :placeholder="t('选择区域')"
          >
            <el-option
              v-for="name in suburbNames"
              :key="name"
              :value="name"
              :label="name"
            />
          </el-select>
        </template>
        <template v-else>
          <p class="small muted">
            {{
              t(
                mode === "polygon"
                  ? "点击地图添加顶点（至少 3 个）"
                  : "点击地图选择中心",
              )
            }}
          </p>
          <div class="row">
            <el-button
              v-if="mode === 'polygon'"
              size="small"
              :disabled="!points.length"
              @click="undo"
            >
              {{ t("撤销上一顶点") }} ({{ points.length }})
            </el-button>
            <el-button size="small" @click="cancel">{{
              t("取消绘制")
            }}</el-button>
          </div>
          <template v-if="mode === 'radius'">
            <label>{{ t("半径（米）") }}</label>
            <el-input-number
              v-model="form.radius"
              :min="1"
              :max="50000"
              class="full"
            />
          </template>
        </template>
        <el-button type="primary" class="full save" @click="save">{{
          t("保存区域")
        }}</el-button>
      </div>
    </div>
    <AdminParcelDrawer :parcel="selected" @close="selected = null" />
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useRoute } from "vue-router";
import { Search, Plus, Close } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import L from "leaflet";
import { api } from "../api";
import { parcelStatus } from "../admin-format";
import { t } from "../i18n";
import AdminParcelDrawer from "../components/AdminParcelDrawer.vue";
import Avatar from "../components/Avatar.vue";

const route = useRoute();
const mapElement = ref(null);
const zoom = ref(16);
const parcels = ref([]);
const users = ref([]);
const zones = ref([]);
const suburbNames = ref([]);
const selected = ref(null);
const query = ref("");
const filter = ref("all");
const shown = ref(50);
const editorOpen = ref(false);
const agent = ref("");
const editing = ref(null);
const mode = ref("polygon");
const suburbs = ref([]);
const points = ref([]);
const form = reactive({ name: "", radius: 500 });
const colors = [
  "#c9a227",
  "#2563eb",
  "#dc2626",
  "#16a34a",
  "#8b5cf6",
  "#0e7490",
];
const filters = [
  { key: "all", label: "全部" },
  { key: "owned", label: "有产权信息", color: "#16a34a" },
  { key: "registered", label: "已登记无产权", color: "#c9a227" },
  { key: "unregistered", label: "未登记", color: "#9ca3af" },
];
const statusColor = (parcel) =>
  filters.find((item) => item.key === parcelStatus(parcel))?.color || "#9ca3af";
const filteredParcels = computed(() =>
  parcels.value.filter(
    (parcel) =>
      (filter.value === "all" || parcelStatus(parcel) === filter.value) &&
      (!query.value ||
        `${parcel.address} ${parcel.lotplan}`
          .toLowerCase()
          .includes(query.value.toLowerCase())),
  ),
);
const shownParcels = computed(() =>
  filteredParcels.value.slice(0, shown.value),
);
const count = (key) =>
  key === "all"
    ? parcels.value.length
    : parcels.value.filter((parcel) => parcelStatus(parcel) === key).length;
const groupedZones = computed(() =>
  users.value
    .map((user) => ({
      id: user.id,
      name: user.name,
      zones: zones.value.filter(
        (zone) =>
          zone.ownerId === user.id &&
          (!route.query.agent || zone.ownerId === route.query.agent),
      ),
    }))
    .filter((group) => group.zones.length),
);
const agentColor = (id) =>
  colors[
    Math.max(
      0,
      users.value.findIndex((user) => user.id === id),
    ) % colors.length
  ];
let map;
let parcelLayer;
let zoneLayer;
let drawingLayer;
let rings;
let boundsRequest = 0;

// Public Queensland cadastral polygons: [lng,lat] micro-degree offsets from origin.
function decodeRings(parcel, origin) {
  return parcel.r.map((flat) => {
    const points = [];
    for (let i = 0; i < flat.length; i += 2) {
      points.push([origin[1] + flat[i + 1] / 1e6, origin[0] + flat[i] / 1e6]);
    }
    return points;
  });
}
function drawZones() {
  if (!zoneLayer) return;
  zoneLayer.clearLayers();
  for (const zone of zones.value) {
    if (route.query.agent && zone.ownerId !== route.query.agent) continue;
    const options = {
      color: agentColor(zone.ownerId),
      weight: 3,
      fillOpacity: 0.07,
    };
    if (zone.pts?.length >= 3) L.polygon(zone.pts, options).addTo(zoneLayer);
    if (zone.center?.length === 2)
      L.circle(zone.center, { ...options, radius: zone.radius }).addTo(
        zoneLayer,
      );
    // Highlight actual parcels in a suburb instead of inventing a suburb boundary.
    const suburbs = zone.suburbs || (zone.suburb ? [zone.suburb] : []);
    if (map.getZoom() >= 16 && suburbs.length) {
      for (const parcel of parcels.value) {
        if (!suburbs.includes(parcel.suburb)) continue;
        const shape = rings.get(parcel.lotplan);
        if (!shape) continue;
        L.polygon(shape.rings, {
          color: agentColor(zone.ownerId),
          weight: 0.8,
          fillOpacity: 0.03,
          interactive: false,
        }).addTo(zoneLayer);
      }
    }
  }
}
function drawDraft() {
  if (!drawingLayer) return;
  drawingLayer.clearLayers();
  if (mode.value === "polygon" && points.value.length) {
    L.polyline(points.value, { color: "#dc2626", weight: 3 }).addTo(
      drawingLayer,
    );
    for (const point of points.value)
      L.circleMarker(point, { radius: 4, color: "#dc2626" }).addTo(
        drawingLayer,
      );
  }
  if (mode.value === "radius" && points.value.length) {
    L.circle(points.value[0], { radius: form.radius, color: "#dc2626" }).addTo(
      drawingLayer,
    );
  }
}
async function refreshParcels() {
  if (!map) return;
  zoom.value = map.getZoom();
  parcelLayer.clearLayers();
  parcels.value = [];
  if (zoom.value < 16) {
    drawZones();
    return;
  }
  const sequence = ++boundsRequest;
  const bounds = map.getBounds();
  const bbox = [
    bounds.getWest(),
    bounds.getSouth(),
    bounds.getEast(),
    bounds.getNorth(),
  ].join(",");
  try {
    const status = await api(`admin/properties?bbox=${bbox}&limit=5000`);
    if (sequence !== boundsRequest) return;
    const visible = [];
    for (const feature of status.features) {
      const info = feature.properties;
      const cadastral = rings?.get(info.lotplan);
      if (!cadastral) continue;
      const coordinates = cadastral.rings;
      if (
        !coordinates.some((ring) =>
          ring.some((point) => bounds.contains(point)),
        )
      )
        continue;
      const parcel = {
        ...info,
        address: cadastral.address,
        suburb: cadastral.suburb,
      };
      visible.push(parcel);
      const polygon = L.polygon(coordinates, {
        color: statusColor(parcel),
        weight: 1.5,
        fillColor: statusColor(parcel),
        fillOpacity: 0.38,
      })
        .on("click", () => {
          if (!editorOpen.value) openParcel(parcel.id);
        })
        .addTo(parcelLayer);
      polygon.parcelStatus = parcelStatus(parcel);
    }
    parcels.value = visible;
    shown.value = 50;
    drawZones();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function openParcel(id) {
  try {
    const found = await api(`admin/properties?q=${encodeURIComponent(id)}`);
    selected.value = found.find((parcel) => parcel.id === id) || null;
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function refreshZones() {
  zones.value = await api("admin/zones");
  drawZones();
}
function startCreate() {
  editing.value = null;
  agent.value = route.query.agent || users.value[0]?.id || "";
  form.name = "";
  mode.value = "polygon";
  resetShape();
  editorOpen.value = true;
}
function resetShape() {
  points.value = [];
  suburbs.value = [];
  drawDraft();
}
function undo() {
  points.value.pop();
  drawDraft();
}
function cancel() {
  editorOpen.value = false;
  editing.value = null;
  resetShape();
}
function editZone(zone) {
  editing.value = zone;
  agent.value = zone.ownerId;
  form.name = zone.name || "";
  form.radius = zone.radius || 500;
  mode.value = zone.pts ? "polygon" : zone.center ? "radius" : "suburbs";
  points.value = zone.pts || (zone.center ? [zone.center] : []);
  suburbs.value = zone.suburbs || (zone.suburb ? [zone.suburb] : []);
  editorOpen.value = true;
  drawDraft();
}
async function save() {
  try {
    if (!agent.value || !form.name.trim())
      throw Error(t("选择经纪人与区域名称"));
    if (mode.value === "polygon" && points.value.length < 3)
      throw Error(t("至少添加 3 个顶点"));
    if (mode.value === "radius" && !points.value.length)
      throw Error(t("点击地图选择中心"));
    if (mode.value === "suburbs" && !suburbs.value.length)
      throw Error(t("选择区域"));
    const data = {
      ownerId: agent.value,
      name: form.name.trim(),
      suburbs: null,
      suburb: null,
      pts: null,
      center: null,
      radius: null,
      ...(mode.value === "suburbs"
        ? { suburbs: suburbs.value }
        : mode.value === "radius"
          ? { center: points.value[0], radius: form.radius }
          : { pts: points.value }),
    };
    if (editing.value) {
      data.version = editing.value.version;
      await api(`admin/zones/${encodeURIComponent(editing.value.id)}`, {
        method: "PATCH",
        body: JSON.stringify(data),
      });
    } else {
      await api("admin/zones", { method: "POST", body: JSON.stringify(data) });
    }
    cancel();
    await refreshZones();
    ElMessage.success(t("已保存"));
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function remove(zone) {
  try {
    await ElMessageBox.confirm(t("确定删除此区域？"));
    await api(`admin/zones/${encodeURIComponent(zone.id)}`, {
      method: "DELETE",
    });
    await refreshZones();
  } catch (error) {
    if (error instanceof Error) ElMessage.error(error.message);
  }
}

onMounted(async () => {
  try {
    const [usersResult, stats, shapeData] = await Promise.all([
      api("admin/users"),
      api("admin/parcel-stats"),
      fetch("/admin/parcels.json").then((response) => {
        if (!response.ok) throw Error(t("地块边界加载失败"));
        return response.json();
      }),
    ]);
    users.value = usersResult.filter((user) => user.role === "agent");
    suburbNames.value = stats.map((row) => row.suburb).filter(Boolean);
    // The cadastral file has one ring per lot/plan; address tuples supply display labels.
    const addresses = new Map(
      shapeData.addresses.map((tuple) => [tuple[4], tuple]),
    );
    rings = new Map(
      shapeData.parcels.map((parcel) => {
        const address = addresses.get(parcel.lp);
        return [
          parcel.lp,
          {
            rings: decodeRings(parcel, shapeData.origin),
            address: address ? `${address[1]} ${address[2]}` : parcel.lp,
            suburb: address?.[3] || "",
          },
        ];
      }),
    );
    await nextTick();
    map = L.map(mapElement.value).setView([-27.581, 153.063], 16);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      attribution:
        "© OpenStreetMap · Addresses & parcels © State of Queensland (CC BY 4.0)",
    }).addTo(map);
    parcelLayer = L.layerGroup().addTo(map);
    zoneLayer = L.layerGroup().addTo(map);
    drawingLayer = L.layerGroup().addTo(map);
    map.on("moveend", refreshParcels);
    map.on("click", (event) => {
      if (!editorOpen.value || mode.value === "suburbs") return;
      if (mode.value === "radius")
        points.value = [[event.latlng.lat, event.latlng.lng]];
      else points.value.push([event.latlng.lat, event.latlng.lng]);
      drawDraft();
    });
    await refreshZones();
    refreshParcels();
  } catch (error) {
    ElMessage.error(error.message);
  }
});
onBeforeUnmount(() => {
  boundsRequest++;
  map?.remove();
});
watch(() => form.radius, drawDraft);
watch(() => route.query.agent, drawZones);
watch(filter, (value) => {
  shown.value = 50;
  parcelLayer?.eachLayer((layer) => {
    const opacity =
      value === "all" || layer.parcelStatus === value ? 0.38 : 0.06;
    layer.setStyle({
      fillOpacity: opacity,
      opacity: opacity === 0.38 ? 1 : 0.2,
    });
  });
});
</script>

<style scoped>
.mv {
  display: flex;
  height: 100%;
}
.panel {
  width: 340px;
  flex: none;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-right: 1px solid var(--line);
}
.ph {
  padding: 16px 14px 12px;
  border-bottom: 1px solid var(--line);
}
.heading {
  font-size: 20px;
  margin-bottom: 12px;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin: 12px 0;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 9px;
  border: 1px solid var(--line);
  background: #fff;
  border-radius: 15px;
  cursor: pointer;
  font-size: 12px;
}
.chip.on {
  color: #fff;
  background: var(--navy);
  border-color: var(--navy);
}
.chip i,
.zone-swatch {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.list {
  flex: 1;
  min-height: 0;
}
.li {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
  border-bottom: 1px solid var(--line);
  cursor: pointer;
  font-size: 13px;
}
.li:hover {
  background: #f8f7f3;
}
.li .dot {
  width: 10px;
  height: 10px;
}
.more {
  margin: 10px;
}
.zone-section {
  padding: 14px;
  border-top: 1px solid var(--line);
}
.zone-section h3,
.editor h3 {
  font-size: 15px;
  margin: 0;
}
.zone-group {
  margin-top: 14px;
}
.agent-label {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 6px;
}
.zone-row {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  padding: 5px 0 5px 32px;
}
.mapwrap {
  flex: 1;
  min-width: 0;
  position: relative;
}
.map {
  height: 100%;
}
.legend,
.zoom-hint,
.editor {
  position: absolute;
  z-index: 500;
  background: #fff;
  box-shadow: 0 4px 18px rgba(15, 42, 68, 0.12);
}
.legend {
  top: 14px;
  right: 14px;
  width: 236px;
  padding: 14px;
}
.legend-row {
  margin: 8px 0;
  font-size: 12px;
}
.zoom-hint {
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  padding: 8px 16px;
  border-radius: 8px;
}
.editor {
  bottom: 20px;
  left: 20px;
  width: 290px;
  padding: 16px;
  border-radius: 12px;
}
.editor label {
  display: block;
  margin: 12px 0 5px;
  color: var(--mut);
  font-size: 12px;
}
.editor p {
  margin: 10px 0;
}
.full {
  width: 100%;
}
.save {
  margin-top: 18px;
}
</style>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t("团队与权限") }}</h1>
        <div class="sub">{{ t("账户角色、访问与区域分配") }}</div>
      </div>
      <div class="sp" />
      <el-button v-if="identity?.role === 'admin'" type="primary" :icon="Plus" @click="createOpen = true">{{
        t("创建账户")
      }}</el-button>
    </div>
    <div v-if="identity?.role === 'admin'" class="card team-manage">
      <h3>{{ t('团队') }}</h3>
      <div v-for="group in teams" :key="group.id" class="row">
        <b class="grow">{{ group.name }}</b>
        <span class="small muted">{{ users.filter(user => user.teamId === group.id).length }} {{ t('人') }}</span>
        <el-button link @click="renameTeam(group)">{{ t('重命名') }}</el-button>
      </div>
      <el-button @click="createTeam">{{ t('创建团队') }}</el-button>
      <div v-if="selected" class="row">
        <label>{{ t('角色') }}
          <el-select :model-value="selected.role" @change="value => patch({ role: value })" style="width: 140px">
            <el-option value="agent" :label="t('经纪人')" />
            <el-option value="leader" :label="t('团队负责人')" />
            <el-option value="admin" :label="t('管理员')" />
          </el-select>
        </label>
        <label>{{ t('团队') }}
          <el-select :model-value="selected.teamId" @change="value => patch({ teamId: value })" style="width: 160px">
            <el-option v-for="group in teams" :key="group.id" :value="group.id" :label="group.name" />
          </el-select>
        </label>
      </div>
    </div>
    <div class="tm">
      <div class="ul">
        <template v-for="group in teams" :key="group.id">
          <div class="group-name">{{ group.name }}</div>
        <button
          v-for="user in users.filter(item => item.teamId === group.id)"
          :key="user.id"
          class="uc card"
          :class="{ on: selected?.id === user.id, off: user.disabled }"
          @click="selectedId = user.id"
        >
          <Avatar :name="user.name" :size="44" />
          <div class="grow">
            <div class="row">
              <b>{{ user.name }}</b
              ><el-tag size="small" effect="plain">{{
                roleLabel(user.role)
              }}</el-tag>
            </div>
            <div class="small muted ellipsis">{{ user.email }}</div>
            <div class="small muted">
              {{ relTime(user.lastLogin) }} · {{ user.device || t("无设备") }}
            </div>
          </div>
          <div class="mini">
            <b class="serif">{{ zoneCount(user.id) }}</b
            ><span>{{ t("区域") }}</span>
          </div>
        </button>
        </template>
      </div>
      <div v-if="selected" class="card ud">
        <div class="uh">
          <Avatar :name="selected.name" :size="64" />
          <div class="grow">
            <div class="serif nm">{{ selected.name }}</div>
            <div class="muted">
              {{ selected.email }} · {{ roleLabel(selected.role) }}
            </div>
          </div>
          <StatusPill :status="selected.disabled ? 'disabled' : 'active'" />
          <el-dropdown v-if="identity?.role === 'admin'" @command="command">
            <el-button :icon="MoreFilled" circle :aria-label="t('账户操作')" />
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="role">{{
                  t("切换角色")
                }}</el-dropdown-item>
                <el-dropdown-item command="disable">{{
                  t(selected.disabled ? "启用" : "禁用")
                }}</el-dropdown-item>
                <el-dropdown-item command="password">{{
                  t("重置密码")
                }}</el-dropdown-item>
                <el-dropdown-item command="revoke">{{
                  t("撤销会话")
                }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <div class="stats">
          <div>
            <div class="small muted">{{ t("最近登录") }}</div>
            <b>{{ relTime(selected.lastLogin) }}</b>
          </div>
          <div>
            <div class="small muted">{{ t("设备") }}</div>
            <b>{{ selected.device || "—" }}</b>
          </div>
          <div>
            <div class="small muted">{{ t("管理区域") }}</div>
            <b class="serif">{{ zoneCount(selected.id) }}</b>
          </div>
        </div>
        <div class="row section-heading">
          <h3 class="grow">{{ t('能力雷达') }}</h3>
        </div>
        <v-chart v-if="agentStats" class="radar" :option="radarOption" autoresize />
        <el-empty v-else :description="t('暂无活动')" :image-size="70" />
        <div class="row section-heading">
          <h3 class="grow">{{ t("经纪人区域") }}</h3>
          <router-link :to="`/map?agent=${selected.id}`">{{
            t("地图查看 →")
          }}</router-link>
        </div>
        <div v-for="zone in selectedZones" :key="zone.id" class="zone-row">
          <span class="dot" style="background: var(--gold)" />{{ zone.name }}
          <span class="small muted">{{
            zone.suburbs?.join("、") || t(zone.pts ? "多边形" : "中心与半径")
          }}</span>
        </div>
        <el-empty
          v-if="!selectedZones.length"
          :description="t('暂无区域')"
          :image-size="70"
        />
      </div>
    </div>
    <el-dialog v-model="createOpen" :title="t('创建账户')" width="450px">
      <el-form label-position="top" @submit.prevent="create">
        <el-form-item :label="t('邮箱')"
          ><el-input v-model="form.email" type="email"
        /></el-form-item>
        <el-form-item :label="t('姓名')"
          ><el-input v-model="form.name"
        /></el-form-item>
        <el-form-item :label="t('角色')">
          <el-select v-model="form.role" style="width: 100%">
            <el-option value="agent" :label="t('经纪人')" />
            <el-option value="admin" :label="t('管理员')" />
            <el-option value="leader" :label="t('团队负责人')" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('密码')"
          ><el-input v-model="form.password" type="password" show-password
        /></el-form-item>
        <el-button type="primary" native-type="submit">{{
          t("创建账户")
        }}</el-button>
      </el-form>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { MoreFilled, Plus } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { api, identity } from "../api";
import { relTime, roleLabel } from "../admin-format";
import { t } from "../i18n";
import Avatar from "../components/Avatar.vue";
import StatusPill from "../components/StatusPill.vue";
import VChart from '../components/chart';

const route = useRoute();
const users = ref([]);
const zones = ref([]);
const teams = ref([]);
const stats = ref([]);
const agentStats = computed(() => stats.value.find(row => row.id === selectedId.value));
const radarFields = [['visits', '上门拜访'], ['contacts', '新联系人'],
  ['appraisals', '预约估价'], ['linked', '新关联业主'], ['done', '已完成待办']];
const radarOption = computed(() => ({
  radar: { indicator: radarFields.map(([key, label]) => ({ name: t(label), max: 100 })) },
  series: [{ type: 'radar', data: [{ name: selected.value?.name || '',
    value: radarFields.map(([key]) => Math.round(100 * (agentStats.value?.[key] || 0) /
      Math.max(1, ...stats.value.map(row => row[key] || 0)))) }] }],
}));
const selectedId = ref("");
const createOpen = ref(false);
const form = reactive({ email: "", name: "", role: "agent", password: "" });
const selected = computed(() =>
  users.value.find((user) => user.id === selectedId.value),
);
const selectedZones = computed(() =>
  zones.value.filter((zone) => zone.ownerId === selectedId.value),
);
const zoneCount = (id) =>
  zones.value.filter((zone) => zone.ownerId === id).length;

async function load() {
  try {
    [users.value, zones.value, teams.value] = await Promise.all([
      api("admin/users"),
      api("admin/zones"),
      api("admin/teams"),
    ]);
    selectedId.value =
      route.query.agent || selectedId.value || users.value[0]?.id;
    if (selected.value) {
      const data = await api(`admin/metrics?period=30&team=${encodeURIComponent(selected.value.teamId || '')}`);
      stats.value = data.performance || [];
    }
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function createTeam() {
  try {
    const { value } = await ElMessageBox.prompt(t('团队名称'));
    await api('admin/teams', { method: 'POST', body: JSON.stringify({ name: value }) });
    await load();
  } catch { /* Cancelled. */ }
}
async function renameTeam(group) {
  try {
    const { value } = await ElMessageBox.prompt(t('团队名称'), '', { inputValue: group.name });
    await api('admin/teams', { method: 'PATCH', body: JSON.stringify({ id: group.id, name: value }) });
    await load();
  } catch { /* Cancelled. */ }
}
async function create() {
  try {
    await api("admin/users", { method: "POST", body: JSON.stringify(form) });
    createOpen.value = false;
    Object.assign(form, { email: "", name: "", role: "agent", password: "" });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function patch(changes) {
  try {
    await api("admin/users", {
      method: "PATCH",
      body: JSON.stringify({
        id: selected.value.id,
        version: selected.value.version,
        ...changes,
      }),
    });
    await load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function command(action) {
  const user = selected.value;
  if (action === "role")
    return patch({ role: user.role === "admin" ? "agent" : "admin" });
  if (action === "disable") return patch({ disabled: !user.disabled });
  if (action === "password") {
    try {
      const { value } = await ElMessageBox.prompt(t("新密码（至少 8 位）"));
      await patch({ password: value });
    } catch {
      /* Cancelled. */
    }
  }
  if (action === "revoke") {
    try {
      await api(`admin/users/${encodeURIComponent(user.id)}/revoke`, {
        method: "POST",
      });
      ElMessage.success(t("已撤销"));
      await load();
    } catch (error) {
      ElMessage.error(error.message);
    }
  }
}
watch(() => selected.value?.teamId, async (teamId) => {
  if (!teamId) return;
  try {
    const data = await api(`admin/metrics?period=30&team=${encodeURIComponent(teamId)}`);
    stats.value = data.performance || [];
  } catch (error) { ElMessage.error(error.message); }
});
watch(
  () => route.query.agent,
  (value) => {
    if (value) selectedId.value = value;
  },
);
onMounted(load);
</script>

<style scoped>
.radar { height: 260px; }
.group-name { font-weight: 700; margin: 14px 0 8px; font-size: 15px; }
.team-manage { padding: 18px; margin-bottom: 16px; }
.team-manage .row { margin: 12px 0; gap: 12px; }
.team-manage label { display: flex; align-items: center; gap: 8px; }
.tm {
  display: grid;
  grid-template-columns: 340px 1fr;
  gap: 16px;
  align-items: start;
}
.ul {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.uc {
  display: flex;
  width: 100%;
  gap: 12px;
  align-items: center;
  text-align: left;
  padding: 12px;
  cursor: pointer;
  border-width: 1.5px;
  font: inherit;
  color: inherit;
}
.uc.on {
  border-color: var(--gold);
}
.uc.off {
  opacity: 0.65;
}
.mini {
  text-align: center;
}
.mini b {
  display: block;
  font-size: 20px;
}
.mini span {
  color: var(--mut);
  font-size: 11px;
}
.ud {
  padding: 20px 24px;
}
.uh {
  display: flex;
  align-items: center;
  gap: 16px;
}
.nm {
  font-size: 26px;
  font-weight: 700;
}
.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin: 20px 0;
  background: #f8f7f3;
  border-radius: 12px;
  padding: 14px;
}
.stats b {
  display: block;
  margin-top: 5px;
  font-size: 17px;
}
.section-heading {
  margin: 24px 0 10px;
}
.section-heading h3 {
  margin: 0;
  font-size: 15px;
}
.zone-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 10px;
  border-bottom: 1px solid var(--line);
}
</style>

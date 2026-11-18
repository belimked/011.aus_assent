<template>
  <el-drawer
    :model-value="!!parcel"
    size="480px"
    :with-header="false"
    class="pdrawer"
    @close="$emit('close')"
  >
    <template v-if="parcel">
      <div class="hero">
        <svg class="house" viewBox="0 0 200 120" aria-hidden="true">
          <path d="M40 112V62l60-44 60 44v50z" fill="rgba(255,255,255,.9)" />
          <rect x="88" y="78" width="22" height="34" rx="2" fill="#c9a227" />
          <rect x="52" y="70" width="22" height="18" rx="2" fill="#cfe3f5" />
          <rect x="126" y="70" width="22" height="18" rx="2" fill="#cfe3f5" />
        </svg>
        <div class="hero-top">
          <StatusPill :status="parcelStatus(parcel)" />
          <div class="grow" />
          <el-button
            circle
            size="small"
            :icon="Close"
            @click="$emit('close')"
          />
        </div>
        <div class="hero-bot">
          <div class="addr serif">{{ parcel.address }}</div>
          <div class="sub">
            {{ parcel.suburb }} · {{ parcel.lotplan || "—" }}
          </div>
        </div>
      </div>
      <div class="body">
        <section>
          <h4>{{ t("房产资料") }}</h4>
          <div class="facts">
            <div>
              <span>{{ t("地块") }}</span
              ><b>{{ parcel.lotplan || "—" }}</b>
            </div>
            <div>
              <span>{{ t("已登记") }}</span
              ><StatusPill :status="parcel.registered ? 'yes' : 'no'" />
            </div>
            <div>
              <span>{{ t("异地业主") }}</span
              ><StatusPill :status="parcel.absenteeOwner ? 'yes' : 'no'" />
            </div>
          </div>
        </section>
        <section class="tp">
          <div class="row">
            <el-icon color="#b8860b"><InfoFilled /></el-icon>
            <h4>{{ t("第三方业主信息") }}</h4>
            <div class="grow" />
            <span class="small muted">{{ t("仅管理员 · 不导出") }}</span>
          </div>
          <div class="tpg">
            <div>
              <span class="small muted">{{ t("业主姓名") }}</span
              ><b>{{ parcel.ownerNames || "—" }}</b>
            </div>
            <div>
              <span class="small muted">{{ t("最近成交") }}</span
              ><b>{{ fmtDate(parcel.lastSaleDate) }}</b>
            </div>
            <div>
              <span class="small muted">{{ t("上次成交价") }}</span
              ><b>{{ money(parcel.lastSalePrice) }}</b>
            </div>
            <div>
              <span class="small muted">{{ t("持有年数") }}</span
              ><b>{{
                parcel.yearsHeld == null
                  ? "—"
                  : t("{n} 年", { n: parcel.yearsHeld })
              }}</b>
            </div>
          </div>
        </section>
        <section>
          <h4>{{ t("覆盖经纪人") }}</h4>
          <div v-if="parcel.coveringAgents?.length" class="agents">
            <div
              v-for="agent in parcel.coveringAgents"
              :key="agent.id"
              class="row agent"
            >
              <Avatar :name="agent.name" :size="30" />{{ agent.name }}
            </div>
          </div>
          <div v-else class="small muted">{{ t("暂无覆盖经纪人") }}</div>
        </section>
      </div>
    </template>
  </el-drawer>
</template>

<script setup>
import { Close } from "@element-plus/icons-vue";
import { fmtDate, money, parcelStatus } from "../admin-format";
import { t } from "../i18n";
import Avatar from "./Avatar.vue";
import StatusPill from "./StatusPill.vue";

defineProps({ parcel: Object });
defineEmits(["close"]);
</script>

<style scoped>
:deep(.el-drawer__body) {
  padding: 0;
}
.hero {
  height: 210px;
  position: relative;
  overflow: hidden;
  color: #fff;
  background: linear-gradient(135deg, #365779, #0f2a44);
}
.house {
  position: absolute;
  width: 260px;
  right: -10px;
  bottom: -6px;
  opacity: 0.55;
}
.hero-top {
  position: absolute;
  left: 18px;
  right: 14px;
  top: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.hero-bot {
  position: absolute;
  left: 20px;
  right: 20px;
  bottom: 16px;
  text-shadow: 0 1px 8px #0b2135;
}
.addr {
  font-size: 28px;
  font-weight: 700;
}
.sub {
  font-size: 13px;
  opacity: 0.9;
}
.body {
  padding: 4px 22px 30px;
}
section {
  padding: 20px 0;
  border-bottom: 1px solid var(--line);
}
section:last-child {
  border: 0;
}
h4 {
  font-size: 15px;
  margin: 0 0 14px;
}
.tp {
  background: var(--gold-soft);
  margin: 16px -10px 0;
  padding: 18px 12px;
  border-radius: 10px;
  border: 1px solid #efdfad;
}
.tp h4 {
  margin: 0;
}
.tpg,
.facts {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 16px;
}
.tpg b,
.facts b {
  display: block;
  font-size: 15px;
  margin-top: 4px;
}
.facts span {
  display: block;
  color: var(--mut);
  font-size: 12px;
}
.agents {
  display: grid;
  gap: 10px;
}
.agent {
  padding: 8px 0;
}
</style>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>{{ t(titles[page]) }}</h1>
        <div class="sub">{{ t(descriptions[page]) }}</div>
      </div>
    </div>

    <div v-if="page === 'market'" class="card bar">
      <label class="field"
        >{{ t("市场文件")
        }}<input
          type="file"
          accept=".csv,.xls,.xlsx"
          @change="file = $event.target.files[0]"
      /></label>
      <label class="field"
        >{{ t("导入类型") }}
        <el-select v-model="kind" style="width: 160px">
          <el-option
            v-for="option in importKinds"
            :key="option.key"
            :value="option.key"
            :label="t(option.label)"
          />
        </el-select>
      </label>
      <el-checkbox v-model="dry">{{ t("试运行") }}</el-checkbox>
      <el-button :icon="Upload" type="primary" @click="upload">{{
        t("导入文件")
      }}</el-button>
    </div>

    <div v-if="['review', 'rex', 'data'].includes(page)" class="card bar">
      <label class="field"
        >{{ t(page === "data" ? "数据类型" : "状态") }}
        <el-select v-model="filter" style="width: 200px" @change="load(true)">
          <el-option
            v-for="option in options"
            :key="option.key"
            :value="option.key"
            :label="t(option.label)"
          />
        </el-select>
      </label>
    </div>
    <div v-if="page === 'audit'" class="card bar">
      <label class="field"
        >{{ t("用户 ID") }}<el-input v-model="audit.user"
      /></label>
      <label class="field"
        >{{ t("实体") }}<el-input v-model="audit.entity"
      /></label>
      <label class="field"
        >{{ t("动作") }}<el-input v-model="audit.action"
      /></label>
      <label class="field"
        >{{ t("开始") }}<input v-model="audit.from" type="datetime-local"
      /></label>
      <label class="field"
        >{{ t("结束") }}<input v-model="audit.to" type="datetime-local"
      /></label>
      <el-button @click="load(true)">{{ t("筛选") }}</el-button>
    </div>

    <div class="card">
      <el-table
        :data="items"
        height="calc(100vh - 290px)"
        row-class-name="record-row"
      >
        <el-table-column
          v-for="column in columns[page]"
          :key="column.key"
          :label="t(column.label)"
          :min-width="column.width || 115"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <template v-if="column.key === 'actions'">
              <template v-if="page === 'review' && row.status === 'pending'">
                <el-button size="small" @click="decide(row, 'approve')">{{
                  t("批准")
                }}</el-button>
                <el-button
                  size="small"
                  type="danger"
                  plain
                  @click="decide(row, 'reject')"
                  >{{ t("拒绝") }}</el-button
                >
              </template>
              <el-button
                v-if="page === 'rex' && row.status === 'failed'"
                size="small"
                @click="retry(row)"
                >{{ t("重试") }}</el-button
              >
              <el-button v-if="page === 'market'" link @click="detail(row)">{{
                t("详情")
              }}</el-button>
            </template>
            <StatusPill
              v-else-if="column.key === 'status'"
              :status="row.status"
              short
            />
            <span v-else-if="column.key === 'kind'">{{
              approvalKind(row.kind)
            }}</span>
            <span
              v-else-if="
                column.key === 'at' ||
                column.key === 'created_at' ||
                column.key === 'createdAt' ||
                column.key === 'updated_at'
              "
              >{{
                ["at", "created_at", "createdAt"].includes(column.key)
                  ? fmtDateTime(row[column.key])
                  : fmtDate(row[column.key])
              }}</span
            >
            <span v-else-if="column.key === 'summary'">{{
              row.summary || row.recordId || row.primaryId || "—"
            }}</span>
            <el-button
              v-else-if="
                ['payload', 'diff', 'agentCounts', 'record'].includes(
                  column.key,
                )
              "
              link
              @click="detail(row)"
              >{{ t("查看") }}</el-button
            >
            <span v-else>{{ row[column.key] ?? "—" }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <div v-if="page !== 'market'" class="pager">
      <span class="small muted">{{
        t("本页 {n} 条", { n: items.length })
      }}</span>
      <el-button :disabled="items.length < 50" @click="next">{{
        t("下一页")
      }}</el-button>
    </div>

    <el-drawer v-model="drawer" :title="t('详情')" size="550px">
      <template v-if="page === 'market' && detailData">
        <div class="report">
          <div v-for="field in reportFields" :key="field.key">
            <span class="small muted">{{ t(field.label) }}</span>
            <b>{{ detailData[field.key] ?? 0 }}</b>
          </div>
        </div>
        <h3>{{ t("经纪人通知") }}</h3>
        <div v-for="(count, id) in detailData.agentCounts || {}" :key="id">
          {{ id }} · {{ count }}
        </div>
        <h3>{{ t("错误明细") }}</h3>
        <p v-for="(error, index) in detailData.errorDetails || []" :key="index">
          {{ error }}
        </p>
        <h3>{{ t("未匹配地址") }}</h3>
        <el-table
          :data="detailData.unmatched || detailData.unmatchedAddresses || []"
        >
          <el-table-column prop="address" :label="t('地址')" />
          <el-table-column prop="suburb" :label="t('区域')" />
          <el-table-column prop="postcode" :label="t('邮编')" />
          <el-table-column prop="kind" :label="t('类型')" />
        </el-table>
      </template>
      <pre v-else class="json">{{ JSON.stringify(detailData, null, 2) }}</pre>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";
import { Upload } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { api } from "../api";
import { approvalKind, fmtDate, fmtDateTime } from "../admin-format";
import { t } from "../i18n";
import StatusPill from "../components/StatusPill.vue";

const props = defineProps({ page: String });
const titles = {
  review: "审核队列",
  audit: "审计日志",
  rex: "Rex 队列",
  market: "市场数据",
  data: "数据浏览",
};
const descriptions = {
  review: "重新分配与合并申请",
  audit: "按用户、实体、动作和时间筛选",
  rex: "Rex 投递状态与重试",
  market: "市场文件导入与历史",
  data: "只读业务数据",
};
const columns = {
  review: [
    { key: "requesterName", label: "申请人" },
    { key: "kind", label: "类型" },
    { key: "summary", label: "记录", width: 180 },
    { key: "status", label: "状态" },
    { key: "note", label: "备注" },
    { key: "actions", label: "操作", width: 165 },
  ],
  audit: [
    { key: "at", label: "时间" },
    { key: "user_id", label: "用户 ID" },
    { key: "entity", label: "实体" },
    { key: "action", label: "动作" },
    { key: "device", label: "设备" },
    { key: "diff", label: "差异" },
  ],
  rex: [
    { key: "created_at", label: "时间" },
    { key: "entity", label: "实体" },
    { key: "op", label: "操作" },
    { key: "status", label: "状态" },
    { key: "tries", label: "次数" },
    { key: "payload", label: "内容" },
    { key: "actions", label: "操作" },
  ],
  market: [
    { key: "createdAt", label: "日期" },
    { key: "filename", label: "文件", width: 180 },
    { key: "rows", label: "行数" },
    { key: "success", label: "成功" },
    { key: "errors", label: "错误" },
    { key: "unmatched", label: "未匹配" },
    { key: "agentCounts", label: "经纪人" },
    { key: "actions", label: "操作" },
  ],
  data: [
    { key: "id", label: "ID", width: 220 },
    { key: "updated_at", label: "更新" },
    { key: "record", label: "记录" },
  ],
};
const kinds = [
  "contacts",
  "properties",
  "todos",
  "captures",
  "enquiries",
  "leads",
  "listings",
  "ofi_attendees",
  "sales",
  "contracts",
  "referrals",
  "market_items",
  "zones",
  "tags",
];
const kindLabels = {
  contacts: "联系人",
  properties: "房产",
  todos: "待办",
  captures: "采集记录",
  enquiries: "咨询",
  leads: "线索",
  listings: "房源",
  ofi_attendees: "开放参观登记",
  sales: "成交",
  contracts: "合同",
  referrals: "转介",
  market_items: "市场记录",
  zones: "区域",
  tags: "标签",
};
const options = computed(
  () =>
    ({
      review: ["pending", "approved", "rejected"].map((key) => ({
        key,
        label: { pending: "待审核", approved: "已批准", rejected: "已拒绝" }[
          key
        ],
      })),
      rex: ["all", "waiting_auth", "pending", "sent", "failed", "skipped"].map(
        (key) => ({
          key,
          label: {
            all: "全部",
            waiting_auth: "等待授权",
            pending: "待投递",
            sent: "已发送",
            failed: "失败",
            skipped: "已跳过",
          }[key],
        }),
      ),
      data: kinds.map((key) => ({ key, label: kindLabels[key] })),
    })[props.page] || [],
);
const importKinds = [
  { key: "auto", label: "自动识别" },
  { key: "sale", label: "成交" },
  { key: "listing", label: "房源" },
  { key: "ownership", label: "产权" },
];
const reportFields = [
  { key: "rows", label: "行数" },
  { key: "success", label: "成功" },
  { key: "skipped", label: "已跳过" },
  { key: "errors", label: "错误" },
  { key: "ignored", label: "页脚忽略" },
  { key: "ownershipUpdated", label: "产权更新" },
  { key: "salesRecorded", label: "成交记录" },
  { key: "salesNotified", label: "成交通知" },
];
const filter = ref("");
const items = ref([]);
const cursor = ref("");
const drawer = ref(false);
const detailData = ref(null);
const file = ref(null);
const kind = ref("auto");
const dry = ref(false);
const audit = reactive({ user: "", entity: "", action: "", from: "", to: "" });

async function load(reset = false) {
  if (reset) cursor.value = "";
  const path = {
    review: "admin/approvals",
    audit: "admin/audit",
    rex: "admin/rex-outbox",
    market: "admin/market/imports",
    data: "admin/data",
  }[props.page];
  if (!path) return;
  const params = new URLSearchParams();
  if (props.page === "data") params.set("kind", filter.value || "contacts");
  if (
    ["review", "rex"].includes(props.page) &&
    filter.value &&
    filter.value !== "all"
  ) {
    params.set("status", filter.value);
  }
  if (props.page === "audit") {
    for (const [key, value] of Object.entries(audit)) {
      if (value)
        params.set(
          key,
          ["from", "to"].includes(key) ? new Date(value).toISOString() : value,
        );
    }
  }
  if (cursor.value) params.set("before", cursor.value);
  try {
    items.value = await api(`${path}?${params}`);
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function next() {
  cursor.value = items.value.at(-1).id;
  load();
}
async function decide(row, decision) {
  try {
    const { value } = await ElMessageBox.prompt(t("审核备注"), "", {
      inputValue: "",
    });
    await api(`approvals/${encodeURIComponent(row.id)}/decide`, {
      method: "POST",
      body: JSON.stringify({ decision, note: value }),
    });
    load(true);
  } catch (error) {
    if (error instanceof Error) ElMessage.error(error.message);
  }
}
async function retry(row) {
  try {
    await api(`admin/rex-outbox/${row.id}/retry`, { method: "POST" });
    load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function detail(row) {
  try {
    detailData.value =
      props.page === "market"
        ? await api(`admin/market/imports/${encodeURIComponent(row.id)}`)
        : row;
    drawer.value = true;
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function upload() {
  if (!file.value) return ElMessage.warning(t("请选择市场文件"));
  const body = new FormData();
  body.set("file", file.value);
  body.set("kind", kind.value);
  body.set("dryRun", String(dry.value));
  try {
    detailData.value = await api("admin/market/imports", {
      method: "POST",
      body,
    });
    drawer.value = true;
    load(true);
  } catch (error) {
    ElMessage.error(error.message);
  }
}
function reset() {
  filter.value =
    props.page === "review"
      ? "pending"
      : props.page === "data"
        ? "contacts"
        : "all";
  load(true);
}
watch(() => props.page, reset);
onMounted(reset);
</script>

<style scoped>
.bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: end;
  margin-bottom: 14px;
  padding: 14px;
}
.field {
  display: grid;
  gap: 5px;
  min-width: 145px;
  font-size: 12px;
  color: var(--mut);
}
.field input[type="datetime-local"],
.field input[type="file"] {
  padding: 7px;
  font: inherit;
  border: 1px solid var(--line);
  border-radius: 8px;
}
.pager {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 14px;
}
.report {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}
.report > div {
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
}
.report b {
  display: block;
  margin-top: 5px;
  font-size: 20px;
}
.json {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  user-select: text;
}
</style>

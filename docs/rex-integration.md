# Rex 集成 · 后端设计

> 状态：草案 v0.2（2026-11-17，按沙箱实测和已确认决策修订 D1–D3）
> 依据：`中文版 - 信息收集开发需求.pdf`、`Rex_API_Integration_Guide.docx`、Rex 官方 API 文档（apibrowser 的 `ApiDocs::*` 和 `::describe` 接口）
> 标注「待确认」的内容要在拿到真实 Rex 账号后，用 `describeModel` / `describeSearchFields` / `read` 验证。

---

## 0. 已定决策

| # | 决策 | 说明 |
|---|---|---|
| D1 | **所有 Rex 调用都用集成用户**（2026-11-17 修订，已确认） | Rex 答复每个中介公司只用一个集成用户，集成用户不能代写。写备注、Lead、提醒时：正文末尾署名“via AxleoCollect · <经纪人>”；assignee / remindee / agent_1 填经纪人本人；完整操作人记录存在我们的审计日志里。第一阶段不做每位中介自己的 Rex 授权（`loginWithOAuthAccessToken` 有接口，但没有公开的 OAuth 申请方式；沙箱 200 个用户中 93 个开了 MFA，`login` 没有 MFA 参数）。所有 token 只存在后端，手机端不直接连 Rex。 |
| D2 | **可见范围以我们自己的分配表为准**（2026-11-17 修订） | 集成用户能看到全部数据，所以经纪人能看到哪些联系人 / 房产由 AxleoCollect 的分配表（片区、负责人）决定，后台可调。导入时 Rex 记录上的负责人作为初始分配；负责人已删除的记录按“未分配”处理。 |
| D3 | **按需同步 + webhook + 每晚对账** | 用户打开什么就刷新什么，写入后立即更新本地。Webhook（沙箱实测稳定后 4–8 秒送达）用于及时更新；它可能延迟或丢失，所以每晚全量 e-tag 对账作为兜底。 |
| D4 | **Rex 是唯一数据源** | 本地只存索引、关系和分配数据。联系人资料（姓名、电话、邮箱、笔记正文）只做加密的短期缓存，有效期不超过 24 小时，不长期保存副本（需求文档「只存一处」）。 |

---

## 1. 总体架构

```
 手机 Web App / Web 工作台
          │  (M365 SSO 登录，只调用我们的 API)
          ▼
 ┌──────────────── 门户后端（澳洲区域）────────────────┐
 │  API 层 ── 可见范围过滤（Visibility）── 审计          │
 │     │                                                │
 │  同步服务（按需刷新 / 夜间对账 / webhook 接收）       │
 │     │                                                │
 │  写入服务（录入任务、去重、幂等、分步提交）           │
 │     │                                                │
 │  Rex 连接器 ── 限流队列（令牌桶 + 优先级 + 合并）     │──► Rex API
 │     │                                                │
 │  数据库：PostgreSQL + PostGIS                        │
 └──────────────────────────────────────────────────────┘
```

- **Rex 连接器是唯一调用 Rex 的模块**，其他模块都通过它访问 Rex。
- 可见范围过滤放在 API 层出口，所有返回给前端的数据都要经过它。

---

## 2. Rex 连接器

### 2.1 请求规范

- 所有调用都是 `POST https://api.rexsoftware.com/v1/rex/<Service>/<method>`，body 是 JSON。测试和正式环境用同一个 URL。
- 固定请求头：
  ```
  Authorization: Bearer <token>
  Content-Type: application/json
  X-App-Identifier: Integration:<公司名>:RexCapture     # 公司名待定；每个请求都必须带
  X-Api-Option: use_status_codes=true,use_strict_arguments=true
  ```
  通过 Verified Integrator 审批后，再加上 Rex 下发的额外请求头（做成配置项）。
- 响应格式是 `{ result, error }`。只要 `error` 不为 null，就当作失败处理。

### 2.2 认证与 token

- 调用 `Authentication::login({ email, password, account_id?, token_lifetime })` 登录。
  - `token_lifetime` 设为 86400（1 天）。官方建议非交互式集成不要跨多天使用同一个 token。
  - 大约每 30 次请求，Rex 会自动续期一次 token。
- 集成用户的密码放在密钥管理服务里（或用加密的环境变量）。token 加密存在数据库里，多个实例共用。
- 第一阶段不存每位中介的 Rex token（见 D1）。限额按中介公司账户计（3,600 次/小时，Rex 文档 Rate Limits），每人授权也不会增加额度。
- 遇到 `TokenException` 时重新登录，**用分布式锁保证只有一个实例去登录**，其他请求等待新 token 后重试一次。
- `loginWithIntegrationAccessToken` 目前没有文档说明，成为 Verified Integrator 后再向 Rex 确认能否改用它。

### 2.3 限流（每个账号每小时 3,600 次）

- **令牌桶**：每秒补充 1 个令牌，桶容量 60，允许短时突发。状态存在数据库或 Redis 里，多个实例共享。
- **优先级**：

  | 级别 | 用途 | 规则 |
  |---|---|---|
  | P0 | 写入（录入、关联、免打扰、新建地址） | 始终优先 |
  | P1 | 交互式刷新（用户正在看的详情或区域） | 正常排队 |
  | P2 | 后台任务（夜间对账、初始导入、批量刷新） | 只有最近 60 分钟用量低于 70% 时才执行 |

- **合并请求**：同一个 `(service, method, key)` 已经在执行或排队时，新请求直接复用那个结果。
- **批量读取**：需要读多条记录时，不要逐条调 `read`，而是用 `search` 加 id 列表条件，每页 100 条，一次请求拿回来。具体条件写法待确认。`BatchRequests` 服务是否只算一次请求，也待确认。
- **用量监控**：每次调用都记到 `rex_request_log`。滚动 1 小时用量超过 80% 时告警。
- **失败重试**：网络错误或 5xx 用指数退避重试，最多 3 次。4xx 或业务错误不重试，直接返回给调用方。

### 2.4 本项目会用到的 Rex 方法

| 用途 | 方法 |
|---|---|
| 登录 | `Authentication::login` |
| 房产 | `Properties::search`（结果格式 `default` / `etags` / `ids` / `map`）、`read`、`create`、`update`、`findPossibleDuplicate`、`setGeoLocation` |
| 联系人 | `Contacts::search`（结果格式包括 `etags` / `name_etags`）、`read`、`create`、`update`、`findPossibleDuplicates` |
| 合并 | `Dedupe::findPossibleDuplicates`、`combineRecords`（需求规定不在 Rex 中自动合并，只允许人工确认后调用） |
| 笔记 | `Notes::create({ data, return_id })`、`search` |
| 提醒 | `Reminders::create` |
| 标签 | `AdminTags::search`（读取标签列表）。给记录打标签的方式待确认 |
| 用户 | `AccountUsers`（读取 Rex 用户，用于初始导入的映射） |
| Webhook | `AdminWebhooks::create` / `getEventsAndCategories` / `markAsHealthy` |
| 关系类型 | 值列表 `contact_reln_listing` / `contact_listing_reln_type_with_custom`；房产的关系类型对应哪个列表，待确认 |

---

## 3. 数据模型

> 原则：**存 id、关系、状态；不长期存个人信息。** 个人信息只放在 `rex_cache` 里，加密存储，24 小时内过期。

### 3.1 组织与用户（来自 M365 SSO）

```sql
teams(id, name, leader_user_id, created_at)
users(id, m365_oid, name, email, role,        -- role: admin | leader | agent | assistant
      team_id, status, created_at)
rex_users(rex_user_id PK, name, email,
          mapped_user_id NULL → users.id)       -- 只在初始导入和展示 Rex 归属时使用
```

### 3.2 分配表（可见范围的依据）

```sql
-- 记录级分配：一条 Rex 记录归属哪个团队或经纪人
assignments(
  id,
  record_type   TEXT,          -- 'property' | 'contact'
  rex_id        BIGINT,
  team_id       NULL → teams.id,
  agent_id      NULL → users.id,   -- 为空表示整个团队共享
  source        TEXT,          -- 'rex_import' | 'manual' | 'capture' | 'area_rule'
  note          TEXT,
  created_by, created_at, updated_by, updated_at,
  UNIQUE(record_type, rex_id)
)

-- 区域级分配（可选，第二阶段的街道分配也用这张表）
area_assignments(
  id, team_id, agent_id NULL,
  kind TEXT,                   -- 'street' | 'suburb' | 'polygon'
  street TEXT, suburb TEXT, geom GEOMETRY NULL,
  priority INT, created_by, created_at
)

assignment_history(id, record_type, rex_id, before JSONB, after JSONB, actor_user_id, at)
```

### 3.3 Rex 索引（非个人信息，长期保存）

```sql
rex_properties(
  rex_id PK, unit, street_number, street_name, suburb, postcode, state,
  addr_norm TEXT,              -- 统一格式后的地址，用于匹配
  geom POINT,                  -- 优先用 Rex 的坐标；没有时用 G-NAF 补上
  etag, system_modtime, record_state,   -- active / archived / trashed
  synced_at, stale_after
)

rex_property_contacts(        -- 房产 ↔ 联系人的关系，用来判断颜色
  property_rex_id, contact_rex_id, reln_type,
  is_owner_type BOOL,          -- 根据 settings.owner_reln_types 计算
  synced_at,
  PRIMARY KEY(property_rex_id, contact_rex_id, reln_type)
)

rex_contacts(                  -- 联系人只存 id 和状态，不存姓名或电话
  rex_id PK, etag, system_modtime, record_state,
  rex_owner_user_id NULL,      -- Rex 里的负责人，仅供参考；具体字段名待确认
  dnc BOOL, unsubscribed BOOL, -- 免打扰 / 退订，用于禁止加入邮件列表
  synced_at, stale_after
)
```

### 3.4 短期缓存（个人信息，加密存储，最长 24 小时）

```sql
rex_cache(
  key TEXT PK,                 -- 例如 'contact:123'、'property_notes:456'
  payload BYTEA,               -- 用应用层 AES-GCM 加密，密钥放在 KMS
  fetched_at, expires_at       -- expires_at 不超过 fetched_at + 24h
)
```
定时任务每小时清理一次过期数据。缓存过期后，下次访问时重新从 Rex 读取。

### 3.5 地址匹配（公开数据 ↔ Rex）

```sql
addresses(id, gnaf_pid, unit, number, street, suburb, postcode, addr_norm, geom, parcel_id)
parcels(id, lot_plan, geom)                     -- 来自 DCDB
address_links(
  address_id, rex_property_id NULL,
  method TEXT,                 -- 'exact' | 'fuzzy' | 'manual' | 'created'
  score NUMERIC,
  status TEXT,                 -- 'auto' | 'pending' | 'confirmed' | 'rejected'
  reason TEXT, reviewed_by, reviewed_at
)
```
`pending` 状态的记录就是原型里的「管理员审核队列」。

### 3.6 写入、审计、运维

```sql
write_jobs(
  id UUID PK,                  -- 由客户端生成，作为幂等键（离线补发时用同一个 id）
  user_id, kind,               -- 'capture' | 'link_owner' | 'flag' | 'new_address' | 'ocr_batch'
  input BYTEA,                 -- 加密存储；任务完成 30 天后清除
  plan JSONB,                  -- 计划执行的 Rex 操作列表
  steps JSONB,                 -- 每一步的状态，以及产生的 rex_id
  status TEXT,                 -- 'draft' | 'queued' | 'running' | 'done' | 'failed' | 'needs_review'
  error TEXT, created_at, updated_at
)

audit_log(
  id, at, actor_user_id,       -- 实际操作人（交互写入在 Rex 里也显示为本人；后台任务显示为集成用户）
  action, target_type, target_rex_id,
  detail JSONB, write_job_id NULL, ip, device
)

rex_request_log(id, at, service, method, priority, http_status, ok, duration_ms, caller)
sync_state(resource PK, last_full_at, last_etag_check_at, cursor JSONB)
settings(key PK, value JSONB)  -- owner_reln_types、TTL、默认可见规则等
webhook_events(id PK, type, received_at, processed_at, payload_ref)   -- 可选
```

---

## 4. 可见范围（Visibility）

### 4.1 归属怎么确定

确定一条记录属于谁时，按以下顺序查找，用第一个找到的结果：

1. `assignments` 里这条记录自己的分配；
2. 如果是**联系人**且没有直接分配：看它关联的房产，取这些房产的归属（有多个时取并集）；
3. 如果是**房产**：看 `area_assignments` 里匹配的街道、区域或多边形，按 `priority` 取最高的；
4. 都没有：视为**未分配**。

### 4.2 谁能看到

设查看者为 `u`，记录的归属为 `(team, agent)`：

| 条件 | 能否看到 |
|---|---|
| `u.role = admin` | 全部可见 |
| `agent = u` | 可见 |
| `agent` 为空，且 `team = u.team` | 可见（团队共享） |
| `agent` 不是 `u`，但 `team = u.team` | 可见。页面显示「归属 Natalie，Team Owen」，录入时归属保持不变，作者记为 `u` |
| `team` 不是 `u.team` | **房产**：在地图上显示为「其他团队跟进」，不显示联系人。**联系人**：不显示；如果录入时匹配到这类联系人，提示管理员 |
| 未分配 | 由配置项 `settings.unassigned_visibility` 决定，默认 `open`（所有人可见）。可以改成 `closed`（只有管理员可见） |

> 规则集中写在一个 `Visibility` 模块里，用纯函数加 SQL 片段实现，所有 API 都调用它，不要在各处自己判断。

### 4.3 地图颜色（每个人看到的可能不同）

对每处房产、每个查看者分别计算：

- **灰色**：`address_links` 里没有已确认或自动关联的 Rex 房产；
- **紫色（其他团队跟进）**：房产归属其他团队，且查看者不是管理员；
- **绿色**：至少有一条 `rex_property_contacts.is_owner_type = true` 的关系，并且对应的联系人查看者能看到；
- **红色**：以上都不满足。

这个计算只用到本地的索引表和分配表，**不调用 Rex**，所以地图和覆盖率统计都不会消耗限流额度。哪些关系类型算「业主」，由 Ping 确认后写入 `settings.owner_reln_types`。

### 4.4 初始导入（`source = rex_import`）

当前 SQLite 服务实现：`axleo rex-import --account 2057 [--plan sandbox-test] [--dry-run] [--limit N]`。默认 `sandbox-test`；旧轮流方式用 `--plan legacy` 或 `--assign mary@axleo.local,john@axleo.local`。必须 `AXLEO_REX_READ=1` 且 `REX_ACCOUNT_ID=2057`；命令强制 ReadOnly，不调用 Rex 写接口。`--limit` 是每种实体各 N 条，0 表示全量。dry-run 仍写调用日志但不写业务表。只读分页每页 100，按 id 排序；房产请求 `extra_options.extra_fields=[contact_reln_property]`。`rex_links` 以 `(entity,account_id,rex_id)` 标识；etag 不变跳过，变更更新源字段和关系，不改已有负责人（包括联系人 `data.owner`）。不写 outbox。

方案需预先创建启用的 `mary@axleo.local`、`john@axleo.local`、`import@axleo.local`（仅用于测试大数据同步耗时）、`admin@axleo.local`；缺任何账号会在 Rex 登录前报错，导入命令不会创建账号。只对未删除的 `source=rex_import` 排序：有 `property_owners` 关系优先，其次以 Sunnybank (-27.5794, 153.0606) 的平面距离平方（经度乘 0.887，缺坐标排最后），再按 id。前 400 个房产按 Mary/John 奇偶交替各 200，接着 1,734 个给 import，其余给 admin，绝不留空。联系人按所关联的最靠前房产负责人：Mary、John 各先取关联自己房产的 200 个（按房产排名），不足从无关联联系人按 id 补足；import 先取剩余有关联的联系人，再取无关联的，最多 3,938 个；其余给 admin。同步更新 `contacts.owner_id` 与 `data.owner`。不重置重跑只给新记录按剩余容量分配，已有负责人不动；分配发生变化会吊销这四个账号的 sessions，手机须重新登录全量同步。`properties.owner_id` 仅对导入数据设置，演示房产仍共享。详细运行和删除步骤见 `docs/ops-runbook.md`。

**与目标架构的差距**：当前服务仍存完整 `data.rex`，不是 §3 所述短期加密缓存；没有夜间对账、P2 跨实例限流，也没有按 Rex 负责人映射（沙箱按固定测试方案分配）。重新分配以本地 owner_id 为准。`/api/v1/sync` 当前未分页，首次登录每人约 4,000 联系人 + 1,700 房产，且逐行查询；手机响应已移除 `data.rex` 和房产 `data.geometry`，支持 gzip JSON，但不降低逐行读取和解压后的内存压力。分页另议；上线前须用真实数据压测。

### 4.5 和中介要书面确认的事

- 集成用户需要能读取全部房产和联系人的权限。
- 中介在 Rex 里原有的权限规则**不会自动生效**，由门户里的分配表代替，并由管理员维护。

---

## 5. 同步策略（按需）

### 5.1 新鲜度

| 数据 | 过期时间（TTL） | 刷新时机 |
|---|---|---|
| 房产详情（地址、关系） | 5 分钟 | 用户打开详情时 |
| 联系人资料、最近 3 条笔记 | 5 分钟（缓存最长保留 24 小时） | 用户打开详情时 |
| 区域 e-tag 检查 | 每个区域 2 分钟 | 用户查看地图视野时 |
| 全量对账 | 24 小时 | 每晚 02:00（布里斯班时间） |

TTL 做成 `settings` 里的配置项，可以调整。

### 5.2 打开房产详情

```
GET /api/properties/:id
 1. 从本地读取，经过 Visibility 过滤后立即返回（附带 synced_at）
 2. 如果已超过 stale_after → 提交一个 P1 刷新任务（相同记录会合并）：
      Properties::read → 更新 rex_properties / rex_property_contacts
      读取可见联系人和最近 3 条笔记 → 写入 rex_cache
 3. 刷新后数据有变化 → 通过 SSE / WebSocket 推送 property.updated，前端局部更新
```

### 5.3 查看地图区域

```
GET /api/map/properties?bbox=...
 1. 用 PostGIS 查出视野内的地址，计算每个点的颜色（§4.3），立即返回
 2. 找出视野覆盖的区域（suburb / postcode）；对超过 2 分钟没检查过的区域：
      P1：Properties::search({ criteria: [该区域], result_format: 'etags', limit: 10000 })
      和本地 etag 比较：
        - 新增或 etag 变了 → 按 id 批量拉取（每页 100 条），更新索引
        - 本地有、远端没有 → 标记为 archived 或 trashed
 3. 有变化时推送 map.updated（只推送变化的点）
```
区域的搜索条件怎么写（字段名），要用 `Properties::describeSearchFields` 确认。

### 5.4 写入后更新

写入成功后，用 Rex 返回的记录或马上再读一次，直接更新本地索引，**不用等下次同步**。录入的经纪人能立刻看到圆点变绿（需求是几秒内）。

### 5.5 夜间对账（兜底）

- 用 P2 优先级，对 `Properties` 和 `Contacts` 全量 `search(result_format: 'etags')`，和本地比较后只拉取有变化的记录；
- 按新的 `owner_reln_types` 重新计算 `is_owner_type`；
- 重算覆盖率统计（按区域、街道、团队）；
- 清理过期的 `rex_cache`；把超过 24 小时没刷新的索引记录标记出来，下次访问时优先刷新。

### 5.6 Webhook（可选）

- 需求文档的目标是「Rex 记录变化后 1 分钟内更新地图」。**办公室同事直接在 Rex 里改的数据**，按需同步只有在有人打开那个区域时才会发现。如果这个 1 分钟是硬性要求，就要启用 webhook。
- 做法：用 `AdminWebhooks::create` 订阅 properties 和 contacts 的 created/updated 事件，`send_format_id` 用 `v1_context_only`（只发 id）。
- 接收端 `POST /api/rex/webhook` 收到后**马上返回 202**（Rex 要求 10 秒内响应），然后把事件放进 P1 刷新队列。
- 每天检查一次 webhook 状态，被标记为不健康时自动调用 `markAsHealthy` 并告警。停用期间漏掉的事件由夜间对账补上。

---

## 6. 写入流程

### 6.1 录入任务（语音、开放参观登记、拍照，三个渠道共用）

```
POST /api/captures            { id: <客户端 UUID>, kind, input }   → status=draft
  1. AI 结构化（由语音/OCR 模块完成）
  2. 去重：先用 AU 标准手机号（+61/61/0061 → 0）匹配本地已链接联系人；沙箱资料显示 Rex `findPossibleDuplicates` 对国际号不等价，若查不到只能在明确核实之后新建；原号仍保留显示。
     将结果按 Visibility 分为三类：
       自己的   → 写入已有联系人（“已匹配已有联系人”，可以撤销）
       团队的   → 写入已有联系人，归属不变，显示归属人
       范围外的 → 不显示，任务标记 needs_review 并通知管理员
  3. 生成 plan（计划执行的 Rex 操作列表），返回给前端确认卡片
POST /api/captures/:id/commit → status=queued，以 P0 优先级按顺序执行：
     Contacts::create / update
     → 关联房产（Properties::update 的 related 行，具体键名待确认）
     → Notes::create（正文末尾加“— 由 <经纪人> 通过 RexCapture 录入”）
     → Reminders::create（跟进提醒）
     → 打标签 / 加入邮件列表（有免打扰或已退订的跳过）
     → 读回结果，更新本地索引；新建的联系人写入 assignments（source=capture，归属录入人）
GET  /api/captures/:id        → 进度，以及每一步的结果
```

### 6.2 幂等与失败恢复

- `write_jobs.id` 由客户端生成。离线时先存在手机上，恢复网络后用**同一个 id** 补发，后端看到已存在就直接返回原任务。
- 每一步执行成功后，把产生的 `rex_id` 写入 `steps`。重试时跳过已完成的步骤，**避免重复创建联系人**。
- 某一步失败时，任务状态变为 `failed`，前端提示，可以重试。已经写入 Rex 的部分**不自动回滚**，只在审计里记录，由人工处理。
- 原型里的「撤销」= 执行反向操作（删除关系、删除笔记等），作为一个新的写入任务执行，也要记审计。

### 6.3 其他写入

| 操作 | Rex 调用 | 本地 |
|---|---|---|
| 关联业主（红 → 绿） | `Contacts::create` + 关联房产 | 关系表、分配表、审计 |
| 免打扰 / 不敲门 / 不投信 | 联系人营销退订字段，或房产上的标签/自定义字段（由 Ping 确认） | `rex_contacts.dnc`、审计 |
| 门口发现的新地址 | `Properties::findPossibleDuplicate` → `create` | `address_links.method=created`，进入审核 |
| 确认地址匹配 | 不调用 Rex | `address_links.status=confirmed` |
| 合并重复联系人 | `Dedupe::combineRecords`，只有管理员能手动执行 | 审计 |

#### 合并联系人后的 Rex 处理（沙箱 2057 实测，2026-11-26）

请求：`Dedupe::combineRecords {service_name:"Contacts", winning_id:A, losing_ids:[B]}`。

实测结果：

- 返回 `{"result":[{"losing_id":"B","records_reassigned":[{"service_name":"Notes","record_ids":[…]}]}]}`。同步完成，B 立即变为 `trashed`。
- **A 的姓名被改成两个名字的拼接**（"AXLEO MERGE TEST A & AXLEO MERGE TEST B"）。
- B 的电话、邮箱、标签全部追加到 A，Rex 不去重；B 的电话 `phone_primary=null`。
- B 的备注转到 A。
- **同一房产上的关系冲突**：合并前 23138536 上 A=owner（行 17077130）、B=tenant（行 17077142）。合并后只剩行 17077130，contact=A，**reln=tenant**。即 Rex 保留赢家的行 id，但关系类型取输家的；B 的行消失。
- 不同房产上 B 的关系行如何转给 A（新 id 还是原 id）未测。worker 不做假设，一律读回 A 的实际行再对齐。
- `Properties::update` 带 `contact_reln_property:[{id:17077130,_destroy:true}]` 只删除该行，其他行（Teddie Bear owner 17016203）不受影响。

因此 worker（`server/rexmerge_worker.go`）在 combine 之后**必须读回 A 并改写为本地合并结果**：

- 姓名：覆盖第一行 `contact_names`，其余行 `_destroy`。
- 电话按 `normAUMobile`、邮箱按小写去重，重复行 `_destroy`。主号码/主邮箱设为本地合并后的值；其他号码保留。
- 标签按忽略大小写去重，单独发一个请求；Rex 拒绝时跳过，并记入 `rex_sync_events`。
- 房产关系：对本地合并结果中的每个房产读 `contact_reln_property`，只处理 contact=A 的行：
  - 类型对 → 不动。
  - 类型错 → 同一行 id 改 `reln_type`。Rex 拒绝或忽略时，改为 `_destroy` 该行再新增。
  - 没有行 → 新增。
  - 多行 → 保留一行，其余 `_destroy`。
  - 绝不触碰其他联系人的行。
- 合并日志备注写到 A。B 已在 Rex 的备注由 combine 自动转到 A，不会再次创建。

细节见 docs/contact-merge.md。

---

## 7. 对前端的 API（初稿）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/me` | 当前用户、角色、团队 |
| GET | `/api/map/properties?bbox&filters` | 视野内的点和颜色（按查看者计算） |
| GET | `/api/map/coverage?area` | 区域和街道的覆盖率 |
| GET | `/api/properties/:id` | 详情：可见的联系人、最近 3 条笔记、标记、归属 |
| GET | `/api/search?q` | 地址、联系人搜索（结果经过 Visibility 过滤） |
| POST | `/api/captures` / `:id/commit` / GET `:id` | 录入任务 |
| POST | `/api/properties/:id/flags` | 免打扰等标记 |
| GET | `/api/events` (SSE) | `property.updated` / `map.updated` / `capture.progress` |
| — | `/api/admin/assignments` | 分配增删改查，可按街道或区域批量设置，导入差异列表 |
| — | `/api/admin/address-review` | 地址匹配审核队列 |
| — | `/api/admin/users` / `teams` / `rex-users` | 用户、团队和 Rex 用户映射 |
| POST | `/api/admin/export` | 只有管理员能用；只导出 Rex 字段；每次导出都记审计 |
| GET | `/api/admin/sync` | 同步状态、限流用量、webhook 状态 |
| GET | `/api/audit` | 审计日志 |
| POST | `/api/rex/webhook` | Rex 回调（可选） |

---

## 8. 审计

- **所有 Rex 写入**、导出、分配变更、权限变更、地址审核都要写入 `audit_log`，长期保存。
- 交互写入以中介本人身份调用，Rex 里看到的操作人就是本人；只有后台任务（对账、同步）显示为集成用户。审计日志仍完整记录每次操作。
- 读取个人信息的操作（打开联系人详情）也记一条轻量的审计记录，为 Privacy Act 合规做准备。

---

## 9. 安全与合规

- Rex 凭证和 token 加密存储；`rex_cache` 和 `write_jobs.input` 用应用层加密，密钥放在 KMS。
- 缓存的 Rex 数据**最长 24 小时**就要刷新或删除（据第三方整理的 Rex API 条款，需向 Rex 直接确认）。
- 第三方业主数据（RP Data / Pricefinder）不能导出，只做短期缓存，和 Rex 数据分开存放。
- 服务器和数据库都放在澳洲区域。

---

## 10. 限流额度估算（假设，需要按实际人数调整）

| 来源 | 估算 | 每小时请求数 |
|---|---|---|
| 录入：每次约 6 次调用（去重、联系人、关联、笔记、提醒、读回） | 10 位经纪人 × 每小时 8 次录入 | ≈ 480 |
| 打开详情刷新（扣除命中 TTL 的） | 10 人 × 每小时 20 次 × 60% 过期 × 2 次调用 | ≈ 240 |
| 地图区域 e-tag 检查，加上变更的批量拉取 | 每小时 30 个区域 × 2 次 | ≈ 60 |
| 合计（白天高峰） | | **≈ 800，约占额度的 22%** |
| 夜间对账 | 全量 etags 只要几十次请求，拉取变更的记录 ÷ 100 | 夜间单独运行，不和白天抢额度 |

初始导入如果要拉几万条记录，要用 P2 优先级、按 100 条一页分几个晚上跑完。

---

## 11. 待确认事项

| # | 事项 | 找谁 / 怎么确认 |
|---|---|---|
| Q1 | `X-App-Identifier` 里写的公司名 | 业务方 |
| Q2 | 哪些关系类型算「业主」（`owner_reln_types`） | Ping |
| Q3 | 未分配记录默认 `open` 还是 `closed` | Ping |
| Q4 | 初始导入用 Rex 的哪个负责人字段（房产或联系人） | 在真实账号上 `read` 一条记录 |
| Q5 | 联系人 ↔ 房产关联在 `update` 的 `related` 里用什么键名 | 真实账号 |
| Q6 | 按区域和按 id 列表搜索的条件写法；`system_modtime` 能否作为搜索条件 | `describeSearchFields` |
| Q7 | 免打扰、不敲门用联系人退订字段、标签还是自定义字段 | Ping 和真实账号 |
| Q8 | `BatchRequests` 是否只算一次请求 | 问 Rex API 团队 |
| Q9 | 「1 分钟内更新」是不是硬性要求（决定 webhook 要不要放进第一阶段） | 业务方 |
| Q10 | 24 小时缓存刷新的条款原文 | 问 Rex API 团队 |
| Q11 | Verified Integrator 获批前占用的付费席位由谁承担 | 业务方和中介 |

---

## 12. 参考

- Rex API 文档：<https://apibrowser.rexsoftware.com/>（文章：Introduction、Authentication、Standards、Rate Limits、Integrations、E-tags Sync、Webhooks）
- 认证：<https://api-docs.rexsoftware.com/article/authentication>
- 测试账号 / Verified Integrator 申请：<https://www.rexsoftware.com/contact/crm-api-enquiry>
- 本地拉取的原始定义：`.tmp/rexdocs/`（`svc_*.json`、`model_*.json`、`art_*.json`）

### 沙箱双向增量同步
读取开关 `AXLEO_REX_READ=1` 启用每 60 秒（`REX_POLL_SECONDS`，最低 30）搜索 Contacts/Properties `result_format=etags`（每页 10000）；变化后按 id 读取，复用导入映射及剩余容量分配；同一记录有未发送 outbox 时跳过，完整搜索后可软删消失的记录（`REX_POLL_SOFT_DELETE=0` 禁用）。写入须 `AXLEO_REX_WRITE=1`、`REX_WRITE_ACCOUNT_ID=2057` 且账户验证通过；`REX_WRITE_ALLOWED_ACCOUNTS` 默认为 2057，758 另需 `REX_ALLOW_PRODUCTION_WRITE=1`，当前不得启用。写入仅限 `source=rex_import` 且已有 rex_links 的联系人；其他 outbox 跳过，不向 Rex 新建数据。联系人更新发送 `Contacts::update`，`data.id` 为已有 Rex ID，`data.related.contact_names/contact_phones/contact_emails` 含已导入的子行 id（如有），以及渠道偏好字段；DND 单独更新。更新成功后读取最新 etag。沙箱已确认 `POST /v1/rex/AccountSettings/read` 请求 `{}` 返回 `result.id=2057`（数字）、`name`、`is_test_account=true`；每次登录/换 token 后重新验证。非测试账户还须 `REX_ALLOW_PRODUCTION_WRITE=1`；验证失败写操作 fail closed，不影响只读。首次实际 Contacts::update 写入结构仍须在沙箱验证。`rex_request_log` 提供实体 ID 筛选；`rex_sync_events` 对应 `/api/v1/admin/rex/events`，方向为 `app_to_rex` / `rex_to_app`。

### UAT2 owner synchronisation
Poll imports changed properties and contacts before saving `contact_reln_property`; missing contact references are read directly. Rex property etags change on owner-relation-only edits (verified in `rex_sync_events`: property 23138536 changed etag within two minutes). As a safety net, `REX_RELATION_REFRESH_SECONDS` (default 86400, minimum 60) re-reads up to `REX_RELATION_REFRESH_LIMIT` (default 100, positive integer) properties updated in the last seven days per run. This is a bounded fallback, not a full historical repair. New polled records use `REX_USER_MAP` (`Rex id or email:local email`, comma-separated), automatic identical email or local rex_user_id, then `REX_DEFAULT_OWNER_EMAIL`, then the sandbox ranking. New owner contacts inherit the property agent; existing assignments remain unchanged.

App-created contacts require an enabled local owner and the existing `AXLEO_REX_WRITE=1`/`REX_WRITE_ACCOUNT_ID` account verification. Outbox POST creates Contacts with related name/phone/email, records the account-scoped link, then queued property owner relations write `Properties::update`. Before create, linked local AU-normalised phones are checked. Ambiguous create results fail closed for manual reconciliation; they are **not** retried blindly. Rejected writes retain `last_error`; transient login/rate/update failures use exponential retries. Properties with no Rex link are skipped as "not in Rex" (product decision 2026-11-25: the contact is still created in Rex, but the app never auto-creates the property in Rex; the owner link stays local and the property shows "Not in Rex yet"). Latest outbox state is delivered as `rexSync`; clients can pull to refresh. Voice DK transcription/extraction audit is stored internally, not written to Rex.

## 房产备注 → Rex Notes

App 房产详情里的备注（`properties.data.notes[]`）在房产已关联 Rex 时，每条写成一条挂在 Rex 房产上的 Note。只同步**新增**，备注修改、删除不同步，也不从 Rex 拉回备注。

**入队**：房产 POST/PATCH（含 `/sync/push`，它同样走 `mutate`）时，用修改前的 `data.notes` 和新 `data.notes` 对比，每条新增备注入队 `rex_outbox(entity='rex_property_note', entity_id='<propertyId>:<noteKey>', op='POST', payload={propertyId,noteKey,t,a,d,at,actorId})`。`noteKey` 取 App 生成的备注 `id`（`pn…`，≤64 字符）；没有 id 的旧备注用 `"h"+sha1(at 或 d ␟ a ␟ t)`。同一 entity_id 已有 outbox 记录就不再入队（重复 PATCH、删除后恢复、同步重放都只会有一条任务）。入队时不判断 Rex 关联，由 worker 判断。旧备注在 `mutate` 里和 `previousOwners` 一起从已读出的记录中复制，不另外查库（`MaxOpenConns(1)`，见 `TestPropertyNoteSingleConnection`）。

**worker**（`rexworker.go`，`rex_property_note`）：
- 不按联系人校验，只看房产：`rex_links(properties, propertyId, 当前账户)` 不存在 → `skipped "not in Rex"`，不调用 Rex。
- `rex_links(rex_property_note, '<propertyId>:<noteKey>')` 已存在 → 直接 `sent`（幂等）。
- 否则 `Notes::create`，成功后写 `rex_links(entity='rex_property_note', local_id='<propertyId>:<noteKey>', rex_id=<note id>, rex_service='Notes')`，并给房产 version+1，让手机拉到新状态。
- 响应丢失（无 error envelope 的超时/5xx）时任务置 `failed "Rex write outcome unknown; reconcile before retry"`，**不盲目重放**；Rex 明确拒绝时记录 `Rex rejected: …`。
- 仍受 `AXLEO_REX_WRITE=1`、`REX_WRITE_ACCOUNT_ID`、测试账户验证约束。

**已在沙箱 2057 验证的请求**（2026-11-25）：

```json
// Notes::create（POST /v1/rex/notes/create）
{"data":{"note":"<正文>\n— via AxleoCollect · <经纪人>","note_type":{"id":"note"},
  "related":{"note_properties":[{"property":{"id":23138536}}]}},"return_id":true}
```

- `result` 为 note id（如 141116060）。只挂房产、不挂联系人可行。
- `Notes::read` 返回 `related.note_properties=[{"id":"39194753","property":"23138536"}]`；`Notes::search` criteria `[{"name":"related_property.id","value":23138536}]` 能查到。
- `Notes::update {"data":{"id":N,"note":"…"}}` 只改正文、不丢房产关联；`Notes::trash {"id":N}` 删除（本功能目前都不调用）。
- 平铺写法 `note_properties=[{"property_id":N}]` 也可用，代码统一用嵌套写法 `{property:{id}}`。

**作者与时间限制**：Rex 的 `created_by` 总是集成用户（沙箱为 80559），`system_ctime` 是 Rex 服务器时间、不可写。因此正文末尾加签名 `\n— via AxleoCollect · <经纪人>`（经纪人取写入备注的登录用户名，找不到时用备注上的 `a`）；备注日期 `d` 不是发送当天（Brisbane）时，正文前加 `[YYYY-MM-DD] `。

**App 状态**：`/sync` 拉取的房产带 `noteRexSync: {<noteKey>: {status, reason, at}}`（服务端计算，写入时丢弃），房产备注卡片用 `RexSyncLabel` 显示 Pending Rex / In Rex / Rex failed / Rex skipped；无 id 的旧备注不显示状态。

## OFI 只读导入（UAT2，UAT3 更新）

`AXLEO_REX_READ=1` 且 Rex 凭证完整时，房产/联系人 poll 后每账户至少间隔 5 分钟读取 `Listings::search` 和 `ListingEvents::search`（分页、无过滤条件；沙箱尚未确认日期和类型的 search criteria），只保留 `event_type` 精确为 `open_home`（兼容旧值 `ofi`）的事件。`listing.id` 关联 Listings；Listings `property.id` / `property_id` 经 `rex_links(properties,account_id,rex_id)` 找房产，缺失时 `Properties::read` 后走现有房产导入。挂牌 owner/team 跟房产 owner/team；未分配房产由导入默认 admin 持有。所有挂牌继续沿用当前 listings 全团队可见规则。按每场 `event_timezone` 的当地日历日保留昨天到未来 90 天（UAT3 前为 14 天；可设 `REX_OFI_PAST_DAYS` / `REX_OFI_FUTURE_DAYS`，0–365；缺时区回退 Brisbane），显示事件当地时间。两个 search 不带日期条件，窗口只影响入库和下发的场次数，不影响 Rex 调用次数（每 5 分钟仍是 Listings + ListingEvents 各 ⌈N/100⌉ 页，加上缺房产时的 `Properties::read`）。同步体积见 `sync_size_test.go`：1500 条挂牌 + 3900 场（300 条在售挂牌每周一场 × 13 周）首次同步明文约 1.2 MB、gzip 后整包增加约 30 KB（合成数据，重复度高，真实数据压缩率会低一些）。本地 `listings.data.ofi[]` 保存 `{d,time,attendees:[],rexEventId,rexAttendeeCount,agent,agentId,rexAgentId[,agentFrom]}`；本地签到仍在独立 `ofi_attendees`，UI 数字为 Rex 计数 + 本地签到数（如果两边重复签到可能重复计数）。重新轮询只替换窗口内 Rex 场次、不动本地场次及本地签到；变更才更新 listings 版本，写 rex_links 和 rex_sync_events。

**挂牌状态（UAT3 问题 4）**：`data.status` 由 `system_listing_state` 映射（缺失时用旧字段 `listing_status`；值可能是字符串或 `{id,text}`）：`current`（含草稿，即未发布的 current 挂牌）→ `listed`；`sold`/`settled`/`unconditional` → `sold`；`leased` → `leased`；`withdrawn`/`expired`/`lost`/`cancelled` → `withdrawn`；`archived`，或 `system_record_state` 为 archived/trashed/deleted → `archived`；其他或缺失 → `unknown`。原值存 `data.rexState`，`data.category` 存 `listing_category.id`。只有 `listed` 在手机上显示 “Listed with <公司名>” 和 P 标；公司名来自 admin 设置 `company_name`（`PATCH /api/v1/admin/settings {companyName}`），为空时用环境变量 `AXLEO_COMPANY_NAME`，随登录和 `/me` 下发为 `user.company`；都为空显示 “Listed with our agency”。地图标记优先级：DNC > 置顶 > 本公司挂牌 P > 竞争对手挂牌 > pending > 普通点。

**中介（UAT3 问题 5）**：`listing_agent_1/2` 存为 `data.agents[{name,rexId,userId}]`，`data.agent` 为第一挂牌中介的 Rex 名字，`data.agentIds` 为能对应上的 App 用户 id（My listings）。每场的 `event_agent` 存为 `ofi[].agent/agentId/rexAgentId`（My open homes）；Rex 场次没选中介时用第一挂牌中介并标 `agentFrom:"listing"`。Rex 用户对应 App 用户的规则与房产负责人相同（`REX_USER_MAP`，再按相同邮箱或 `users.rex_user_id`），但**不**用 `REX_DEFAULT_OWNER_EMAIL` 兜底：对应不上的（如 “Marketing Team”）只显示 Rex 名字，`agentId` 为空。这些字段只由 poll 写；手机 PATCH 回传的 `agent/agents/agentIds/rexState/rexId/category` 对 `rex-listings-*` 记录忽略（`rexOwnedKey`）。手机端不再把 Rex 挂牌的中介替换成负责人或当前用户。可见性不变：listings 全员可见（场次负责人和挂牌中介都能收到），登记人 `ofi_attendees` 仍只给签到的本人（owner_id），不随场次中介放宽。

**房产属性**：Rex 房产导入只映射 `attr_bedrooms/attr_bathrooms/attr_garages` → `beds/baths/cars`；土地面积、地块、购入日期没有从 Rex 映射（字段名未在样例中证实）。手机 PATCH 带的是自己那份完整记录，版本不符时 409 由服务端版本覆盖；版本相符时 Rex 导入房产（`source=rex_import`）上手机发来的 `beds/baths/cars: null` 被忽略。房产 outbox 的 `properties` 任务在 rexworker 中不调用 Rex，这些属性不会写回 Rex。未找到场次则显示空状态。手机 OFI 页列出今天起的全部场次，按 Today / Tomorrow / 日期分组，筛选 All / My open homes / My listings。

**已实测 / 待确认**：2057 的 `ListingEvents::search` 真实开放日样本中 `event_type="open_home"`、`event_time="09:45:00"`、`event_timezone="Australia/Perth"`、`event_duration_minutes="30"`（字符串），没有 `event_end_time` / `end_time`；结束时间优先用这两个显式字段，缺失时由当地开始时间加时长分钟计算，时长无效才显示同一时刻。`Listings::search` 已在 probe 配置；初始导入演练（`tools/rex-import-dryrun.sh`，见 rex-feedback-2026-11 §10）读到 `system_listing_state` 的 current / sold / leased / withdrawn 四个值和 `listing_agent_1/2.id`；草稿挂牌的 `system_listing_state`、`event_agent` 非空时的结构（是否带 `name`）、`listing_agent_*` 是否带 `name`/`email` 仍待在沙箱核对（`event_agent` 字段存在，样例为 null）。登记数字段仍待确认。计数字段可用 `REX_OFI_COUNT_FIELD` 配置（默认 `attendee_count`）；字段缺失返回 0。`OpenHomeCheckIns` 已确认有 read/checkIn，但需要未知 token，未确认可搜索或聚合人数，因此**未调用**；不能把 0 当成已证实 Rex 无访客。须在沙箱对 16 Gager St 建真实场次并核对字段/人数，必要时再调整 event/count 映射。`CalendarEvents::search` 沙箱报内部错误，不作为数据源。

## 联系人搜索（feat/contact-search）

**流程**（`mobile/src/screens/Search.tsx`，判断逻辑在 `mobile/src/lib/contactSearch.ts`）：

1. 先搜手机本地数据（联系人、房产、挂牌、询盘）。**只要本地有任何结果就直接显示**，不请求后台，也不查 Rex。
2. 本地没有结果，且查询 ≥2 个字符（号码需要 ≥3 位数字）时，停止输入 500ms 后调用 `GET /api/v1/contacts/search?q=`。
   - 输入变化时，上一个请求会被取消（AbortController）。
   - 已在手机上的结果按 id 或 rexId 去重。
3. 后台**同时**查本地数据库和 Rex，合并后一次返回。手机只显示一个列表，标题为"后台与 Rex"：
   - **自己的联系人**：显示完整信息，点按打开（不在本机时先 sync）。
   - **其他中介的联系人**：只显示首字母加点，以及"归属：XX"；点按后提示联系归属人或申请改派。
   - **只在 Rex 的联系人**：标"在 Rex 中 / In Rex"，点按后调用 `import-rex`、sync，然后打开联系人。
4. Rex 相关的提示只显示为一行灰字，不作为错误：
   - `rexTimedOut` → "Rex 未及时响应 / Rex didn't respond in time"
   - `rexError` → "Rex 暂不可用，仅显示后台结果"
   - 限流 → "Rex 搜索稍后可用"
5. 离线且本地无结果时，提示"联网后可搜索后台和 Rex"。
6. 已删除"在 Rex 中搜索"按钮，也不再要求输入完整手机号才自动查 Rex。

**`GET /api/v1/contacts/search?q=`**（`server/contactsearch.go`）

- 查询少于 2 个字符时返回空列表（`rexSkipped`, `rexReason:"query too short"`）。
- **号码查询**：只由数字、空格、`+()-.` 组成，且至少 3 位数字。
  - 用 `normAUMobile` 规范化（`+61 4…`、`614…`、`04…`、带空格的写法都视为同一个号码）。
  - 在数据库中按 `mobile` 去掉符号后的数字做包含匹配，也匹配 `data.mobileNorm`。
- **名字查询**：匹配 名 / 姓 / 全名 / email（不区分大小写）。
- 数据库结果最多 20 条，自己的联系人排在前面。
- **Rex 查询**（与数据库查询并行，一次调用）：
  - 号码查询：规范化后 ≥8 位数字时调用 `Contacts::findMatchingContacts {"contact_type":"any","phones":[q],"limit":20}`。号码更短时不查 Rex（`rexReason:"partial number"`）。
  - 名字查询：调用 `Contacts::search {"criteria":[{"name":"contact.name_full","type":"like","value":"%q%"}],"limit":20}`。
    - **`like` 写法待沙箱确认**，文档没有列出 like，脚本 `.tmp/search/sandbox-check.sh` 会验证；不行就改用 `Contacts::autocomplete`。
- **合并规则**：
  - Rex 命中的记录如果已经通过 `rex_links` 关联到本地联系人，就以本地联系人的身份出现，遵守下面的权限规则。
  - 未关联的记录作为 `source:"rex"` 项返回（`rexId`、名字、`mobileMasked`、Rex 归属人），**不返回 email 或完整号码**。
- **时间上限 3 秒**：
  - Rex 的预算是 2.5s（`searchRexBudget`），登录、排队都算在内。
  - 超时只返回数据库结果，并带 `rexTimedOut:true`。
  - Rex 出错时返回 `rexError:true`，同样只返回数据库结果。
- **限流**：
  - 每个用户每分钟最多 20 次 Rex 调用（`allowEndpointCall(...,"rex_search",20)`）。超过时返回数据库结果，并带 `rexSkipped:true, rexReason:"rate limited"`。
  - 命中缓存不计入次数。
- **缓存**：按规范化后的查询缓存 60 秒（键为 `phone:0401999088` 或 `name:teddie bear`）。失败不缓存。
- **客户端**：
  - 只读（`ReadOnly`），Caller 为 `search`，只用 `REX_ACCOUNT_ID` 账户，登录后校验账户号。
  - 和 worker / poller 共用账户级闸门 `rexAccountGate`（默认间隔 1s，可用 `REX_GLOBAL_INTERVAL_MS` 调整）。
  - 每次调用都写入 `rex_request_log`。
  - 同一时间只发出一个 Rex 搜索请求。
- **响应格式**：`{items:[...], rexTimedOut, rexSkipped, rexReason, rexError}`。每一项包含：
  - `source:"local"|"rex"`、`full`、`name`、`ownerName`、`inRex`
  - `full:true` 时还会带 `id`、`first`、`last`、`mobile`、`email` 等字段

**权限规则（`contactSearchView`，待 Ping 确认）**：

| 情况 | 返回内容 |
|---|---|
| 管理员，或联系人归属于自己 | 完整信息 |
| 其他中介的联系人 | 只返回 id、首字母打码后的名字（`T••• B•••`）、`ownerName`、`inRex`；不含电话、email、备注 |
| 只在 Rex 中的联系人 | 名字、打码号码（`0401 999 •••`）、Rex 归属人；不含 email |

这些规则全部写在一个函数里，以后要调整只改这一处。

**`POST /api/v1/contacts/import-rex {"rexId":"..."}`**

- 只能导入**本用户最近 10 分钟搜索结果里出现过**的 rexId，否则返回 404。
- 和搜索共用每分钟 20 次的限流。
- 已关联的记录直接返回现有联系人（`imported:false`, 200）。
- 未关联的记录：先 `Contacts::read`，再走 `rexAdoptOrImport`。
  - 归属人是调用者本人，`REX_USER_MAP` 不覆盖。
  - 如果本地已有同号码、未关联的联系人，就接管它，不新建。
- 返回 `{imported, item}`（201），手机端随后执行 sync。

## 同号码查重（问题02）

**worker 新建前查 Rex**（`rexworker.go` → `rexdedupe.go: rexPreCreate`）。新建联系人前，用手机号调用 `Contacts::findMatchingContacts {"contact_type":"person","phones":[mobile]}`：

| 结果 | 处理 |
|---|---|
| 没有匹配 | 正常 `Contacts::create` |
| 唯一匹配，且该 Rex 记录没有关联到别的本地联系人 | **接管**：写 `rex_links`，create 任务标记为 `sent`；排一个 `PATCH {"rexAdopt":true}`，把 App 中非空的名字、号码、email 写回 Rex（不清空 Rex 字段，缺少的电话或 email 子行会补上） |
| 唯一匹配，但已经关联到另一个本地联系人 | 不关联，任务标 `skipped`；两个联系人都写入 `data.possibleDuplicateOf`，联系人页的重复提示会显示，提醒人工合并 |
| 多个匹配 | 任务 `failed "several Rex contacts share this mobile"`，等人工处理 |
| 查询失败（网络错误、5xx） | 按 worker 原有的退避策略重试，**不会盲目新建** |

另外，本地已有同号码、已关联的联系人（`findLocalDuplicate`）时，现在也标记 `possibleDuplicateOf` 并跳过，不再把同一个 rex_id 再关联一次。

**poller 接管未关联的本地联系人**（`rexpoll.go` → `rexAdoptOrImport`）。Rex 新出现、本地没有关联的联系人，先按规范化手机号找本地联系人：

- **本地联系人未关联，且它的 create 任务还没有 sent**：
  - 接管它，写 `rex_links`，把 create 任务标为 `sent`。
  - 字段合并按"最后写入者胜"：Rex 的 `system_modtime`（Unix 秒）和本地的 `updated_at` 比较，Rex 的空值不覆盖本地。
  - 本地较新时，排一个 `rexAdopt` PATCH 写回 Rex。
- **本地联系人已经关联到另一个 rex_id**：照常导入成新联系人，两边都写入 `data.possibleDuplicateOf`。
- **没有找到**：照常导入。

**检查已有的重复关联**（只读）：

- `axleo rex-dup-links` 列出多个本地联系人指向同一个 rex_id 的情况，包括 `rex_links` 和 `data.rexId` 两个来源。
- `serve` 启动时如果发现重复，会打一条 warning 日志，但不做任何修改。
- 确认后通过合并联系人处理（docs/contact-merge.md）。合并后只清除这一对的 `possibleDuplicateOf`，指向第三个联系人的标记保留；poller 不会恢复已被合并掉的联系人。

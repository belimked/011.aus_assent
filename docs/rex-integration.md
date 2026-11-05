# Rex 集成 · 后端设计

> 状态：草案 v0.1（2026-11-04）
> 依据：`中文版 - 信息收集开发需求.pdf`、`Rex_API_Integration_Guide.docx`、Rex 官方 API 文档（apibrowser 的 `ApiDocs::*` 和 `::describe` 接口）
> 标注「待确认」的内容要在拿到真实 Rex 账号后，用 `describeModel` / `describeSearchFields` / `read` 验证。

---

## 0. 已定决策

| # | 决策 | 说明 |
|---|---|---|
| D1 | **交互操作用中介本人的 Rex 身份；后台任务用集成用户**（2026-11-05 修订） | 中介在 App 里连接一次自己的 Rex 账号（`Authentication.loginWithOAuthAccessToken`），之后打开卡片、查重、录入、写备注都以本人身份调用，Rex 的权限和 closed database 设置自动生效，Rex 里看到的作者也是本人。集成用户只用于夜间对账、webhook、Pricefinder 同步，以及“号码是否已存在”这类不返回字段的查重。所有 token 只存在后端，手机端不直接连 Rex。依据：反馈要求“只显示中介在 Rex 中有权限的联系人”，而 `Contacts.findMatchingContacts` 等接口按调用者权限过滤（`min_user_relationship_to_contact` 默认 read）。 |
| D2 | **可见范围以 Rex 权限为准**（2026-11-05 修订） | 不再自建分配规则。我们库里的分配表只做缓存和地图着色加速，结果必须与本人身份调用 Rex 的结果一致；不一致时以 Rex 为准并刷新缓存。 |
| D3 | **按需同步** | 用户打开什么就刷新什么，写入后立即更新本地。每晚全量 e-tag 对账作为兜底。Webhook 是可选的加速手段，核心流程不依赖它。 |
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
- 每位中介的 OAuth token 加密存在数据库里，按用户隔离；中介离职或管理员撤销时立即删除，并停止该用户的所有 Rex 调用。
- 待确认：OAuth 调用是否仍按公司计入 3,600 次/小时限额（见 `feedback-q11-answers.md` V1）。
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

1. 用集成用户登录，读取 `AccountUsers`，把 Rex 用户和我们的用户对应起来（先按邮箱自动匹配，再由管理员在后台核对）；
2. 用 P2 优先级全量拉取 `Properties` 和 `Contacts` 的 id 和 etag，再分页拉取详情，写入索引表和关系表；
3. 根据 Rex 里的负责人字段（具体字段名待确认），生成 `assignments` 草稿，`source = rex_import`；
4. 管理员在「分配管理」页面按团队、区域核对后启用；
5. 导入完成后，**分配以我们的库为准**。之后 Rex 里负责人的变化**不会自动覆盖**我们的分配，只会出现在「差异提示」列表里，由管理员决定是否采纳。

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
  2. 去重：Contacts::findPossibleDuplicates（按手机、邮箱、姓名加地址的顺序）
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

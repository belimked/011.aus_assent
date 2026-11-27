# AxleoCollect 后端设计 v1（草案，待确认）

2026-11-16。已定：部署在 65（内网测试机）；后端用 Go；角色分中介（agent）和管理员（admin）；Rex 暂不对接，STT/翻译暂不做。

## 1. 范围

- 把移动端现在的本地 mock 数据（`mobile/src/data/types.ts` 的 `AppState`）换成服务器数据，多台手机、多个中介之间数据共享。
- 管理员能管账号、审批（合并、改派）、看审计日志、看待推送 Rex 的队列。
- 凡是原本要写 Rex 的操作，先写本地库，同时记一条 `rex_outbox`，状态为 `waiting_auth`。拿到授权后由后台 worker 补推，App 不用改。
- 本期不做：Rex/Pricefinder 对接、STT/翻译/朗读、OCR、短信发送、推送、DNCR 实时核验（这几项接口留好，继续用 mock）。

## 2. 技术选型

| 项 | 选择 | 理由 |
|---|---|---|
| 语言 | Go 1.27，标准库 `net/http` 路由 | 65 已装 Go 1.27.3，和同机 xiaohe 一致 |
| 数据库 | SQLite（`modernc.org/sqlite`，无 cgo），WAL 模式 | 单机、几十个中介的量足够；备份就是拷文件。以后上量再换 PostgreSQL，SQL 尽量写通用 |
| 登录 | 邮箱 + 密码（bcrypt），不透明 token（库里存哈希，可吊销），有效期 30 天，滑动续期 | 比 JWT 好吊销；设备丢了管理员可踢下线 |
| 迁移 | SQL 文件 `embed` 进二进制，启动时按版本执行，执行前自动备份库 | |
| 管理端 | Go 内嵌一个小的 Web 管理页（`/admin`） | 不另起服务 |

## 3. 角色与权限

| 能力 | agent | admin |
|---|---|---|
| 看/改自己负责的联系人、待办、采集 | ✓ | ✓ |
| 看团队其他人的联系人 | 只读，手机号/邮箱打码（同原型 Mary/John） | 全部明文 |
| 改派联系人/待办给别人 | 发起申请 → 管理员审批 | 直接改 |
| 合并联系人 | 发起申请 | 审批/直接合并 |
| 账号管理、重置密码、踢设备 | ✗ | ✓ |
| 审计日志、Rex 队列、数据导入 | ✗ | ✓ |

所有写操作写 `audit_log`（谁、什么时候、改了什么、来自哪台设备）。

## 4. 数据表（主要字段）

所有业务表都有 `id TEXT`（客户端生成 ULID，便于离线新建、重试幂等）、`team_id`、`created_at`、`updated_at`、`version`、`deleted_at`（软删）。

- `users`（email, name, role, password_hash, disabled）、`sessions`（token_hash, user_id, device, expires_at）
- `contacts`（first, last, mobile, email, owner_id, roles, tags, flags, consent），`contact_notes`
- `properties`（no, street, suburb, geometry, pf_owner, last_sale, flags）——首次从 `web/public/parcels.json`（原手机内置 QLD 数据）导入；`property_owners`（property_id, contact_id, relation）
- `todos`（due, time, text, assignee_id, created_by, kind, done, contact_id, property_id）——`created_by` 用来显示 “Assigned by me”
- `captures`（离线语音/文字记录：text, audio_ref, status, property_id, extracted JSON）
- `enquiries`、`leads`、`listings`、`ofi_events`、`ofi_attendees`、`sales`、`contracts`、`referrals`
- `market_items`（市场监控）、`zones`（片区多边形）、`tags`（团队标签/我的标签，含排序、使用次数）
- `approvals`（kind=merge/reassign…, payload, status, requested_by, decided_by）
- `audit_log`、`rex_outbox`（entity, op, payload, status=waiting_auth/pending/sent/failed, tries, last_error）

## 5. 接口（前缀 `/api/v1`，JSON）

- 认证：`POST /auth/login`、`POST /auth/logout`、`GET /me`
- 同步：`GET /sync?since=<cursor>` 返回自 cursor 以来所有变化（含删除），App 本地缓存 + 增量同步；`POST /sync/push` 批量提交离线期间的写操作（按 id 幂等，`version` 冲突返回 409 和服务器当前值）
- 资源：`contacts`、`properties`、`todos`、`captures`、`enquiries`、`leads`、`listings`、`ofi`、`sales`、`market`、`zones`、`tags` 的标准 GET/POST/PATCH/DELETE
- 动作：`POST /contacts/{id}/merge`、`POST /todos/{id}/reassign`、`POST /approvals/{id}/decide`、`POST /market/{id}/confirm`
- 管理：`/admin/users`、`/admin/audit`、`/admin/rex-outbox`、`/admin/import`
- 健康：`GET /health`（版本号、库版本）

## 6. 移动端改造

- `src/store/index.ts` 保留 `useStore/update` 的用法不变，底下换成：本地 AsyncStorage 缓存 + 后台增量同步 + 写操作入队（离线可用，联网自动补发，失败显示在现有 sync 队列页）。
- 登录页接真实登录；Me 页的 “Reset demo” 改为 “退出登录 / 清本地缓存”。
- API 地址按构建环境配置（`app.json` extra）。保留离线 demo 模式开关，方便没网时演示。

### 6.1 新包首次启动自动全量同步（2026-11-27）

- **机制**：`mobile/src/store/sync.ts` 在 `ax_sync`（`{cursor,queue,lastSync,build}`）里记下“上次完成全量同步时的构建标识” `build`，取值 `版本号(原生构建号)`，例如 `0.1.0(67)`；版本号来自 `expo-constants` 的 `expoConfig.version`，构建号来自 `Constants.platform.ios.buildNumber`（Info.plist 的 `CFBundleVersion`，CI 写入 run number；Android 用 `versionCode`）。
- **触发**：已登录用户启动 App 时，若记下的标识与当前包不同（包括旧包从没记过），下一次同步先照常推送离线队列，再不带 `since` 拉取 `GET /sync`；服务器返回 `full:true`，手机按表整条替换本地服务器数据（联系人、房产、待办、挂牌、市场、区域、标签、活动、审批等），本地还有未上传修改的记录保留本地版本。`/me` 每次启动都会重新取，公司名等登录信息随之刷新。
- **安全性**：推送失败或离线时不拉全量、不清队列、不更新标识，下次联网自动重试；只有全量数据已替换并写盘后才写入新标识。设置、语言、语音语言、片区、合作方、邮件列表目录等本地状态不受影响，登录态保留。刚登录本来就是全量拉取，完成后直接记下当前标识，不会重复拉。演示模式 / 未登录不触发。界面沿用现有“上次同步”时间，没有新提示。
- **发版**：出了新包（构建号变了）后，**不再需要让测试人员退出再登录**。
- **例外**：只改服务器、没出新包时，手机端不会因为构建号自动全量。若改动影响已下发记录的内容（例如服务器给记录新增派生字段），仍需让测试人员退出再登录。

## 7. 部署（65）

> **2026-11-24 已迁到 AWS Lightsail 悉尼（16.178.91.22），走新隧道 `axleo-lightsail`，域名不变。** 现行部署见 `docs/ops-runbook.md`“部署位置”；下表为 65 时期的原设计，目录/用户/单元在新机器上保持一致。

和同机 xiaohe 隔离，不碰它的文件和服务：

| 项 | 值 |
|---|---|
| 系统用户 | `axleo` |
| 程序 | `/opt/axleo/{releases,current,bin}`，`current` 软链指向当前版本，便于回滚 |
| 配置 | `/etc/axleo/env`（root:axleo 640） |
| 数据 | `/data/axleo/{db,backup}` |
| 服务 | `axleo.service`，监听 `127.0.0.1:8082`（8080/8081 已被 xiaohe/submgt 占用） |
| 备份 | 每天 03:30 拷库，保留 14 份 |
| 外网 | 走 Cloudflare Tunnel，新子域名（建议 `axleo-api.saulxj.ggff.net`），手机 App 才能访问 |
| 发布 | GitHub Actions 跑 `go test` + 编译；部署第一版用本机脚本 scp 到 65 → 切软链 → 重启 → `/health` 失败自动回滚。以后可在 65 上给本仓库再装一个 runner |

## 8. 里程碑

- **B1**：Go 骨架、迁移、登录/角色、联系人/房产/待办 CRUD、审计、部署到 65 + 外网健康检查
- **B2**：增量同步 + 离线写队列；移动端登录和 store 接入；导入原型演示数据
- **B3**：其余业务表（采集、询盘、线索、OFI、成交、市场、片区、标签）和审批流
- **B4**：管理端页面、`rex_outbox` 队列页、备份与回滚演练；出 iOS 包验收

## 9. 已确认（2026-11-16）

1. 外网：Cloudflare Tunnel，子域名 `axleo-api.saulxj.ggff.net`。
2. 数据库：SQLite。
3. 中介之间**完全隔离**：agent 只能看到/操作自己负责（owner/assignee/created_by）的数据；第 3 节"看团队其他人的联系人"一行作废，改派/合并由 admin 审批。房产（公共地块数据）所有人可见，但房产关联的联系人只显示自己的。
4. 首批账号：1 个 admin + 演示中介 Mary、John。
5. 导入原型演示数据作为测试数据（按原型里的负责人分给 Mary/John）。
6. 前端补缺（摇一摇翻译 + S/M 项）与后端同时进行。

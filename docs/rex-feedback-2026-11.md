# Rex 技术回复（2026-11）· 对设计的影响

> 来源：Rex 邮件 “Re: Write or read data / data entry”，逐条回答我们问题清单的 Section 02 / 03 / K。
> 本文只记录结论和要改的地方；`rex-integration.md` 中与本文冲突的内容以本文为准。

## 1. 已确定（直接照做）

| # | Rex 回复 | 我们怎么做 |
|---|---|---|
| K3 / Q1 | `X-App-Identifier: Integration:PlaceSunnybank:Portal`；只给 Place Sunnybank 用不需要 Verified Integrator | 写进连接器配置。**以后要卖给其他 Rex 中介，必须先申请 Verified Integrator** |
| K1 / Q11 | 集成用户免费、不占席位，走 Rex 内部审批后通知 | 等 Rex 通知；不再有席位费用问题 |
| K1 / 03-5 / Q8 | 3,600 次/小时是**整个 Place Sunnybank 账号**共用的软限制；BatchRequests 里每个子调用都计数 | 保留连接器限流队列（令牌桶 + 优先级）；Batch 不省额度，只省往返 |
| 02-2 | 路径用小写连字符：`/v1/rex/leads/create` | 修正 `rex-integration.md` §2.1（原写 `<Service>/<method>`） |
| 02-1 | 初次导入用 search + offset/limit 分页；日常变化用 e-tag 同步，可加 webhook；不建议 `Export::queueExport` | 与 D3 一致：初次导入分页 + 夜间 e-tag 对账 |
| 03-6 / Q9 | 可用 webhook（中介给集成用户开 Manage Webhooks / Create Zaps 权限），仍建议每晚 e-tag 对账 | 第一阶段可上 webhook 实现“1 分钟内更新”，夜间对账兜底 |
| 02-6 | `contact_tags` 已废弃，用 `related.contact_tags`；标签没有分组 | App 的标签分组映射成前缀：`DK - `、`OFI - `、`Enquiry - ` |
| 03-11 / Q7 | 不投信用联系人的 `contact_by_letter = 0`；**不要在同一次 update 里带 `is_dnd`**（会重置所有联系渠道） | NO JUNK MAIL → `contact_by_letter=0`；DND 单独一次 update |
| 02-x | `Dedupe::combineRecords(winning_id, losing_ids)` | 正好对应 B3 的合并审批：主记录 = winning，副记录 = losing |
| 02-x | `Reminders::toggleCompletionState` + `create_activity_entry` 写的是活动日志，不是联系人备注 | 待办勾选完成走这个；需要备注时另调 `Notes::create` |
| 02-x | `SystemValues` 返回 id/text；`getValueCategories` 列出所有值列表 | 关系类型、来源等下拉从这里取（Q2/Q5 在沙箱确认） |
| 02-x | `Properties::getHistory` 返回该房产的 OAB、估价、挂牌记录 | 房产详情“历史”区用它 |
| 02-x | `ListingEvents` 可按 `event_type / event_date / event_time` 搜索 | OFI 场次从这里读 |
| 03-3 | 反馈和线索是两种记录，只写线索即可 | 询盘只写 lead |
| 03-4 | 自定义字段要中介先在 Rex 建好字段定义，集成只写值 | 列清单交给中介建（见 §3） |
| 03-7 | 第一阶段的房产更新方式可行 | 不变 |
| K2 | 给了共享沙箱账号，邀请已发到我们注册的邮箱；共享账号，不是专属 | 可以开始对沙箱开发连接器（注意别留真实个人信息） |

## 2. Rex 做不到 → 改用备用方案

| # | 功能 | Rex 回复 | 影响 / 方案 |
|---|---|---|---|
| 02-15 | Market 页“每日新成交、新挂牌” | 不提供，Rex 只有本中介自己的记录和自己录的 OAB，不是市场数据源 | **Market 页需要另找数据源**（见 §4 决策 1） |
| 02-16 | 上次成交、登记业主、Pricefinder | 不能通过集成提供（Pricefinder 授权给中介本身） | 房产卡片的成交价/业主建议需另接数据商（见 §4 决策 1） |
| 03-1 | Prospecting 地图 | 依赖 Rex Prospecting，不确定集成用户能否用 | 用我们自己的按 suburb 搜索方案（已在设计中） |
| 03-2 | 估价预约（空闲时段、短信、提醒） | 未对外开放 | 用备用方案：我们建待办/提醒 + Rex 里写 appraisal 记录（沙箱确认能否写） |
| 03-8 | 记录外发的短信/邮件 | 不提供 | 用 Notes 记录“已发送 xxx” |
| 03-9 | 更多 match profile 条件 | 不提供 | 多出的条件存在我们库里，匹配在我们这边做 |
| 03-10 | 以其他中介身份写入 | 不提供；**所有记录的创建人显示为集成用户** | 见 §4 决策 2 |

## 3. 要交给中介（Place Sunnybank）办的事

1. 在 Rex 里给集成用户开 **Manage Webhooks / Create Zaps** 权限。
2. 按清单在 Rex 建自定义字段定义（字段清单待沙箱里核对后给出，例如：采集渠道、App 记录 id、不敲门）。
3. 确认标签前缀命名：`DK - `、`OFI - `、`Enquiry - `。
4. 如果要 Market 页和成交/业主数据：由中介提供自己的数据商授权（见决策 1）。

## 4. 需要业务方决定

**决策 1：Market 页和成交/业主数据从哪来**（Rex 和 Pricefinder 集成都给不了）
- A. 中介用自己的 Pricefinder / CoreLogic 授权，单独接数据商 API（要费用和合同）；
- B. 用公开数据（QLD 政府地籍/成交公开数据，粒度和时效差）；
- C. 第一阶段先去掉 Market 页的自动来源，只保留中介手动录入。
建议先 C，同时问中介是否有数据商 API 授权。

**决策 2：写入身份**（原 D1“以中介本人身份写入”不成立）
- Rex 明确：集成写入的记录创建人都是集成用户，不能代写为其他中介。
- 建议：统一用集成用户写入；每条备注/线索正文带 “via AxleoCollect · Mary”，并把负责中介写进 Rex 的负责人字段（如 `listing agent / contact owner`，沙箱确认字段名）。
- 可见范围也随之改为我们自己的隔离规则（B1–B3 已按此实现），不再依赖“以本人身份调用 Rex 自动过滤”。原 D2“可见范围以 Rex 权限为准”需要改写；若中介坚持按 Rex 权限，需要 Rex 确认集成用户能否读到每条记录的权限设置。

## 5. 下一步（等沙箱账号到手）

1. 接受 Rex 沙箱邀请，把账号交给我们（密码放到服务器的 `/etc/axleo/env`，不进仓库）。
2. 在沙箱上跑 describe-model / read-fields / searchable-fields，补完 Q2、Q4、Q5、Q6 和自定义字段清单。
3. 实现 Rex 连接器（限流队列、e-tag 对账、webhook 接收），把 `rex_outbox` 从 `waiting_auth` 改为真实推送——**只连沙箱**，正式账号等中介授权后切换。

## 6. 未回答 / 需要追问（2026-11-16 对照问题清单）

我们的问题清单（cc.saulxj.ggff.net/rex-api-requirements-en）里，以下几处 Rex 没有回答或答非所问：

| # | 我们问的 | Rex 的回复 | 处理 |
|---|---|---|---|
| F1 | Contacts::create / update 时 related.contact_tags 写一个账号里还没有的标签，会不会自动创建？ | 未回答 | 沙箱验证（axleo rex-probe） |
| F2 | Leads::create 直接指定 assignee 时，集成用户是否需要 leads.assign_to_anyone 权限？只传 listing 不传 assignee 时，会不会自动分给该挂牌的中介？ | 未回答 | 沙箱验证（axleo rex-probe） |
| F3 | Reminders::create 指派给同事后，Rex 手机端会不会推送通知？ | 未回答 | 沙箱验证，需人工看手机推送 |
| F4 | 03-3：签到时除了 lead，能否写 Rex 原生的开放参观到访记录，让到访出现在给卖家的报告里？ | 答成“反馈和线索是两种记录，只写线索即可” | 沙箱验证（axleo rex-probe）API 部分；vendor report 人工核对 |
| F5 | 03-11：要的是“这户不要投信”，按房产、可搜索、可按片区统计 | 给了联系人上的 contact_by_letter | 沙箱验证（axleo rex-probe） |
| F6 | K1：超过 3,600 次/小时会怎样（拒绝/延迟/停用）？有没有每日上限、并发上限？ | 只说“目前是软限制，没有公布其他限额” | 邮件追问 |
| F7 | 02-1：联系人与房产的关系（业主/租客）如何批量拉取 | 只答了联系人、房产用 search 分页 | 沙箱验证（axleo rex-probe） |
| F8 | （新）每位中介用自己的 Rex 账号授权（OAuth），记录创建人显示本人，是否允许？除 X-App-Identifier 外还需要什么？是否同样计入 3,600 次？ | 03-10 只回答了“集成用户不能代写” | 邮件追问 |

以下 Rex 只说“看 describe-model / read-fields / searchable-fields”，**不需要追问**，拿到沙箱后自己确认返回结构：02-3 Reminders::getRemindeeList、02-4 Dedupe::findPossibleDuplicates、02-7 AdminTags::search、02-8 Leads::search、02-9 Reminders::search、02-11 MatchProfiles::search、02-13 Oabs::search。

### 当前在等的
1. Rex 审批“免费集成用户”（邮件说已提交，批准后通知）。
2. 沙箱邀请已发到 Sunnybankintegration@eplace.com.au：需要 Ping 接受邀请，把沙箱登录信息交给我们。
3. 中介在 Rex 里：给集成用户开 Manage Webhooks / Create Zaps 权限；建自定义字段（同意方式、DNCR 核验日期等，清单在沙箱核对后给出）。

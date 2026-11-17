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
| F1 | Contacts::create / update 时 related.contact_tags 写一个账号里还没有的标签，会不会自动创建？ | 未回答 | **已确认（2026-11-17 沙箱写入）**：写 `related.contact_tags: [{tag}]`，账号里没有的标签照样保存在联系人上，读回可见；不会出现在 AdminTags（后台预设标签列表）。自由文本标签可直接用 |
| F2 | Leads::create 直接指定 assignee 时，集成用户是否需要 leads.assign_to_anyone 权限？只传 listing 不传 assignee 时，会不会自动分给该挂牌的中介？ | 未回答 | **部分确认（2026-11-17）**：指定 assignee 可写入（共享沙箱只测了指派给集成用户自己，派给其他用户是否要 assign_to_anyone 仍未知，到正式账户或邮件确认）；只传 listing 时 **不会自动分配**（assignee 为 null），需要我们自己指定负责中介 |
| F3 | Reminders::create 指派给同事后，Rex 手机端会不会推送通知？ | 未回答 | 2026-11-17 已能创建（必填：`remindee:{id,type:"user"}`、`reminder_type:{id:"task"}`、`reminder_date`，且必须挂在 contact/property/listing 等对象上）；推送需人工用集成账号登录 Rex 手机端查看 |
| F4 | 03-3：签到时除了 lead，能否写 Rex 原生的开放参观到访记录，让到访出现在给卖家的报告里？ | 答成“反馈和线索是两种记录，只写线索即可” | 沙箱验证（axleo rex-probe）API 部分；vendor report 人工核对。**2026-11-17 只读结果**：没有 `ListingEventAttendees`；Rex 有 `OpenHomeCheckIns`（`read` / `checkIn`），两者都要一个 check-in token，token 从哪来未知。**到访记录的正确入口是 Feedback**：`feedback_type=ofi` 的 Feedback 记录挂在 `listing` + `date_of` 上，到访人在 `related.feedback_individual[{contacts, interest_level, enquiry_source, price_indication, note}]`，`Feedback::create` 可写；`Feedback::searchOpenHomeAttendance` / `searchVendorSendStage` 显示这些记录会进入给卖家的反馈。**2026-11-17 沙箱写入已确认**：`Feedback::create`（必填 `date_time_start`，格式 `HH:MM:SS`）写入 `feedback_type ofi + listing + date_of + related.feedback_contacts + related.feedback_individual[{contacts:[{contact:{id}}], interest_level:{id:hot/warm/cold}, enquiry_source:{id:"198" Open Home}, price_indication, note}]`；读回一致；`searchOpenHomeAttendance` 能查到；在 `searchVendorSendStage(to_send)` 以 `kind: attendee` 出现，即进入给卖家的反馈队列。Feedback 只有 trash，没有 purge。Rex 网页 vendor report 未人工看 |
| F5 | 03-11：要的是“这户不要投信”，按房产、可搜索、可按片区统计 | 给了联系人上的 contact_by_letter | **2026-11-17 只读结果**：房产本身有 `property.is_dnd`（Do Not Contact，真实字段，可搜索）；联系人可按 `contact.contact_by_letter` 搜索；房产也可按 `owning_contacts.contact.contact_by_letter` 搜索。是否用 `property.is_dnd` 表示“不投信”需业务确认（它是“不联系”，比“不投信”宽） |
| F6 | K1：超过 3,600 次/小时会怎样（拒绝/延迟/停用）？有没有每日上限、并发上限？ | 只说“目前是软限制，没有公布其他限额” | 邮件追问 |
| F7 | 02-1：联系人与房产的关系（业主/租客）如何批量拉取 | 只答了联系人、房产用 search 分页 | **已确认（2026-11-17）**：`Properties::search` 加 `extra_options.extra_fields: ["contact_reln_property"]`，每条房产返回 `related.contact_reln_property[{contact, reln_type, do_not_contact, id}]`，可分页批量拉 |
| F8 | （新）每位中介用自己的 Rex 账号授权（OAuth），记录创建人显示本人，是否允许？除 X-App-Identifier 外还需要什么？是否同样计入 3,600 次？ | 03-10 只回答了“集成用户不能代写” | 邮件追问 |

以下 Rex 只说“看 describe-model / read-fields / searchable-fields”，**不需要追问**，拿到沙箱后自己确认返回结构：02-3 Reminders::getRemindeeList、02-4 Dedupe::findPossibleDuplicates、02-7 AdminTags::search、02-8 Leads::search、02-9 Reminders::search、02-11 MatchProfiles::search、02-13 Oabs::search。

### 当前在等的
1. Rex 审批“免费集成用户”（邮件说已提交，批准后通知）。
2. 沙箱邀请已发到 Sunnybankintegration@eplace.com.au：需要 Ping 接受邀请，把沙箱登录信息交给我们。
3. 中介在 Rex 里：给集成用户开 Manage Webhooks / Create Zaps 权限；建自定义字段（同意方式、DNCR 核验日期等，清单在沙箱核对后给出）。

## 7. 连接器接口沙箱验证（2026-11-17，账户 2057，测试数据已清理）

| # | 接口 | 结果 | 请求要点 / 注意 |
|---|---|---|---|
| A1 | `Notes::create` | ✅ | `data:{note, note_type:{id:"note"}, related:{note_contacts:[{contact:{id}}], note_properties:[{property:{id}}]}}`；创建人为集成用户 |
| A2 | `Reminders::toggleCompletionState` | ✅ | `{id, is_completed:true, create_activity_entry:true}`；读回 `system_completed_time` / `system_completed_by_user` |
| A3 | `Dedupe::combineRecords` | ✅ | `{service_name:"Contacts", winning_id, losing_ids:[...]}`；副记录变为 trashed，电话等子记录并入主记录（重复项不去重，需要时我们自己清理）。查重 `findPossibleDuplicates` 需 `service_name`、`record_id`、`match_fields`（只能是 name / email / phone） |
| A4 | `Contacts::update` 联系渠道 | ✅ 有坑 | 只改 `contact_by_letter:0` 不影响其他渠道。**`is_dnd:1` 会把四个渠道全部置 0；`is_dnd:0` 会把四个渠道全部恢复为 1，之前的不投信设置被覆盖**。取消 DND 后必须再单独写一次 `contact_by_letter:0` |
| A5 | `Leads::create` 带联系人 | ✅ | `data:{contact:{id}, listing:{id}, assignee:{id}, note}` |
| A6 | 房产-联系人关系写入 | ✅ | `Properties::update` `data:{id, related:{contact_reln_property:[{contact:{id}, reln_type:{id:"owner"}}]}}`；联系人侧同步可见。reln_type：owner / tenant / past_owner / past_tenant |
| A7 | `Properties::update` / `getHistory` | ✅ | update 用 `data:{id, ...}`；房产 `is_dnd` 可写（可作为“这户不联系”）；`getHistory` 返回 listed / appraisal 等事件流 |
| A8 | e-tag | ✅ | `search` 加 `result_format:"etags"` 返回 `[{id, etag}]`（etag = `id-修改时间`），记录修改后变化 |
| A9 | `Appraisals::create` | ✅ | `data:{property:{id}, appraisal_type:{id:"sale"}, appraisal_date, agent_1:{id}, price_min, price_max}`；出现在房产历史 |
| A10 | `BatchRequests::execute` | ✅ 有坑 | `requests:[{method:"Contacts::read", args:{...}}]`，返回按顺序的结果数组。**任一子请求失败整个批量返回错误、拿不到其他结果**；连接器里批量只用于不会失败的读，写入逐条发 |

删除方式：Leads / Notes / Contacts / Properties / Feedback 用 trash；Reminders / Appraisals 可 purge。

## 8. 第二轮沙箱验证（2026-11-17，B 组，测试数据已清理）

| # | 功能 | 结果 | 请求要点 / 注意 |
|---|---|---|---|
| B1 | 录入前查重 `Contacts::findPossibleDuplicates` | ✅ 有坑 | 不需要已有记录 ID，直接传 `phone` / `email` / `name_first+name_last`。电话忽略空格标点；邮箱不区分大小写。**`+61412…` 和 `0412…` 不算相同**，查重前必须统一成 `04…`。默认 classic 模式只给姓名时返回空；Rex 建议集成用 `match_behaviour:"strict"`（给了姓名必须同时匹配） |
| B2 | 给已有联系人追加标签 | ✅ | `Contacts::update` 的 `related.contact_tags:[{tag}]` 是**追加**，原有标签保留 |
| B3 | 加入邮件列表 | ✅ | `related.contact_mailing_lists:[{mailing_list_name}]`（要用名称，传 id 报错 “Mailing list name must be specified”）；列表读 `AdminMailingLists::search`；成员数 `getListMembers` |
| B4 | 联系人-挂牌关系 | ✅ | `Contacts::update` `related.contact_reln_listing:[{listing:{id}, reln_type:{id:"purchtenant"}}]`；类型只有 owner / purchtenant |
| B5 | 删除关系（撤销） | ✅ | `related.<子表>:[{id, _destroy:true}]`；业主关系、挂牌关系可只传 id；**删标签必须同时带 `tag` 值**，否则报 tag is required |
| B6 | 按区域查房产 | ✅ 有坑 | 条件 `property.adr_suburb_or_town` / `property.adr_postcode`（多个用 `type:"in"`）；`result_format:"map"` 返回 `{id, adr_latitude, adr_longitude}`；etags 一次最多 50,000、ids 最多 1,000,000，沙箱 3,450 套一次拿完。**增量 `system_modtime` 必须传 Unix 秒数，传日期字符串会被静默忽略并返回全部记录** |
| B7 | 地址查重 `Properties::findPossibleDuplicate` | ✅ | 平铺 `adr_*` 参数，返回 id 或 false；`St` 与 `Street` 视为相同，缺邮编也能匹配；**单元号必须放 `adr_unit_number`**，不带单元号查不到 `2/7` |
| B8 | `Properties::setGeoLocation` | ✅ | `{id, latitude, longitude}`，写入 `adr_latitude/adr_longitude` |
| B9 | 读最近备注 `Notes::search` | ✅ | 条件 `related_property.id` 或 `related_contact.id`，`order_by:{system_ctime:"DESC"}` |
| B10 | `name_etags` | ✅ | 行为 `{id:"cid:<联系人>_pid:<姓名行>", etag}`，一个联系人多个姓名时多行；单页上限 10,000 |
| B11 | token 过期 | ✅ | 过期返回 **HTTP 401 `TokenException`（code 3）**，消息 “The token you provided has just expired.”。`token_lifetime` 最长 1 周，有活动会续期。**`rexclient.go` 目前没有过期重登，连接器要加：遇 TokenException 重新登录后重试一次** |

## 9. 自定义字段（2026-11-17，沙箱，用其他集成商已建的字段在我们自己的测试记录上读写，已清理）

| 功能 | 结果 | 要点 |
|---|---|---|
| 读字段定义 | ✅ | `AdminCustomFields::getFlatDefinition({module_name:"contacts"/"properties"})`，返回 `field_id / field_name / field_fq_name(Tab.Group.Field) / field_type / field_options`；支持 Contacts / Properties / Listings / AccountUsers |
| 写值 | ✅ | `CustomFields::setFieldValues({service_name:"Properties", service_object_id, value_map:{<field_id 或字段名>: 值}})`，返回该记录全部自定义字段 |
| 写值 + 改主记录一次完成 | ✅ | `setFieldValuesAndUpdateRecord({..., value_map, data:{id,...}, return_custom_fields_keyed_by:"field_name"})` |
| 读值 | ✅ | `readFieldValuesAndRecord` 或 `getValuesKeyedByFieldName`（service_name 必须首字母大写 `Properties`）；普通 `Contacts::read` **不带**自定义字段 |
| 按自定义字段搜索 | ✅ | `Properties::search` 条件名 `cf.Properties.<field_id>` 可直接用 |
| 下拉字段 | ⚠️ | 写不在选项里的值（"Landlord"）**不报错**，校验要我们自己做 |
| 新建字段 / 分组 / 标签页 | ✅（沙箱） | `AdminCustomFields::createTab({data:{module_name:"properties", label, priority}})` → `createGroup({data:{tab_id, label, priority}})` → `createField({data:{group_id, label, field_type_id:"select", options:[...], priority}})`，均返回带 id 的对象；字段立即出现在 `getFlatDefinition`，可写值、可用 `cf.Properties.<id>` 搜索；`updateField({data:{id, options}})` 可加选项。删除：`purgeField({id, confirm:true})`（会删掉所有已存值），`purgeGroup` / `purgeTab` 只收 `{id}`，须先删字段再删组再删标签页。**下拉字段写不在选项里的值（"Bogus"）照样保存**。集成用户在沙箱有建字段权限；正式账户 758 是否同样有权限未知 |

## 10. 初始导入演练（2026-11-17，沙箱 2057，只读）

脚本 `tools/rex-import-dryrun.sh`（只落 id / 类型 / 关系，不存姓名电话）。全量 **132 次调用、约 4 分钟、0 错误**。

| 对象 | 数量 | 方法 | 带回的关系 |
|---|---|---|---|
| 房产 | 3,449 | `Properties::search` + `extra_fields:["contact_reln_property"]`，每页 100 | 业主/租客关系、坐标（全部有）、负责人 `system_owner_user` |
| 联系人 | 7,856（person 7,817 / company 39） | `Contacts::search` + `extra_fields:["related.contact_reln_property","related.contact_reln_listing"]` | 房产关系、挂牌关系、负责人、DND、不投信 |
| 挂牌 | 1,413（current 1,196 / sold 128 / leased 47 / withdrawn 42） | `Listings::search` | 房产 id（全部能对上）、listing_agent_1/2 |
| 用户 | 214 | `AccountUsers::search` | — |

关系两侧对账：房产侧与联系人侧一致 1,591 条；差异都能解释：
- 房产侧有 **375 条关系的 contact 为 null**（联系人已删除或不可见），导入时跳过；
- 有 **81 条重复关系**（同一联系人、同一类型、不同关系 id），导入时按 (房产, 联系人, 类型) 去重；同一联系人在同一房产上可有多种类型（如 tenant + past_owner），属正常；
- 有 47 条 reln_type 为 null；挂牌关系里出现自定义类型 id（如 `1166`），不在标准值列表里，要按“其他”处理。

要注意：
- **search 返回的 `is_dnd` / `contact_by_letter` 是字符串 "1"/"0"，read 返回布尔值**，解析时统一；
- **负责人对不上**：128 个负责人 id 中 21 个（含 `0`）在 AccountUsers 里查不到（已删除的用户），涉及 1,223 条记录 → 导入时归为“未分配”，由管理员处理；
- 按 3,600 次/小时算，这个规模的全量导入占 132 次；Place Sunnybank 正式数据量要在 758 上数一下才能估。

## 11. Webhook 实测（2026-11-17，沙箱 2057 → 65）

- 接收端：`POST /api/v1/rex/webhook/{token}`（`server/rexwebhook.go`），token 来自 `AXLEO_REX_WEBHOOK_TOKEN`（≥32 位，未设置时 404）；只记录事件名和记录 id，回 202。经 Cloudflare Tunnel `https://axleo-api.saulxj.ggff.net` 对外。65 上部署版本 `461c75d4b114-wh1`（未提交代码，手动发布，`/data/axleo/previous-release` 指向 `461c75d4b114`）。
- 集成用户有 `AdminWebhooks` 权限。创建：`AdminWebhooks::create({data:{name, url, send_format_id:"v1_context_only", is_enabled:true, events:[{event_id:"contacts.created"}, ...]}, return_id:true})`（events 要用 `event_id`，不是 `{event:{id}}`）；删除 `purge({id})`。
- Rex **不签名**，只能靠 URL 里的随机 token 防伪造。
- 收到的内容（v1_context_only）：`{id:<事件uuid>, type:"contacts.updated", account_id, service:"Contacts", record_id, 操作人 id}`，不含字段值。

| 事件 | 延迟 |
|---|---|
| `sendTestEvent`（type=sample） | 约 98 秒（刚创建 webhook 后） |
| 第一次 contacts.updated | 约 68 秒（同上） |
| **第一次 contacts.created** | **没收到**（webhook 刚创建约 10 秒时） |
| properties.created / updated | 7–8 秒 |
| contacts.updated / contacts.created（之后） | 4–5 秒 |

结论：稳定后几秒内送达，满足“1 分钟内更新地图”；但**刚创建时有延迟、且丢过一条**，事件 id 每条唯一可用于去重；夜间 etag 对账必须保留作兜底。测试 webhook 和测试记录已删除。

## 12. 沙箱测不了的三项：已知信息与下一步（2026-11-17）

**限流（F6）**
- Rex 文档 Rate Limits：**3,600 次/小时，按中介公司账户计**（多个中介账户可以分别用满）；目前是软限制，“以后会严格执行”。Sync 文档：请求过多“可能被限速，甚至被停用 API 访问”。
- 响应头 CORS 暴露了 `Retry-After`，推测超限时会返回 429 + `Retry-After`（未证实）。没有用量响应头，只能自己计数。
- 做法：连接器按账户令牌桶，上限按 3,000/小时（留 1/6 余量），交互请求优先；遇 429/5xx 按 `Retry-After` 或指数退避。白天估算约 800 次/小时（`rex-integration.md` §10），初始导入在沙箱规模是 132 次。
- 仍要问 Rex：超限后是 429 还是直接停用、`Retry-After` 是否可靠、有没有并发/每日上限。

**每位中介用自己的 Rex 账号授权（F8）**
- 接口有 `loginWithOAuthAccessToken({access_token(at+jwt), account_id})`，但 Rex 公开文档的 Authentication 只写了用户名密码登录，**没有任何 OAuth 客户端申请、授权地址的说明**，属于未公开功能。
- 另一种做法是中介在 App 里输入自己的 Rex 账号密码换 token（文档提到交互式应用可用 token 做会话）：只存 token 不存密码，token 最长 1 周、有活动自动续期。但 `login` 没有 MFA 参数，沙箱 200 个用户中 93 个开了 MFA，**开 MFA 的中介很可能无法这样登录**（未证实）；且把 Rex 密码交给第三方 App 需要中介同意。
- 限额按账户计，所以每人授权也**不会增加额度**，仍共享 3,600。
- 目前按“决策 2”：集成用户统一写入 + 正文署名 + 负责人字段，不依赖此项。要问 Rex：OAuth 是否开放给集成商、怎么申请 client、记录创建人是否显示本人。

**正式账户 758 的数据结构（Q2/Q4）**
- 需要只读跑一次，内容：总数（房产/联系人/挂牌/用户，约 6 次调用）→ 关系类型实际取值 → 负责人字段分布与用户对应 → 已有自定义字段/标签/邮件列表 → 集成用户在 758 的权限（能否建 webhook / 自定义字段）。
- 只存数量和 id，不落个人信息；调用量按沙箱比例约 150–300 次，占当小时额度 5–10%，会在 Rex 审计里留下集成用户的读取记录。需要业务方 / Ping 同意后执行。

## 13. 补测（2026-11-17，沙箱 2057，测试记录已清理）

对照 cc.saulxj.ggff.net/rex-api-requirements 清单，补测之前没调用过的接口。逐项结果已更新到该页面（“沙箱实测”标签页：通过 34、有注意事项 16、未通过 2、未能测试 6）。

| 接口 | 结果 | 要点 |
|---|---|---|
| `UserProfile::read` | ✅ | 返回集成用户资料 |
| `Contacts::findMatchingContacts` | ✅ | 电话 / 邮箱 / 姓名都返回 `verdict:"unique"`；**`+61400…` 与 `0400…` 视为同一号码**，邮箱不区分大小写 → 录入查重用它，而不是 `findPossibleDuplicates` |
| `Leads::read` / `update` | ✅ | |
| `Leads::toggleCompletionState` | ✅ 有坑 | 没有 `lead_type` 时报 “You must specify a lead type”；建 lead 时带 `lead_type:{id:"general"/"appraisal_request"/"listing_enquiry"}` |
| `Reminders::update` | ✅ | `data:{id, reminder_description, reminder_date}` |
| `Appraisals::search` | ✅ | 沙箱 408 条 |
| `ListingEvents::search` | ✅ | 不加条件 675 条，字段 event_type / event_date / event_time / event_timezone / listing |
| `MatchProfiles::create` / `update` / `search` | ✅ | `data:{contact_id, profile_name, category}`；删除 purge |
| `ProspectingMap::searchPropertyPinsInArea` | ✅ | `{polygon: GeoJSON（经度在前）}` → `{rows:[{id,lat,lng,status,is_ai_prospect}], truncated}`；集成用户可用 |
| `CalendarAvailabilities::getAvailability` | ✅ | `{user_ids, times:[{from,to}]}`；不带时区按 UTC |
| `CalendarEvents::create` | ❌ | `calendar_id`（`Calendars::search` 里用户的默认日历 uuid）+ `starts_at/ends_at:{date_time, timezone}` 通过校验，但 **Rex 返回 HTTP 500 空响应**；`CalendarEvents::search` 也报 Rex 内部错误（`GuzzleHttp\Promise\rejection_for()` 未定义）。要报给 Rex |

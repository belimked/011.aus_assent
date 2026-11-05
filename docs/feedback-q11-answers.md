# 手机 App 反馈第 11 节 · 问题回答与解决方案

> 对应：`agentCollect_手机App反馈_中文版.pdf` 第 11 节「需要 Kevin 回答的问题」，以及第 10 节中请 Kevin 确认的 DNCR 订阅问题
> 依据：`Rex_API_Integration_Guide.docx`、`.tmp/rexdocs/`（Rex API 定义，共 291 个服务）、`docs/rex-integration.md`
> 状态：草案（2026-11-05）

**状态标记**
- ✅ 文档里已有答案
- 🔍 方向已定，需要在 Rex 测试账号上验证，或向 Rex API 团队确认
- 🌐 需要外部调研或比价，Rex 文档无法回答

**重要前提：** 所有 Rex 结论都来自 API 定义，**尚未在真实账号上调用验证**。拿到测试账号后，先跑一遍第 6 节的验证清单。

---

## 1. Rex

### Rex Q1 · Rex API 能读写哪些字段？Rex 没有的字段存在哪里？　✅ / 🔍

**回答：** 需求里的数据，Rex 基本都有对应的服务或字段：

| 需求 | Rex 服务 / 字段 |
|---|---|
| 联系人、手机、邮箱 | `Contacts`：`contact_phones`、`phone_primary_sms`、`email_address` |
| 备注 | `Notes.create` / `search` |
| 标签 | `contact_tags`；标签列表由 `AdminTags` 管理 |
| 关联关系 | `contact_relationships.relationship_type`；`AdminPropertyRelationshipTypes` 可以新建和合并类型 |
| 提醒、待办 | `Reminders.create`、`getRemindeeList`（指派同事）、`toggleCompletionState`、`defer` |
| 预约估价 | `Appraisals.create` + `CalendarEvents.create` |
| 中介空闲时段 | `CalendarAvailabilities.getAvailability` |
| 确认短信、前一天提醒 | `CalendarEvents.sendConfirmationMessages` / `scheduleAlertMessages` |
| 买家需求档案 | `MatchProfiles`（价格、区、卧室/卫生间/车位/土地面积、features、tags）；`matchAgainstListings` |
| 退订和联系渠道 | `contact_by_sms/phone/email/letter`、`is_dnd` |
| 邮件列表订阅 | `contact_mailing_lists`：`subscription_status`、`subscribe_timestamp`、`unsubscribe_timestamp/reason` |
| Last updated by | `system_modified_user`、`system_modtime`（负责人是另一个字段 `system_owner_user`） |
| 最后联系 | `last_contacted_date` |
| 兴趣度 | `interest_level`（hot / warm / cold） |

**Rex 没有对应字段的数据**（贷款状态、同意方式和日期、DNCR 核查日期和结果、卖房原因等）：

**解决方案：**
1. 用 `AdminCustomFields.createField` 在联系人模块建自定义字段，由 agentCollect 在接入时自动创建，或者由管理员一次性创建。
2. 读写用 `CustomFields.getValuesKeyedByFieldName` / `setFieldValues`。
3. 原则不变：**Rex 是唯一数据源**，agentCollect 不长期保存联系人资料的副本（`rex-integration.md` D4）。
4. 只有 Rex 存不了的运营数据才放在我们自己的库里：屏蔽名单、DNCR 核查记录的完整日志、审计日志、同步队列。

🔍 **待验证：** 自定义字段写入是否需要额外权限；标签是写 `contact_tags`，还是要通过 `AdminTags` 关联（设计文档里原本标为待确认）。

---

### Rex Q2 · 能否按手机号搜索并合并重复记录？匹配到没有权限的联系人时显示什么？　✅ / 🔍

**回答：**
- **查重：** `Contacts.findMatchingContacts(phones[], email, name_first, name_last…)` 一次调用就返回结论，手机号会自动展开成多种存储格式去匹配。
- **合并：** `Dedupe.findPossibleDuplicates` 找候选，`Dedupe.combineRecords` 执行合并。
- **权限：** 参数 `min_user_relationship_to_contact`（none / read / update / owner）的默认值是 `read`，**调用者没有读权限的联系人不会出现在结果里**，也不影响查重结论。

**这会影响整体架构：** 现有设计 D1 是"只用一个 Rex 集成账号"，这样 Rex 只会按集成账号的权限过滤，无法做到"只显示中介本人有权限的联系人"。

**解决方案：**
1. **改 D1：每位中介用自己的 Rex 身份调用。** Rex 支持 `Authentication.loginWithOAuthAccessToken`，中介在 App 里连接一次 Rex 账号，Rex 的权限和 closed database 设置就自动生效。
2. 集成账号只保留给后台任务：夜间对账、webhook、Pricefinder 同步。
3. **匹配到无权限的联系人时：** 先用中介身份查（`min_user_relationship_to_contact=read`）。没有结果时，后端再用集成账号以 `none` 查一次，只返回"存在"这个结论，不返回任何字段。界面显示：**"该号码已是公司其他同事的联系人，详情不可见。新建会产生重复，请联系办公室。"** 同时禁止新建，提供"请求访问"按钮，通知该联系人的负责人。
4. **合并：** 只在中介点确认后调用 `combineRecords`，界面左右对比，选择保留哪边的值；合并操作写入审计日志。

🔍 **待确认：** OAuth 登录时，3,600 次限额是否仍按公司计算（见 Q3）；`combineRecords` 需要什么权限。

---

### Rex Q3 · App 每小时用掉 3,600 次请求中的多少？离线同步和重试如何控制在限额内？　✅ / 🔍

**回答：**
- 限额是**每家中介公司每小时 3,600 次**，目前是软限制，Rex 说以后会严格执行。不同公司之间互不占用。
- Draft 1 只是原型，没有连接 Rex，**现在的用量是 0**。估算见 `rex-integration.md` 第 10 节，要按实际人数更新。

**解决方案：**
1. **后端统一限流：** 所有 Rex 调用都经过后端的连接器，按公司做令牌桶，预留 20% 给交互请求（打开卡片、查重）。
2. **增量同步：** 用 `result_format: "etags"` 拉取变化（Rex 说这类请求很便宜），只读取有变化的记录。
3. **Webhook 代替轮询：** 联系人、备注、房产的变化由 Rex 推送过来；需要中介授予 `Manage Webhooks` 权限。webhook 被标记为不健康期间的事件会丢失，所以保留夜间对账。
4. **离线队列：** 恢复网络后不一次性提交，按优先级（新建联系人 > 备注 > 标签）限速提交。每条记录带幂等键，避免重复写入。
5. **失败重试：** 遇到 429/5xx 按指数退避重试；一直失败的进入"待同步"列表，中介能看到（对应反馈"Rex 同步失败"一项）。
6. 尽量批量调用，`BatchRequests` 是否只算一次请求要请 Rex 确认（设计文档 Q8）。

🔍 **待确认：** 每位中介用 OAuth 调用时如何计数；`BatchRequests` 的计数方式。

---

## 2. 短信

### 短信 Q1 · 用自己手机的发送队列，能否把每条已发短信记录到 Rex？　✅（有限制）

**回答：** 能记录，但**只能记成"已打开发送"，无法确认对方是否真的发出**：
- iPhone：App 只能预先填好短信并打开发送界面（`MFMessageComposeViewController`）。系统会告诉 App 用户是点了发送还是取消，**但不提供短信内容和送达状态**。
- Android：用系统短信 App（`ACTION_SENDTO`）时同样拿不到结果。只有把 agentCollect 设为默认短信 App，或者用网关模式，才能确认发送，但运营商条款和 Google Play 政策有限制。
- Rex 里**没有找到**公开的"记一条短信活动"接口：`MessageSends` 只能查询，`MailMerge` 是 Rex 自己发短信用的（每条 10 分）。

**解决方案：**
1. 发送队列：每条短信预先填好，中介点发送 → 返回 App 后自动打开下一条。
2. iPhone 上根据系统返回的"已发送 / 已取消"决定是否记录；Android 上中介回到 App 时默认记为已发送，可撤销。
3. 记录方式：`Notes.create`（备注写"SMS sent: [模板名] [正文]"）+ 更新 `last_contacted_date`。
4. 群发营销不走这个渠道，走短信 Q2 的供应商。

🔍 **待确认：** 问 Rex 是否有公开接口能把短信记成活动（显示在联系人的通讯记录里），而不只是备注。

---

### 短信 Q2 · 大批量群发选哪家？回复和 STOP 能否回传到 agentCollect 和 Rex？　🌐 / ✅

**回答：**
- **选哪家：** Rex 文档回答不了，需要比价（Mobile Message、Cellcast 等）。
- **回传到 agentCollect：可以**，前提是供应商支持回复回调（inbound webhook）。
- **回传到 Rex：可以写入退订状态，但写不进 Rex 的短信记录：**

| 要做的 | Rex 接口 | 结论 |
|---|---|---|
| 标记不接收短信 | `Contacts.update`：`contact_by_sms = false`；全部渠道禁止用 `is_dnd` | ✅ |
| 记录退订原因和时间 | `contact_mailing_lists.unsubscribe_*` 只适用于邮件列表 | 🟡 仅邮件 |
| 记录"收到 STOP" | `Notes.create` | ✅ 作为备注 |
| 回复显示为 Rex 里的短信往来 | 没有对应接口 | ❌ |

**解决方案：**
1. **选供应商的必备条件：** 回复回调、自动识别 STOP、专用回复号码（用字母发件人名对方无法回复）、按段计费、退订免费、数据存放在澳洲。
2. **STOP 处理流程：** 供应商回调 → agentCollect 识别 STOP 和明显的退订用语 → 加入全公司屏蔽名单 → Rex 写 `contact_by_sms=false` 和一条备注 → 发送确认短信 → 通知中介 → 卡片显示红色"OPTED OUT OF TEXTS"横幅，停用 Text 按钮。
3. **屏蔽名单以 agentCollect 为准：** Rex 自己发出的短信，回复由 Rex 处理，agentCollect 拿不到。所以以 agentCollect 的屏蔽名单为主，同步到 Rex 的 `contact_by_sms`，Rex 那边群发时也能跳过。
4. 每次群发前在后端按屏蔽名单、`contact_by_sms`、联系同意再过滤一次，并显示费用。
5. 供应商做成适配层，以后可以换供应商。

🌐 **待办：** 做一张供应商对比表（价格、回复回调、STOP 处理、发件号码、数据存放地）。
🔍 **待确认：** Rex `MailMerge` 群发短信时，会不会自动跳过 `contact_by_sms=false` 的联系人。

---

### 短信 Q3 · 房产网站询盘现在怎么进 Rex？agentCollect 能否自动获取，并接收从手机分享来的线索？　✅ / 🔍

**回答：**
- REA、Domain 的询盘一般作为 `Leads` 进入 Rex（Rex 的 lead 邮箱解析门户邮件）。`Leads.getAllowedSourcesForAccount` 可以查看公司开启了哪些来源。eplace.com.au（官网）是不是同一个流程，要问 Ping。
- 每位用户都有 Rex 的 lead 邮箱和 email dropbox 地址（`Leads.getLeadDropboxAddressForUser`、`EmailDropbox.getEmailDropboxAddressForUser`），转发进去的邮件会自动关联到联系人（`EmailDropbox.autoAttachToContacts`）。

**解决方案：**
1. **自动获取：** 订阅 Leads 的 webhook（不可用时定时 `Leads.search`），新线索进入 agentCollect 后走反馈第 3 节的流程：识别 → 匹配 → 显示历史 → 追加备注 → 建议下一步。
2. **手机分享：** 原生 App 注册为系统分享目标（iOS Share Extension / Android Intent），接收文字、图片和截图 → OCR 加 LLM 提取姓名、手机、邮箱、房产、来源和需求 → 走同一流程。
3. **转发邮件和短信：** 给每位中介一个 agentCollect 收件地址，统一解析；同时可以抄送 Rex 的 dropbox，Rex 里也会有原件。
4. **语音：** 和录入共用语音服务。

🔍 **待确认：** 用测试账号看一条门户询盘在 Rex 里的样子（`Leads.read`），确认能拿到哪些字段；Leads 有没有 webhook 事件。

---

## 3. 数据

### 数据 Q1 · 能否用每位中介自己的 Pricefinder 账号做整个区域的批量更新和成交/挂牌实时提醒？每天能刷新多少处？成交后多久能看到？　🔍 / 🌐

**回答：**
- **Rex 自带的 Pricefinder/RP Data 集成只能逐个房产查询**（`ThirdPartyServicePriceFinderPropertyV2.getPropertyDetails`、`ThirdPartyServiceCoreLogicProperty.getPropertyDetails`），不能整片区域批量刷新。
- Rex 自己的勘察地图有"按多边形查房产，并叠加 RP Data 里不在 Rex 的房产"（`ProspectingMap.searchPropertyPinsInAreaWithThirdParty`），用的是中介自己的 RP Data 登录。但这些方法看起来是 Rex 自己 App 的内部接口，**是否允许第三方调用要确认**。
- Properties 里有 `oab.*`（其他中介公司的挂牌，含公司、状态、价格），可以用来显示地图上的琥珀色"其他中介公司在售"。
- 每天能刷新多少、成交后多久能看到，取决于 Pricefinder API 的配额和数据更新频率，**Rex 文档里没有**。

**解决方案：**
1. **第一阶段：** 用 Rex 自带的逐个查询，在打开房产卡片时显示业主和购买日期；不做批量。
2. **第二阶段：** 直接接入 Pricefinder API，每位中介用自己的账号授权（OAuth），后端按区域每周批量刷新一次，新画区域时立即刷新；有变动的进入"等待批准"。
3. 数据提供商做成插件接口（Pricefinder / RP Data / 以后 PropTrack）。
4. 如果 Rex 确认 `ProspectingMap` 可以给第三方调用，就优先用它，开发量最小。

🌐 **待办：** 向 Pricefinder 申请 API 文档，确认批量查询的配额、成交数据延迟、是否有事件推送。
🔍 **待确认：** 问 Rex 是否允许第三方调用 `ProspectingMap.*`。

---

### 数据 Q2 · 地图的准确地址和地块边界用哪个数据源？　🌐（建议方案）

**回答：** Draft 1 的问题在于房产点是按街道线段生成的，不是真实地址。建议用开放数据：

| 用途 | 数据源 | 说明 |
|---|---|---|
| 地址和坐标 | **G-NAF**（Geoscape，开放数据） | 全澳官方地址库，坐标落在地块内 |
| 地块边界 | **QLD DCDB**（昆州地籍数据，QSpatial 开放下载） | 含 Lot/Plan，可以和 G-NAF 关联 |
| 卫星图 | QLD Globe 影像服务 / Esri World Imagery / Nearmap（付费，分辨率最高） | 先用免费的，需要更清晰再考虑 Nearmap |
| 街道底图 | OpenStreetMap（现有） | 门牌号由 G-NAF 叠加，不依赖底图 |

**解决方案：**
1. 后端导入 Sunnybank 一带的 G-NAF 地址和 DCDB 地块（表结构已在 `rex-integration.md` 3.5 节设计好）。
2. 地图按地块画边界，标记放在地块内；放大后显示门牌号。
3. 地址和 Rex 房产自动匹配（规范化地址后精确匹配，否则模糊匹配），匹配不上的进入管理员审核队列；匹配后可以用 `Properties.setGeoLocation` 修正 Rex 里的坐标。
4. 单元楼（strata）按地块聚合，点开列出各单元。

🌐 **待确认：** 各数据源的使用许可（G-NAF 和 DCDB 是 CC BY，需要署名；卫星影像的商用条款）；DCDB 的更新频率。

---

## 4. App

### App Q1 · 能否离线使用、之后再同步？能否在 Android 上禁止截屏，在 iPhone 上检测截屏？　✅（技术上可行）

**回答：**
- **离线：** 可以，但需要原生 App 或者 Capacitor 这类壳。纯网页的离线存储会被系统清理，不可靠。
- **Android 禁止截屏：** 可以，`FLAG_SECURE` 会禁止截屏和录屏，最近任务列表里也显示空白。
- **iPhone：** **无法禁止截屏**，只能检测（`userDidTakeScreenshotNotification`），以及检测录屏和投屏（`UIScreen.isCaptured`），检测到录屏时可以遮住界面。

**解决方案：**
1. **App 方案：** 用 Capacitor 包装现有的 Vue 代码，复用最多；原生能力（分享入口、截屏控制、拨号、短信、推送）用插件实现。这个决定要先定下来，它会影响多项第一阶段的功能。
2. **离线：** 本地用 SQLite 加密存储；录入先存手机，显示"等待同步 N 条"；恢复网络后按 Rex Q3 的方式限速提交，带幂等键；缓存 24 小时过期（离线也生效）。
3. **截屏：** Android 开启 `FLAG_SECURE`；iPhone 检测到截屏就记录审计日志（谁、何时、哪个页面）并提醒管理员，检测到录屏时遮住联系方式；两端都保留姓名加日期的水印。

---

### App Q2 · 哪个语音转文字服务对英语、普通话和粤语都处理得好？每分钟多少钱？　🌐

**回答：** 需要实测。候选服务都支持澳洲英语、普通话和粤语，但**中英混说**（例如"明年 maybe sell"）的效果差别很大，价格也要以官网为准：

| 候选 | 特点 |
|---|---|
| Azure Speech | 支持 en-AU、zh-CN、zh-HK（粤语），有澳洲数据中心 |
| Google Cloud Speech（Chirp） | 多语言模型，有澳洲区域 |
| OpenAI 转写（Whisper / gpt-4o-transcribe） | 中英混说效果通常较好，数据存放地要确认 |

**解决方案：**
1. 用真实场景录 20–30 段测试音频（门口嘈杂环境、中英混说、粤语加英文、澳洲口音），对比几家的准确率、延迟和每分钟价格。
2. 选出结果后，转写由后端调用（密钥不放在手机上），之后交给 LLM 拆分字段。
3. 优先选可以把数据存在澳洲、服务商不保留音频的方案。
4. "Talk to me"翻译器和录入共用同一个服务。

🌐 **待办：** 做语音转写测试和比价表。

---

## 5. 第 10 节：DNCR 订阅能否所有公司共用？　🌐

**回答：** Rex 文档无法回答，需要查 ACMA 的条款，或者直接问 ACMA。DNCR 订阅的主体是"代表电话营销方核查号码的组织"，agentCollect 作为服务商代各家公司核查是否允许，以及费用怎么分摊，都要确认。

**解决方案：**
1. 先确认条款。允许的话，由 agentCollect 统一订阅，按公司记录核查日志。
2. 按反馈的设计：超过 30 天的号码每晚批量重新核查，点 Call 时实时核查；结果和日期存在 Rex 自定义字段里作为证据（见 Rex Q1）。

---

## 6. 拿到 Rex 测试账号后的验证清单

| # | 验证项 | 对应问题 |
|---|---|---|
| V1 | 用 OAuth 以中介身份登录，确认权限过滤和 3,600 次计数方式 | Rex Q2、Q3 |
| V2 | `findMatchingContacts` 在有权限和无权限两种情况下的返回 | Rex Q2 |
| V3 | 写入标签、关联关系、自定义字段 | Rex Q1 |
| V4 | `contact_by_sms=false` 写入后，Rex 群发是否跳过 | 短信 Q2 |
| V5 | `Leads.read` 一条门户询盘，看字段；Leads 有没有 webhook 事件 | 短信 Q3 |
| V6 | 第三方能否调用 `ProspectingMap.*` | 数据 Q1 |
| V7 | `BatchRequests` 是否只算一次请求 | Rex Q3 |
| V8 | 有没有能把短信记成活动的接口 | 短信 Q1 |

## 7. 需要外部调研的

| # | 事项 | 对应问题 |
|---|---|---|
| R1 | 短信供应商对比表 | 短信 Q2 |
| R2 | Pricefinder API 配额、数据延迟、事件推送 | 数据 Q1 |
| R3 | G-NAF、DCDB、卫星影像的许可 | 数据 Q2 |
| R4 | 语音转写实测和比价 | App Q2 |
| R5 | DNCR 订阅能否由服务商共用 | 第 10 节 |

## 8. 需要先做的决策

1. **D1 改为每位中介用自己的 Rex 身份（OAuth）调用**，集成账号只用于后台任务（Rex Q2）。
2. **App 用 Capacitor 包装现有 Vue 代码**，还是用 React Native 重写（App Q1）。

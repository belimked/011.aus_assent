# 市场数据（Pricefinder）

Rex 不提供这些数据。Pricefinder 等数据商提供的成交、挂牌和登记业主信息，属于**授权给中介的第三方数据**。业主姓名是个人信息。

**使用许可（2026-11-18 已确认）**：客户的 Pricefinder 合同允许把导出数据导入本系统缓存使用，但只能在内部使用：
- 只给负责该片区的中介和 admin 查看；
- 不提供导出，不对外分发；
- 管理后台没有整表导出。

## 数据流

1. 管理员上传文件（管理后台或 CLI），逐行校验，不合格的行跳过并记录。
2. 按地块号（lot/plan）或地址匹配房产。
3. 业主和最近成交写入独立的 `property_ownership` 表。
4. 需要提醒的成交，给片区覆盖该房产的每个中介生成一条 `market_items`。中介通过现有的 `/sync` 增量下载，Yes/No 仍走原来的确认流程。

其他规则：
- 没匹配上房产的成交仍保存在 `market_events`，但不触发提醒。
- 每次正式导入都记录 sha256、错误行和 audit_log；同一个文件不能导入两次。
- dry-run 不写数据库，日志也不写。

## 文件格式

- 支持 Pricefinder 导出的 **.xls**（Address Search，BIFF8）、**.xlsx** 和 **.csv**（UTF-8，可含 BOM），最大 20 MB。
- 按文件内容判断格式，不看扩展名；只读第一个工作表。
- 日期接受 `YYYY-MM-DD` 和澳洲格式 `DD/MM/YYYY`，统一存成 ISO 格式。
- 价格允许带 `$` 和逗号。
- 文件末尾的版权、免责声明行和空行计入 ignored，不算错误。

### 列名别名

匹配列名时不区分大小写、空格、下划线和标点。

| 字段 | 接受的列名 |
|---|---|
| address | Address, Street Address, Property Address, **Street** |
| lot_plan | Lot/Plan, Lot Plan, **Legal Description** |
| suburb / postcode | Suburb, Locality / Postcode, Post Code |
| price | Sale Price, Price, **Last Sale** |
| date | Sale Date, Contract Date, Listed Date, Event Date, **Last Sale Date** |
| owners | Owner Name(s), Owner Names, Owner Name, **Current Owners** |
| beds / baths | Beds, Bedrooms / Baths, Bathrooms |
| cars | Car, Cars, Parking, **Car Parks** |
| land | Land Size, Land, **Area** |
| provider_id | Property ID, Listing ID, Sale ID, Record ID, Provider ID, **PDS ID** |

- 可以用 `server/market-mappings/pricefinder.json` 或 `/etc/axleo/market-mappings/pricefinder.json` 按字段配置别名数组，后者覆盖前者。
- 地块号会统一写法：`L7 RP111842`、`Lot 7 RP 111842`、`7/RP111842` 都视为同一块地。

## 快照文件（Pricefinder Address Search 导出）

**识别条件**：文件有 `Current Owners` 和 `Last Sale Date` 两列，并且没有 Kind 列。识别为快照后，每一行代表一套房产的现状，处理方式如下：

- **更新业主和最近成交**：写入 `property_ownership`。如果库里已有更新的成交日期，不会被旧日期覆盖。
- **记录成交**：有 Last Sale Date 的行，写一条 `sale` 事件，ID 为 `PDS ID:成交日期`。重复导入不会产生重复记录。
- **提醒中介**：两个条件同时满足才提醒：
  - 成交日期在**近 90 天内**（按布里斯班日期算）；
  - 成交日期比库里原来的记录新。

  历史成交只更新数据，不提醒。

不是快照的文件（例如 Kind 列为 sale/listing/withdrawn/ownership 的 CSV），仍按原来的“一行一条事件”逻辑处理。

## 隐私

- `market_events.raw` 只保存白名单里的非个人信息列，包括：
  - 地址、区、邮编、地块号
  - 面积、卧室、浴室、车位
  - 土地用途、分区、估价
  - 最近成交价、日期、类型
  - PDS ID、Load Date、楼名
- **业主通信地址（Current Owners Address）、卖方（Vendor Names）和买方（Purchaser Names）一律不保存**。
- 业主姓名只存在 `property_ownership.owner_names`。
- 业主通信地址只用来计算一个是/否标记 `absenteeOwner`：业主不住在这套房，可能是投资房。这个标记存在 `property_ownership.absentee_owner`。
- 片区覆盖该房产的中介和 admin 能看到 `market.owners` 和 `market.absenteeOwner`。其他中介只能看到：
  - `market.lastSalePrice`、`market.lastSaleDate`
  - `market.source`、`market.updatedAt`
- 含个人信息的导出文件不要放进仓库或公开目录。仓库根目录的 `export_to_excel_*.xls` 已加入 `.gitignore`。

## 操作

```
axleo market-import <file> --provider pricefinder --kind auto --dry-run   # 先核对数量
axleo market-import <file> --provider pricefinder --kind auto             # 正式导入
```

- 也可以由管理员在 `/admin/` → Market data 上传，同样可以选择 dry-run。
- 导入历史：`GET /api/v1/admin/market/imports`。
- 单次导入详情（前 50 条错误和未匹配地址）：`GET /api/v1/admin/market/imports/{id}`。
- 以上接口只有 admin 能调用。

## 可见性与片区

- 片区支持三种形式：已保存的多边形、圆心加半径（米）、suburb（`suburb`/`suburbs`）。
- 没有配置任何片区时，不会有提醒，也看不到业主姓名。
- 移动端的 `patch.suburbs` 目前只存在手机本地，没有上传到后端。要按 suburb 监控，需要在服务端 zones 里保存带 `suburb`/`suburbs` 的记录。

## 以后接 Pricefinder API

`axleo market-pull` 目前只提示“未配置/未实现”。接 API 需要中介提供：
- API 文档和真实地址；
- 认证方式；
- 增量（cursor/since）语义；
- 分页和限流规则；
- 字段样例。

密钥只放在 `/etc/axleo/env` 的 `AXLEO_PRICEFINDER_*` 里。没拿到文档之前，不猜 URL，也不自动请求。

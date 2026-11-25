# 市场数据（RP Data / CoreLogic）

Rex 不提供市场成交和登记业主数据。默认来源为 RP Data 文件导入；Pricefinder 历史格式仍可通过 `--provider pricefinder` 导入，见 [pricefinder-import.md](pricefinder-import.md)。第三方数据含个人信息，不得提交仓库、公开分发或整表导出。

**许可待确认**：CoreLogic 的许可是否与已确认的 Pricefinder 合同相同（仅内部使用、按片区授权、不对外导出）？未经确认不得在生产环境导入。QLD 业主姓名不得用于直接营销。只给覆盖房产片区的中介或 admin 下发业主与卖家姓名；其他用户仅能看到非个人信息。

## 导入

支持首个工作表的 xlsx、xls 和 UTF-8 CSV（含 BOM）；上限 100 MB（上传和 CLI），不导入嵌入的缩略图。自动在前 20 行寻找表头，前言计入 ignored。缺值 `-` 视为空。价格可含 `$`、逗号；日期接受 `YYYY-MM-DD`、`DD/MM/YYYY`、`10 Jul 2016` 和 Excel 日期序列号。RP Data 列映射随构建从 `server/market-mappings/rpdata.json` **嵌入可执行文件**，运行时不依赖当前工作目录，也无需在服务器部署该目录。Pricefinder 的 `server/market-mappings/pricefinder.json` 同样内嵌。需要定制时，可用 `/etc/axleo/market-mappings/<provider>.json` 按字段覆盖内置映射：

| 标准字段 | RP Data 列 |
|---|---|
| address / suburb / postcode | Street Address / Suburb / Postcode |
| lot_plan | Parcel Details |
| price / date | Sale Price / Sale Date |
| agency / agent | Agency / Agent |
| beds / baths / cars | Bed / Bath / Car |
| land | Land Size (m²) |
| owners | Owner 1/2/3 Name（非空用 ` & ` 连接） |
| provider_id | Open in RPData 链接最后的数字（缺失则回退哈希） |

每一行按房产快照处理：匹配唯一地块或地址，更新 `property_ownership`；有成交日期则记录 `sale` 事件，ID 为 `<CoreLogic ID>:<ISO 日期>`，同一事件幂等。旧成交不覆盖新成交；只有比库中日期新且在布里斯班近 90 天内的成交才提醒片区中介。未匹配的事件仍存档但不提醒。正式导入记录 SHA256 和 audit；相同文件不能再次导入，dry-run 不写库。

附加属性写入 `properties.data.rpdataAttributes`，包括房型、卧浴车、土地/建筑面积、年份、用途/分区/地块、最近成交、经纪人、CoreLogic URL 和 ID、updatedAt、可解析的 exportedAt，以及业主/卖家姓名。不会覆盖 Rex 原字段；这些属性供 UI 单独展示。`market_events.raw` 只保留非个人信息白名单列；`rpdataAttributes.owners` 和 `.vendors` 及 `property_ownership.owner_names` 对无 zone 授权者从同步和单条 API 响应中删除。不要将原文件、未匹配地址或姓名写入公开日志。

测试服务器的备份、上传、dry-run、正式导入及源文件清理步骤见 [rpdata-import.md](rpdata-import.md)。测试用的合成导出文件及清理方法见 [rpdata-synthetic.md](rpdata-synthetic.md)。管理员也可在 `/admin/` → Market data 选择 RP Data / Pricefinder 上传，先 dry-run 核对行数及匹配率。Pricefinder 原有快照仍按其旧规则运行；API pull 尚未实现。

# RP Data 合成测试数据（仅测试服务器）

**只用于测试服务器，生产环境禁止导入。** 合成文件不含任何真实个人信息：地址和地块来自公开的 QLD 地块数据（`web/public/parcels.json`，© State of Queensland，CC BY 4.0）；业主、卖家、经纪人、中介、价格和日期都是虚构的。所有人名和中介名都以 ` (Test)` 结尾。生成器只依据导入器代码（`server/market_rp.go`、`market.go`、`market-mappings/rpdata.json`）和真实导出的表头结构编写，不读取真实数据行。

## 生成

生成器是 Go 小工具 `server/cmd/gen-fake-rpdata`。仓库的 Node 项目里没有 xlsx 依赖，而 server 已经在用 excelize，导入器也用它读文件，所以用 Go 最简单。它不在发布构建里（`release.sh` 只编译 `server/.`）。

```sh
cd server
go run ./cmd/gen-fake-rpdata --out ../.tmp/fakerp/out
```

输出（生成的 xlsx 不要提交）：

- `out/initial/SYNTHETIC_<Suburb>_RPData_propertyExport_<时间戳>.xlsx`：初始批，每区一个文件。结构与 CoreLogic 导出相同：只有一个 `Sheet1`；第 1 行是 `Search String | <Suburb> QLD <postcode>`，第 2 行空，第 3 行是 30 列表头，第 4 行起是数据。邮编格式为 `#0000`，土地和建筑面积格式为 `#,###,###,##0`，价格和日期是文本（`$1,250,000`、`10 Jul 2016`），缺值写 `-`。`Parcel Details` 写成 `Lot 434 RP900761`。文件里没有图片或超链接，`Open in RPData` 列为空，因此 `withCorelogicId=0`，事件 ID 退回地址+日期哈希。
- `out/incremental/…`：增量批。每区从初始批里抽 3–5 套房产，生成最近 7 天内的新成交：价格比上一次更高，新业主是新的虚构名字，卖家是原来的业主。导入后会给片区经纪人发成交提醒。

文件只覆盖四个区：Sunnybank、Sunnybank Hills、Calamvale、Parkinson。房产按整条街道抽取：街道随机排序后整条加入，最后一条街截取连续的一段。Sunnybank 一定包含 Gager St、Feltwell St、Boorman St、Lister St。每个地块（lot/plan）最多输出一行；同一地块上有多个地址时，写不带单元号的街道地址。

| 参数 | 默认值 | 说明 |
|---|---|---|
| `--ratio` | `0.10` | 行数占真实导出行数的比例（约 3,860 / 6,650 / 7,370 / 4,010 → 386 / 665 / 737 / 401，合计 2,189） |
| `--count-per-suburb` | 0 | 每区固定行数，设置后覆盖 `--ratio` |
| `--seed` | `20261125` | 随机种子。种子和 `--today` 相同，生成的文件逐字节相同 |
| `--today` | 布里斯班今天 | 参考日期 `YYYY-MM-DD`。初始批导出时间是 today−8 天，增量批是 today |
| `--incremental-min/-max` | 3 / 5 | 增量批每区的新成交条数 |
| `--recent-initial` | 0 | 初始批中每区放进最近 90 天的成交条数。默认 0，初始批不会触发提醒 |
| `--no-sale-ratio` | 0.15 | 没有成交记录的房产比例 |
| `--suburbs` | 全部 | 逗号分隔，只生成其中几个区 |
| `--parcels` | `../web/public/parcels.json` | 地块来源 |

成交价按当前价值生成：独立屋 80–200 万澳元，单元房 38–75 万，联排 56–90 万；较早的成交按每年约 5% 折算。成交日期分布在近 15 年内，到 today−91 天为止。约 15% 的房产没有成交记录。

**规模提醒**：有 `rpdataAttributes` 的房产都会同步到手机。默认的 10% 约 2,200 套；不要按 `--ratio 1` 导入两万多套，否则 App 会卡顿。

## 导入与提醒

导入器没有“不发提醒”的开关。只有成交日期比库里新、并且在布里斯班近 90 天内，才会给片区经纪人发提醒。初始批默认把所有成交放在 90 天以外，所以导入时 `salesNotified=0`；增量批的成交都在最近 7 天内，每一条都会提醒。

```sh
axleo market-import --provider rpdata --dry-run '/data/axleo/.tmp/fakerp/initial/'*.xlsx   # 确认 synthetic=true、unmatched=0
axleo market-import --provider rpdata '/data/axleo/.tmp/fakerp/initial/'*.xlsx
axleo market-import --provider rpdata '/data/axleo/.tmp/fakerp/incremental/'*.xlsx      # 测试成交提醒
```

文件名以 `SYNTHETIC_` 开头时，导入报告里 `synthetic=true`，房产写入 `rpdataAttributes.synthetic="true"`；`market_imports.filename` 保留原文件名。

本地验证（seed + `import-parcels` 全量公开地块 + `market-zone mary@axleo.local`，`--today 2026-11-25`）：

- 初始批 2,189 行全部匹配（Sunnybank 329 行按地块、57 行按地址匹配；Sunnybank Hills 582/83；Calamvale 392/345；Parkinson 384/17），未匹配 0，`salesNotified=0`，共 1,877 条成交。
- 增量批 14 行全部匹配，`salesNotified=14`，生成 14 条提醒。

## 清理（正式导入真实数据前必须执行）

```sh
axleo market-purge-synthetic --dry-run   # 只统计
axleo market-purge-synthetic
```

这条命令在一个事务里完成以下操作：

1. 软删除合成导入产生的提醒（`market_items.deleted_at`），手机同步时会收到删除记录。
2. 删除对应的 `market_events` 和 `property_ownership`。
3. 删除带 `synthetic` 标记的 `rpdataAttributes`。
4. 删除 `SYNTHETIC_` 文件的 `market_imports`，之后同一文件可以重新导入。
5. 把同步重置点（`settings.sync_reset_seq`）设为当前位置，让手机下次拉取完整快照，不再显示已移出同步范围的房产。
6. 写一条审计记录（`audit_log`，entity=`market_imports`，id=`synthetic`）。

之后再跑一次 `--dry-run`，全部计数应为 0。命令不会修改 `properties.registered`：`import-parcels` 导入的地块原本就是 1。

同等的 SQL（仅供核对，优先用命令）：

```sql
-- S = SELECT id FROM market_imports WHERE substr(filename,1,10)='SYNTHETIC_'
UPDATE market_items SET deleted_at=:now,updated_at=:now,version=version+1 WHERE deleted_at IS NULL AND event_id IN (SELECT id FROM market_events WHERE import_id IN (S));
DELETE FROM market_events WHERE import_id IN (S);
DELETE FROM property_ownership WHERE import_id IN (S);
UPDATE properties SET data=json_remove(data,'$.rpdataAttributes'),updated_at=:now,version=version+1 WHERE json_extract(data,'$.rpdataAttributes.synthetic')='true';
DELETE FROM market_imports WHERE id IN (S);
```

**不要让合成数据和真实数据混在一起。** 如果在清理之前导入了真实文件，而真实成交比合成成交更早，导入器会保留较新的合成业主姓名，后续清理也会删掉这些房产的整行 ownership。所以必须先执行清理，再导入真实数据。

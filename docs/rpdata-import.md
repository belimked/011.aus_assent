# RP Data（CoreLogic）文件导入：测试服务器

**先确认 CoreLogic 合同是否允许内部缓存、按片区授权和不可导出。QLD 业主姓名不得用于直接营销。** 源文件含个人信息和嵌入图片，不放仓库、公开目录或日志。仅由有权限的管理员处理。

1. 停止服务、备份数据库与上传目录；记录快照位置，确认有可用空间。需要房产匹配时先按既有流程运行 `import-parcels` 和 `dedupe-parcels`；先各自 dry-run，再正式执行。
2. 将四区 xlsx 经受控通道上传至测试服务器受限目录（例如 `/data/axleo/.tmp/rpdata-in`），目录权限 0700、文件 0600，归 `axleo` 用户所有。不要写入 web 根目录。
3. 在测试服务器加载 `/etc/axleo/env` 并切换至 `/data/axleo`，先运行：
   `axleo market-import --provider rpdata --kind auto --dry-run /data/axleo/.tmp/rpdata-in/*.xlsx`
   核对 headerRow、rows、success、ignored、errors、matchedLot/matchedAddress、unmatched、withParcel/withOwners/withCorelogicId；未匹配地址报告只在受限环境内查看，不公开转发。dry-run 不写数据库。
4. 确认匹配率、日期、片区授权和近 90 天提醒条件符合预期，才运行同一命令去掉 `--dry-run`。重复导入同一文件 SHA256 会被拒绝；不同文件同一 CoreLogic ID 和成交日期会合并事件。检查 `/admin/` → Market data 的导入报告，以及手机房产页 OWNER 区的登记业主（按需读取，规则见 market-data.md“登记业主姓名”）。
5. 导入后删除服务器上的源 xlsx 和临时副本（包括失败文件），核对受限目录为空；保留只含统计的审计记录。异常时停止服务并依备份恢复，通知用户重新同步。

每个文件上限 100 MB。Excel 图片不导入；CLI 和 HTTP 都只读取首个工作表。文件名包含 `_YYYYMMDDHHMMSS` 时保存为 exportedAt。Pricefinder 历史导入仍可指定 `--provider pricefinder`。

测试服务器可先导入合成数据（`SYNTHETIC_*.xlsx`，见 [rpdata-synthetic.md](rpdata-synthetic.md)）；导入真实文件前必须先运行 `axleo market-purge-synthetic` 清除。生产环境禁止导入合成文件。

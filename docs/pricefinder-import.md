# QLD 地块与 Pricefinder 导入（65）

## 新增 Macgregor、Robertson、Runcorn、Kuraby、Eight Mile Plains

QLD Spatial LandParcelPropertyFramework 地址 layer 0 / 地块 layer 4（© State of Queensland，CC BY 4.0）。只下载新五区，保留在不入库的 `.tmp/parcels-new/`；两层的 Macgregor locality 大小写不同（地址为 `Macgregor`、地块为 `MacGregor`），下载工具已分别处理。若要从零重建九区，将两个目录的页面合并（文件名包含 suburb，不冲突）：

```sh
node tools/download-qld-parcels.mjs .tmp/parcels-new MACGREGOR ROBERTSON RUNCORN KURABY 'EIGHT MILE PLAINS'
node tools/download-qld-parcels.mjs .tmp/parcels-old SUNNYBANK 'SUNNYBANK HILLS' CALAMVALE PARKINSON
mkdir -p .tmp/parcels-all
cp .tmp/parcels-{old,new}/*.json .tmp/parcels-all/
node tools/build-sunnybank-data.mjs .tmp/parcels-all .tmp/all-mobile.js
node tools/build-sunnybank-mobile.mjs .tmp/all-mobile.js
cp mobile/src/data/sunnybank.json web/public/parcels.json
cp web/public/parcels.json server/web/parcels.json
```

生成时的地址点数（按 locality；全量 44,457 条，保留有地址的地块 38,714 条）：Sunnybank 4,262、Sunnybank Hills 7,382、Calamvale 7,411、Parkinson 4,498、Macgregor 2,487、Robertson 1,964、Runcorn 6,155、Kuraby 3,249、Eight Mile Plains 7,049。新五区原始页面合计 35,494,535 B。

新五区原始 GeoJSON 不入库，服务器上只需复制 `.tmp/parcels-new/` 中的 `addr_*.json` / `parcel_*.json` 至 `/data/axleo/.tmp/parcels-new/`（由 axleo 用户可读，目录不得混入旧区或其他 JSON）。升级服务和 App 后，在维护窗口先停服务、按下文备份；以 root 执行以下命令，先核对 dry-run，再批准正式导入：

```sh
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo import-parcels /data/axleo/.tmp/parcels-new --dry-run'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo import-parcels /data/axleo/.tmp/parcels-new'
```

已有的 `market-zone` 不会自动扩充旧 zone：核对并在后台编辑现存 zone 加入新五区，或在确认为新账号时运行 `axleo market-zone <email>` 创建九区 zone。请 Ping 另行导出五区的 RP Data / Pricefinder 业主资料并经审批后按对应导入流程处理；公共 QLD 数据不含业主信息。手机内置文件从 4,686,809 B 增至 8,730,782 B（1.86 倍，未超过三倍）；后台静态地块文件也更新。重新构建、发布 App 与后台资源、验证地图及同步性能，不能只导入数据库。

输入文件包含个人信息：**只在受限的 `.tmp` 中保存，不进 git、日志或测试夹具**。QLD Spatial LandParcelPropertyFramework 地址 layer 0、地块 layer 4（© State of Queensland，CC BY 4.0）；`tools/download-qld-parcels.mjs` 按区分页下载。Sunnybank Hills、Calamvale 的 Pricefinder 导出各受 5000 条上限约束：后续追加文件再导入，已导入文件按 SHA-256 去重。

本地准备（重新打包手机 App：`mobile/src/data/sunnybank.json` 包含九区地址和多边形；旧包不能显示新区背景灰点）。原始 API GeoJSON 放在 `.tmp`，**不要复制到版本库**：

```sh
node tools/download-qld-parcels.mjs .tmp/qld-raw
node tools/build-sunnybank-data.mjs .tmp/qld-raw .tmp/qld-mobile.js
node tools/build-sunnybank-mobile.mjs .tmp/qld-mobile.js
```

部署包含此版本的服务与新版 App 后，在 65 的维护窗口以 root 执行（假设文件安全传至 `/data/axleo/.tmp/{qld-raw,pf-in}`，仅 axleo 用户可读；用实际 Mary/John 邮箱替换示例邮箱）：

```sh
systemctl stop axleo
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo backup /data/axleo/backup'
# 若原始 GeoJSON 尚未传上来，在仓库/工具目录运行 node tools/download-qld-parcels.mjs /data/axleo/.tmp/qld-raw
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo import-parcels /data/axleo/.tmp/qld-raw --dry-run'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo import-parcels /data/axleo/.tmp/qld-raw'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo dedupe-parcels --dry-run'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo dedupe-parcels'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-import --provider pricefinder --kind ownership --dry-run /data/axleo/.tmp/pf-in/*.xls'
# 核对逐文件 matchedLot/matchedAddress/unmatched/misaligned/errors 和样例，再批准正式导入：
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-import --provider pricefinder --kind ownership /data/axleo/.tmp/pf-in/*.xls'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-zone mary@axleo.local; /opt/axleo/current/axleo market-zone john@axleo.local'
systemctl start axleo
```

`market-zone` 为指定账号幂等创建这四区的私有 zone，admin 本身有权限；不改其他账号权限。**先确认真实邮箱及 zone 归属**。顺序：import-parcels → dedupe-parcels → market-import。先按区、规范化门牌街道和单元找唯一业主房产（Rex/人工/非地块房产），再按 lotplan 提升旧地块或新建；同址多个旧地块不算歧义，已有不同 lotplan 的业主房产则跳过。只补空缺的地块号、坐标、面积、多边形，不改 Rex 字段。旧 seed 中无 source 但有 lotplan/lot 的记录也视作地块。St 仅作街道末尾 Street，Mt 仅作前缀 Mount；单元与整栋不混配。多个业主房产匹配同址才记 ambiguous。import-parcels 输出 added（新建）、promoted（旧地块提升）、linkedToExisting（业主房产补齐）、ambiguous、skipped（dry-run 同口径）；dedupe-parcels 输出 moved、ambiguous、skipped，迁移历史及无 source 的旧地块的 PF 资料、联系人和活动引用并软删除重复地块。先核对 dry-run 计数和样例。非 Rex 地块按 lotplan 幂等插入；只提升未注册的旧地块；已有 Rex/人工房产仅补缺失字段。业主导入只写 `property_ownership` 与房产的 `pricefinderAttributes`（估值、卧浴车位等独立保存），不创建联系人、不写 `rex_outbox`。单元只在 QLD 单元号相同或唯一地址匹配时写入，不把整栋业主错配给单元。`Current Owners` 未获 zone 授权不出现在手机同步数据。注意即使 dry-run 的未匹配样例只有地址，也要作为内部运维数据处理。

本地 2026-11-23 的四个真实导出在演示 seed + QLD 全量 21168 个新增/提升房产上 dry-run：Calamvale 5003 行匹配 909；Parkinson 3913 行匹配 845；Sunnybank Hills 5003 行匹配 953；Sunnybank 3792 行匹配 746（此处计数随版本/数据变化，以上为导入策略的粗略基线）。大量导出行并非规范的 lotplan/地址，**不要强行模糊匹配**；正式导入前核验样例及覆盖率，特别是重复门牌、多单位楼盘。App 新版内置地块由 756 KiB 增至约 4.5 MiB，手机需重新打包；服务器同步仍只传点坐标，不传 `geometry`。本地 Mary 演示库加入地块后的首次同步约 **954 KiB gzip / 6.7 MiB 原始 JSON**（旧基线约 120 KiB gzip），须在上线前压测手机内存/耗时。

回滚：保持服务停止，按本 runbook 的「备份与恢复」用上述备份 `axleo restore <快照>` 后再启动，用户退出并重新登录以替换旧全量快照；**恢复会回退维护窗口内的所有其他变更**。不要仅回退二进制（旧 App 仍缺四区地块）。如只撤销 zone，在管理界面删除这两个专用 zone；不要直接删有下游引用的房产。确认后从 65 的 `.tmp/pf-in` 清除导出及临时 GeoJSON。

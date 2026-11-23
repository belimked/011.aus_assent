# Pricefinder 四区导入（65）

输入文件包含个人信息：**只在受限的 `.tmp` 中保存，不进 git、日志或测试夹具**。QLD Spatial LandParcelPropertyFramework 地址 layer 0、地块 layer 4（© State of Queensland，CC BY 4.0）；`tools/download-qld-parcels.mjs` 按区分页下载。Sunnybank Hills、Calamvale 的 Pricefinder 导出各受 5000 条上限约束：后续追加文件再导入，已导入文件按 SHA-256 去重。

本地准备（重新打包手机 App：新版 `mobile/src/data/sunnybank.json` 包含四区地址和多边形；旧包不能显示新区背景灰点）。原始 API GeoJSON 放在 `.tmp`，**不要复制到版本库**：

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
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-import --provider pricefinder --kind ownership --dry-run /data/axleo/.tmp/pf-in/*.xls'
# 核对逐文件 matchedLot/matchedAddress/unmatched/misaligned/errors 和样例，再批准正式导入：
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-import --provider pricefinder --kind ownership /data/axleo/.tmp/pf-in/*.xls'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo market-zone mary@axleo.local; /opt/axleo/current/axleo market-zone john@axleo.local'
systemctl start axleo
```

`market-zone` 为指定账号幂等创建这四区的私有 zone，admin 本身有权限；不改其他账号权限。**先确认真实邮箱及 zone 归属**。非 Rex 地块按 lotplan 幂等插入；仅提升未注册且非 Rex 的旧地块记录；已有 Rex/人工房产不覆盖属性。业主导入只写 `property_ownership` 与房产的 `pricefinderAttributes`（估值、卧浴车位等独立保存），不创建联系人、不写 `rex_outbox`。单元只在 QLD 单元号相同或唯一地址匹配时写入，不把整栋业主错配给单元。`Current Owners` 未获 zone 授权不出现在手机同步数据。注意即使 dry-run 的未匹配样例只有地址，也要作为内部运维数据处理。

本地 2026-11-23 的四个真实导出在演示 seed + QLD 全量 21168 个新增/提升房产上 dry-run：Calamvale 5003 行匹配 909；Parkinson 3913 行匹配 845；Sunnybank Hills 5003 行匹配 953；Sunnybank 3792 行匹配 746（此处计数随版本/数据变化，以上为导入策略的粗略基线）。大量导出行并非规范的 lotplan/地址，**不要强行模糊匹配**；正式导入前核验样例及覆盖率，特别是重复门牌、多单位楼盘。App 新版内置地块由 756 KiB 增至约 4.5 MiB，手机需重新打包；服务器同步仍只传点坐标，不传 `geometry`。本地 Mary 演示库加入地块后的首次同步约 **954 KiB gzip / 6.7 MiB 原始 JSON**（旧基线约 120 KiB gzip），须在上线前压测手机内存/耗时。

回滚：保持服务停止，按本 runbook 的「备份与恢复」用上述备份 `axleo restore <快照>` 后再启动，用户退出并重新登录以替换旧全量快照；**恢复会回退维护窗口内的所有其他变更**。不要仅回退二进制（旧 App 仍缺四区地块）。如只撤销 zone，在管理界面删除这两个专用 zone；不要直接删有下游引用的房产。确认后从 65 的 `.tmp/pf-in` 清除导出及临时 GeoJSON。

# AxleoCollect 运维速查（65）

## 发布与回滚

在本地 `server/` 执行 `go test ./... && go vet ./...`；审核变更后用 `SSHPASS=… deploy/release.sh` 发布。脚本切换 `/opt/axleo/current`、重启 `axleo.service`，健康检查失败自动回滚。手动回滚：在 65 以 root 执行 `ln -sfn "$(cat /data/axleo/previous-release)" /opt/axleo/current && systemctl restart axleo`；检查 `curl -fsS http://127.0.0.1:8082/api/v1/health` 和 `journalctl -u axleo -n 100`。**回滚二进制不能自动回退数据库 schema**；如需回退数据，按下面的恢复流程操作。发布前保留快照。

## 备份与恢复

`axleo-backup.timer` 每日 03:30 调用 `/opt/axleo/current/axleo backup /data/axleo/backup`，用 SQLite 在线快照；保留最近 14 份，文件位于 `/data/axleo/backup/axleo-*.db`。可手动执行相同命令。备份验证：`/opt/axleo/current/axleo verify /data/axleo/backup/<文件>.db`，输出完整性检查与各表行数。备份应额外异机保存。

恢复（**会覆盖当前库**）：先确认要恢复的文件、维护窗口及程序版本。root 执行 `systemctl stop axleo`；确认无其他进程使用数据库；再以 `axleo` 用户、加载 `/etc/axleo/env` 的环境运行 `/opt/axleo/current/axleo restore <快照绝对路径>`。命令校验快照和 schema，将当前库另存于数据库同级 `backup/` 并输出文件名，然后替换数据库。恢复失败不要启动服务，先检查日志与备份。最后 `systemctl start axleo` 并检查健康、账号和业务数量。不要恢复比当前程序更新的 schema；不要在服务运行时替换 WAL 文件。

重置测试数据（**清空业务数据并恢复 Mary/John 演示数据**）：root 执行 `systemctl stop axleo`；以 `axleo` 用户运行 `sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; exec /opt/axleo/current/axleo reset-data'` 查看各表行数（只读，不做任何修改）；确认后同样方式运行 `reset-data --yes`：先在 `AXLEO_BACKUP_DIR` 生成并校验快照（失败则中止），再在单个事务内清空业务表（包括 Rex 导入联系人、关系、`rex_links` 和 `rex_outbox`）、删除不在 demo.json 中的 Rex 导入房产（即使含 lotplan）、删除手机端新建的房产、把 demo.json 中的 4,461 处房产恢复为种子值，并重新写入演示数据；保留账号、团队、设置、管理员绘制的区域、Pricefinder 导入和 `rex_request_log` 调用审计。预览会列出 Rex 导入房产/联系人、链接、outbox、关系和保留的日志数量。所有会话被注销，手机须退出后重新登录。最后 `systemctl start axleo`。回滚：用输出的快照执行 `restore`。重置不会调用 Rex；需要重置后重导入时使用 `reset-data --yes --rex-import`（账户取 `REX_ACCOUNT_ID`，默认按 sandbox-test 固定方案分配（见 `docs/rex-integration.md`））。它先完成重置，随后检查 `AXLEO_REX_READ=1`、账户和凭证并执行只读导入；检查或导入失败会输出 `未导入：原因`、以非零状态退出，**已提交的重置不会回滚**。此模式仅用于沙箱 2057。

本地运维机可用单条命令远程执行（先部署包含此脚本和新服务版本的 release；需要 `sshpass`，**不会自动部署二进制**）：

```sh
SSHPASS=1 bash server/deploy/reset-test.sh                 # 演示数据
SSHPASS=1 bash server/deploy/reset-test.sh --rex-import    # 演示数据 + 重新导入 2057
```

脚本对 `--rex-import` 先验证远端 `REX_ACCOUNT_ID=2057`，然后停服务、以 axleo 用户重置、重新启动并检查 `/api/v1/health`。重置命令打印 `Verified backup: /data/axleo/backup/axleo-*.db`、before/after 各表行数和（若请求导入）`properties: added=… updated=… skipped=…`、`contacts: ...`。任何步骤失败均尝试启动服务，打印快照恢复命令；如果连备份都没取得，会提示先查备份目录。失败时不要盲目重复导入或恢复，先检查 `journalctl -u axleo` 和本次输出。

## 只读演练

在 65 以 root 设置 `AXLEO_ADMIN_EMAIL`、`AXLEO_ADMIN_PASSWORD`，运行 `server/deploy/restore-drill.sh`（先把脚本送至 65）。脚本从最新快照恢复至 `/data/axleo/drill/run.*`，在 `127.0.0.1:8092` 启动临时实例，检查健康和 admin 登录、对比线上与快照的各类记录数，退出时杀进程并清理临时文件；线上服务不停止。若线上在备份后有新写入，数量比较会不一致，需选择静默时段或手工核对差异。依赖 `curl`、`jq`。

## 账号

改密码：停止/不停服务均可，在加载 `/etc/axleo/env` 后执行 `printf '%s\n' '<新密码>' | /opt/axleo/current/axleo passwd <邮箱>`；会吊销旧 session，密码至少 8 位。加账号：在 `/admin/` 用管理员登录，在 Accounts 创建（邮箱、姓名、角色、初始密码），并通过安全渠道告知密码；需要时禁用账号或执行 Sign out devices。切勿把密码放在工单、命令行历史或截图中。

## Rex 账户与调用日志

在 65 的 `/etc/axleo/env` 中配置 `REX_BASE_URL`、`REX_APP_ID`、`REX_EMAIL`、`REX_PASSWORD`、`REX_ACCOUNT_ID`（2057 共享沙箱 / 758 Place Sunnybank 正式）。缺任一项绝不调用。`AXLEO_REX_READ=1` 允许后续读取；仅 `AXLEO_REX_WRITE=1` 且 `REX_WRITE_ACCOUNT_ID` 与目标账户完全一致时启动写入 worker。删除旧 `AXLEO_REX_ENABLED`（被忽略）；可选 `REX_INTERVAL_MS` 和 `AXLEO_REX_WEBHOOK_TOKEN`。修改后重启 `axleo`，检查 `journalctl -u axleo` 中禁用原因。管理员 `/admin/#/settings` 查看连接状态，`/admin/#/rex-log` 查看分页调用日志；`rex_request_log` 每次 HTTP 尝试和 webhook 留痕，启动时清理 180 天前日志。探测 CLI 仅使用配置账户，需显式 `--account` 匹配；如数据库不可用，日志落到 `--out/rex-request-log.jsonl`。切勿在正式账户运行沙箱探测。

## Rex 2057 沙箱一次性导入（维护窗口）

先确认 `/etc/axleo/env` 中 `REX_ACCOUNT_ID=2057`、`AXLEO_REX_READ=1`、凭证完整、`REX_INTERVAL_MS>=1000`，Mary/John/admin 已创建，并由运维在 Accounts 提前创建启用的 `import@axleo.local`（仅测试大数据同步耗时）。导入命令不会创建账号；缺任何账号在 Rex 登录前报错。重置已完成但导入失败时需检查原因，不要以为重置已回滚。**停止服务以免并发写库；不要设置 `AXLEO_REX_WRITE`。** 65 上 root 执行：

```sh
systemctl stop axleo
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo backup /data/axleo/backup'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo rex-import --account 2057 --dry-run --plan sandbox-test'
# 人工核对新增/更新/跳过数量和 rex_request_log 错误后：
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo rex-import --account 2057 --plan sandbox-test'
systemctl start axleo
```

只试少量数据：两条导入命令均可加 `--limit 100`（每种实体 100 条；关联联系人不一定落在首 100 条）。重跑相同正式命令，已有记录即使源字段变化也不改本地负责人；新记录按剩余容量分配，满额给 admin。旧的 Mary/John 轮流方式可显式使用 `--plan legacy` 或 `--assign mary@axleo.local,john@axleo.local`。导入修改分配后会吊销 Mary/John/import/admin 的 sessions；手机用户须重新登录以全量同步，不能依赖增量同步移除旧范围记录。**不要在正式账户执行。** 导入并非全局事务，出错可修好后重跑；呼叫日志 `caller=import` 可核对。移动端已有旧快照的用户建议重新登录。

删除全部导入数据：优先在停服后用上述备份执行 `axleo restore <快照路径>`，但这也会回滚备份后其他写入。只删导入数据时，先停服并备份，核对以下 SQL 只会命中 `source=rex_import` 的 id；以 `sudo -u axleo sqlite3 /data/axleo/db/axleo.db` 运行（需要安装 sqlite3），**不要删除演示数据**：

```sql
PRAGMA foreign_keys=ON;
BEGIN IMMEDIATE;
INSERT INTO settings(key,value) VALUES('sync_reset_seq',(SELECT COALESCE(max(seq),0) FROM sync_changes)) ON CONFLICT(key) DO UPDATE SET value=excluded.value;
DELETE FROM contact_notes WHERE contact_id IN (SELECT id FROM contacts WHERE json_extract(data,'$.source')='rex_import');
DELETE FROM property_owners WHERE property_id IN (SELECT id FROM properties WHERE json_extract(data,'$.source')='rex_import') OR contact_id IN (SELECT id FROM contacts WHERE json_extract(data,'$.source')='rex_import');
DELETE FROM rex_links WHERE account_id='2057' AND ((entity='properties' AND local_id IN (SELECT id FROM properties WHERE json_extract(data,'$.source')='rex_import')) OR (entity='contacts' AND local_id IN (SELECT id FROM contacts WHERE json_extract(data,'$.source')='rex_import')));
DELETE FROM properties WHERE json_extract(data,'$.source')='rex_import';
DELETE FROM contacts WHERE json_extract(data,'$.source')='rex_import';
COMMIT;
```

之后启动服务，客户端须重新全量同步。SQL 未清理手工添加且指向导入实体的其他业务记录（待逐一核对）；若出现 FK 错误事务将失败，请回滚并使用快照恢复。`/api/v1/sync` 仍未分页，约每人 4,000 联系人 + 1,700 房产。同步 JSON 已剔除联系人/房产的 `data.rex`，以及房产的 `data.geometry`（手机使用本地地块图层和 `ll` 点坐标），后台详情保留；支持请求 `Accept-Encoding: gzip` 时对 >=1 KiB JSON API 响应压缩，流式响应与静态资源不压缩。gzip 降低网络流量，但不减少逐行查询与手机解压后内存；上线前仍应压测真实 2057 数据。

### Rex 双向同步安全开关（沙箱）
仅在沙箱 2057 经批准后设置 `AXLEO_REX_READ=1`、`AXLEO_REX_WRITE=1`、`REX_ACCOUNT_ID=2057`、`REX_WRITE_ACCOUNT_ID=2057`。写入还需 AccountSettings::read 当前账户验证通过并在 `REX_WRITE_ALLOWED_ACCOUNTS`（默认只有 2057）内；758 即使列入也必须另设 `REX_ALLOW_PRODUCTION_WRITE=1`，**目前不可设置**。已在沙箱确认 `POST /v1/rex/AccountSettings/read`（请求 `{}`）返回数字 `result.id`、`name`、`is_test_account`；不匹配账户或验证失败时写入一律拦截；非测试账户须额外设置 `REX_ALLOW_PRODUCTION_WRITE=1`（目前不设）。后台 `/api/v1/admin/rex/status` 显示拦截和轮询状态。`REX_POLL_SECONDS=60`（至少 30）拉取，`REX_POLL_SOFT_DELETE=0` 可暂停消失记录的软删。每小时基线约 120 次 search（两种实体各一页），变化记录另加每条一次 read，登录另计；超过 10000 条每页另加 search。迁移 015 将旧未关联 outbox 标为 skipped，新未关联任务由 worker 同样跳过。部署前先备份，核对 import 用户和日志，切勿把真实 Rex 凭证用于自动化测试。

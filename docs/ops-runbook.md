# AxleoCollect 运维速查（65）

## 发布与回滚

在本地 `server/` 执行 `go test ./... && go vet ./...`；审核变更后用 `SSHPASS=… deploy/release.sh` 发布。脚本切换 `/opt/axleo/current`、重启 `axleo.service`，健康检查失败自动回滚。手动回滚：在 65 以 root 执行 `ln -sfn "$(cat /data/axleo/previous-release)" /opt/axleo/current && systemctl restart axleo`；检查 `curl -fsS http://127.0.0.1:8082/api/v1/health` 和 `journalctl -u axleo -n 100`。**回滚二进制不能自动回退数据库 schema**；如需回退数据，按下面的恢复流程操作。发布前保留快照。

## 备份与恢复

`axleo-backup.timer` 每日 03:30 调用 `/opt/axleo/current/axleo backup /data/axleo/backup`，用 SQLite 在线快照；保留最近 14 份，文件位于 `/data/axleo/backup/axleo-*.db`。可手动执行相同命令。备份验证：`/opt/axleo/current/axleo verify /data/axleo/backup/<文件>.db`，输出完整性检查与各表行数。备份应额外异机保存。

恢复（**会覆盖当前库**）：先确认要恢复的文件、维护窗口及程序版本。root 执行 `systemctl stop axleo`；确认无其他进程使用数据库；再以 `axleo` 用户、加载 `/etc/axleo/env` 的环境运行 `/opt/axleo/current/axleo restore <快照绝对路径>`。命令校验快照和 schema，将当前库另存于数据库同级 `backup/` 并输出文件名，然后替换数据库。恢复失败不要启动服务，先检查日志与备份。最后 `systemctl start axleo` 并检查健康、账号和业务数量。不要恢复比当前程序更新的 schema；不要在服务运行时替换 WAL 文件。

重置测试数据（**清空业务数据并恢复 Mary/John 演示数据**）：root 执行 `systemctl stop axleo`；以 `axleo` 用户运行 `sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; exec /opt/axleo/current/axleo reset-data'` 查看各表行数（只读，不做任何修改）；确认后同样方式运行 `reset-data --yes`：先在 `AXLEO_BACKUP_DIR` 生成并校验快照（失败则中止），再在单个事务内清空业务表、删除手机端新建的房产、把 demo.json 中的 4,461 处房产恢复为种子值，并重新写入演示数据；保留账号、团队、设置、管理员绘制的区域和 Pricefinder 导入。所有会话被注销，手机须退出后重新登录。最后 `systemctl start axleo`。回滚：用输出的快照执行 `restore`。

## 只读演练

在 65 以 root 设置 `AXLEO_ADMIN_EMAIL`、`AXLEO_ADMIN_PASSWORD`，运行 `server/deploy/restore-drill.sh`（先把脚本送至 65）。脚本从最新快照恢复至 `/data/axleo/drill/run.*`，在 `127.0.0.1:8092` 启动临时实例，检查健康和 admin 登录、对比线上与快照的各类记录数，退出时杀进程并清理临时文件；线上服务不停止。若线上在备份后有新写入，数量比较会不一致，需选择静默时段或手工核对差异。依赖 `curl`、`jq`。

## 账号

改密码：停止/不停服务均可，在加载 `/etc/axleo/env` 后执行 `printf '%s\n' '<新密码>' | /opt/axleo/current/axleo passwd <邮箱>`；会吊销旧 session，密码至少 8 位。加账号：在 `/admin/` 用管理员登录，在 Accounts 创建（邮箱、姓名、角色、初始密码），并通过安全渠道告知密码；需要时禁用账号或执行 Sign out devices。切勿把密码放在工单、命令行历史或截图中。

## Rex 账户与调用日志

在 65 的 `/etc/axleo/env` 中配置 `REX_BASE_URL`、`REX_APP_ID`、`REX_EMAIL`、`REX_PASSWORD`、`REX_ACCOUNT_ID`（2057 共享沙箱 / 758 Place Sunnybank 正式）。缺任一项绝不调用。`AXLEO_REX_READ=1` 允许后续读取；仅 `AXLEO_REX_WRITE=1` 且 `REX_WRITE_ACCOUNT_ID` 与目标账户完全一致时启动写入 worker。删除旧 `AXLEO_REX_ENABLED`（被忽略）；可选 `REX_INTERVAL_MS` 和 `AXLEO_REX_WEBHOOK_TOKEN`。修改后重启 `axleo`，检查 `journalctl -u axleo` 中禁用原因。管理员 `/admin/#/settings` 查看连接状态，`/admin/#/rex-log` 查看分页调用日志；`rex_request_log` 每次 HTTP 尝试和 webhook 留痕，启动时清理 180 天前日志。探测 CLI 仅使用配置账户，需显式 `--account` 匹配；如数据库不可用，日志落到 `--out/rex-request-log.jsonl`。切勿在正式账户运行沙箱探测。

## Rex 2057 沙箱一次性导入（维护窗口）

先确认 `/etc/axleo/env` 中 `REX_ACCOUNT_ID=2057`、`AXLEO_REX_READ=1`、凭证完整、`REX_INTERVAL_MS>=1000`，Mary/John 均已创建。**停止服务以免并发写库；不要设置 `AXLEO_REX_WRITE`。** 65 上 root 执行：

```sh
systemctl stop axleo
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo backup /data/axleo/backup'
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo rex-import --account 2057 --dry-run --assign mary@axleo.local,john@axleo.local'
# 人工核对新增/更新/跳过数量和 rex_request_log 错误后：
sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; /opt/axleo/current/axleo rex-import --account 2057 --assign mary@axleo.local,john@axleo.local'
systemctl start axleo
```

只试少量数据：两条导入命令均可加 `--limit 100`（每种实体 100 条；关联联系人不一定落在首 100 条）。重跑相同正式命令，etag 未变跳过且不改本地负责人。**不要在正式账户执行。** 导入并非全局事务，出错可修好后重跑；呼叫日志 `caller=import` 可核对。移动端已有旧快照的用户建议重新登录。

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

之后启动服务，客户端须重新全量同步。SQL 未清理手工添加且指向导入实体的其他业务记录（待逐一核对）；若出现 FK 错误事务将失败，请回滚并使用快照恢复。`/api/v1/sync` 目前未分页，约每人 4,000 联系人 + 1,700 房产，完整 `data.rex` 会增加传输和逐行查询开销；上线前压测首次同步和低端手机内存，若超时先停用导入数据的手机全量同步并实现分页，切勿无监控放量。

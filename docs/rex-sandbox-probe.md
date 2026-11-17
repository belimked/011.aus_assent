# Rex 沙箱验证（axleo rex-probe）

此工具**不会自动读取凭证文件**，也不会修改本地 SQLite / rex_outbox。仅在收到 Rex 共享沙箱凭证、确认账户 ID 后使用；先只读，再写入。不可对正式账号运行。

## 在 65 上执行

将沙箱凭证放在服务器已有的 `/etc/axleo/env`（文件权限仅管理员可读；切勿提交到仓库）：

```sh
REX_BASE_URL=https://api.rexsoftware.com
REX_EMAIL=沙箱集成用户邮箱
REX_PASSWORD=沙箱密码
REX_ACCOUNT_ID=沙箱账户ID
REX_APP_ID=Integration:PlaceSunnybank:Portal
```

在 65 上用可读取环境文件、可写报告目录的管理员 shell 执行（按实际部署路径修改二进制路径）：

```sh
set -a; . /etc/axleo/env; set +a
mkdir -p /data/axleo/rex-probes
/data/axleo/axleo rex-probe --account "$REX_ACCOUNT_ID" --out /data/axleo/rex-probes/readonly
# 核对报告后，使用不同的输出目录：
/data/axleo/axleo rex-probe --account "$REX_ACCOUNT_ID" --write --remindee '沙箱用户ID' --out /data/axleo/rex-probes/write
```

`--account` 必填且必须等于 `REX_ACCOUNT_ID`；不符时只登录并打印可访问账户 ID，不执行检查。`--only F1,F5` 可筛选，`--keep` 暂留测试数据供人工检查；之后执行 `/data/axleo/axleo rex-probe cleanup --state /data/axleo/rex-probes/write/state.json`。清理失败时保留 state.json 中的未清理 ID，必须人工核对并重试。报告目录须保密，尤其 state.json 中的 Rex ID。

## 检查与报告

`report.md` 的 PASS 表示请求成功，**不代表业务语义自动确认**；UNKNOWN 表示接口/参数不确定或需要核对 raw；MANUAL 必须人工核实。`raw/<检查>-<序号>.json` 保存请求、响应结构（真实记录值脱敏）及 `key_tree`（字段及类型，数组仅取首项形状）；错误和接口字段要根据沙箱结果确认。P0 检查登录、账户、两种路径；F1 测试联系人新标签；F2 测试指定/自动分配 lead；F3 测试提醒并要求查看手机推送；F4 查看开放参观场次与描述字段（卖家报告须在 Rex 网页人工核对）；F5 检查不投信字段搜索及仅在写模式下新建房产标签；F7 检查房产 owner/tenant 关系；S1 提取提醒、去重、标签、线索、匹配、OAB、场次和值列表结构。只读模式跳过写检查和去重（无测试联系人）。

## 安全边界

沙箱与其他公司**共享**，API 写入可能触发 Rex 自动化、通知及推送；所有写入必须先获得许可。所有新记录以 `AXLEO TEST - ` 开头；不要更新已有沙箱记录。创建返回 ID 后立即写入 state.json，默认结束时尝试 delete，失败尝试 archive；有失败必须人工检查。默认客户端每秒最多 1 次请求（仍与办公室共用每小时 3,600 次额度），遇到 429 按 Retry-After 有界重试。凭证和 token 不写入报告；不要将报告或凭证上传到公开位置。

## 2026-11-17 实际运行记录

- 集成账号能访问两个账户：**2057 = Integration Partners Testing Account（共享沙箱）**、**758 = Place Sunnybank（正式，禁止运行 probe）**。登录必须带 `account_id`。
- 本机凭证：`/etc/axleo/rex-sandbox.env`（600），用法 `set -a; . /etc/axleo/rex-sandbox.env; set +a`。
- 集成用户 ID 为 `80559`。共享沙箱有 200 多个其他集成商用户，`--remindee 80559` 同时用作 F2 的指定负责人，**不要派给其他用户**。
- Rex 没有通用 `delete`：清理按 purge → trash → archive 依次尝试；联系人和房产只能 trash/archive。
- 结论见 `rex-feedback-2026-11.md` §6。

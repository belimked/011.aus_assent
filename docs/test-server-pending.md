# 测试服务器待办（2026-11-25 记录）

测试服务器：16.178.91.22（AWS Sydney），main = `3c34580`，schema 18。
所有操作前先备份：`axleo backup /data/axleo/backup`。

## 1. 成交提醒测试（未做）
- 增量文件：本地 `.tmp/fakerp/out/incremental/`，共 4 个文件 14 条近 7 天的“新成交”（Sunnybank 3、Sunnybank Hills 4、Calamvale 3、Parkinson 4）。
- 导入后 Mary 会收到 14 条提醒。
- 步骤：scp 到服务器 `/data/axleo/.tmp/fakerp/incremental/`（属主 axleo），然后：
  ```sh
  axleo market-import --provider rpdata --dry-run '/data/axleo/.tmp/fakerp/incremental/'*.xlsx
  axleo market-import --provider rpdata '/data/axleo/.tmp/fakerp/incremental/'*.xlsx
  ```

## 2. 清除假数据（导入真实 RP Data 之前必须先做）
- 测试服务器上已导入 2,189 套 SYNTHETIC 房产（4 个区，业主名带 “(Test)”），初始批 0 条提醒。
- 拿到 CoreLogic 授权、导入真实数据**之前**：
  ```sh
  axleo market-purge-synthetic --dry-run   # 先看数量
  axleo market-purge-synthetic
  ```
- 顺序不能反：先导入真实数据再清理，会误删业主记录。详见 `docs/rpdata-synthetic.md`。
- 生产环境禁止导入假数据。

## 3. 删除 Rex 沙箱测试备注（等人工在 Rex 网页核对后）
- 房产 16 Gager Street（Rex property 23138536）上的两条 “AXLEO TEST - …” 备注：Notes **141116060**、**141116065**。
- 核对内容：房产页能否看到、作者显示、末尾签名 “— via AxleoCollect · Test Agent”。
- 删除：`ENV_FILE=/etc/axleo/env bash /home/admin/rexnotes.sh cleanup 141116060 141116065`（脚本源：本地 `.tmp/rexnotes/sandbox-property-note.sh`）。

## 4. 真实 RP Data 正式导入前的前置工作
- 等 Kevin/Ping 确认 CoreLogic 授权与数据处理。
- 真实约 2.19 万套全部同步到手机会再次卡顿：先实现“RP Data 房产不预先下发、打开时按需从服务器读取”。
- 本地 `.tmp/rpdata-src/` 的 4 个真实文件（约 160MB，含个人信息）授权确认前建议删除。

## 5. 其他
- 数据库备份中仍含已清理联系人（Sawley）的数据，测试阶段结束后统一清理旧备份。
- 等 Ping 用 64 号包测试：16 Gager St 写房产备注 → Rex 房产页可见；issue 4 在 16 Gager St 重测；issue 7 OFI。

# agentCollect iOS Test Build Manual · agentCollect iOS 测试版手册

Ad Hoc distribution: register an iPhone, sign a test build, and install it in Safari.  
Ad Hoc 分发：注册 iPhone、签名测试版，并通过 Safari 安装。

| | |
| --- | --- |
| App · 应用 | agentCollect · Place Sunnybank |
| Bundle ID · 包标识 | `com.placesunnybank.agentcollect` (from `mobile/app.json`; confirm before creating App ID) |
| Apple Team · 开发者团队 | `8J42Q87M5P` |
| Repository · 仓库 | `belimked/011.aus_assent` |
| Workflow · 工作流 | **Mobile iOS Ad Hoc** (`.github/workflows/mobile-ios-adhoc.yml`) |
| Install page · 安装页 | <https://belimked.github.io/011.aus_assent/> |
| Device · 设备 | Registered iPhone, iOS 16.4+; tapping map properties requires iOS 18+ · 已登记的 iPhone，iOS 16.4+；点击地图房产需 iOS 18+ |

## One-time setup · 首次设置

1. On <https://developer.apple.com/account>, select team **8J42Q87M5P** → Certificates, Identifiers & Profiles → Identifiers → **+** → App IDs → App, create an explicit App ID for `com.placesunnybank.agentcollect` (or the confirmed ID in `mobile/app.json`). This is a **new** App ID, not AusFlow's.  
   在 Apple 开发者后台选择团队 **8J42Q87M5P** → Certificates, Identifiers & Profiles → Identifiers → **+** → App IDs → App，为 `com.placesunnybank.agentcollect`（或确认后的 `mobile/app.json` ID）新建明确的 App ID，不能沿用 AusFlow 的 App ID。
2. Register tester UDIDs (steps below). Create an **Ad Hoc** iOS provisioning profile named **agentcollect adhoc** for that App ID, using the team's Apple Distribution certificate and selecting all tester devices. Devices registered for AusFlow can be reused in this new profile. Download the `.mobileprovision` file.  
   登记测试设备 UDID（步骤见下文）。为新 App ID 创建名为 **agentcollect adhoc** 的 iOS **Ad Hoc** 描述文件，选择团队的 Apple Distribution 证书及测试设备。已登记的 AusFlow 设备可以重新选入本描述文件。下载 `.mobileprovision`。
3. In `belimked/011.aus_assent` → **Settings → Pages**, set **Source = GitHub Actions**.  
   在仓库 **Settings → Pages** 中设 **Source = GitHub Actions**。
4. In the repo's **Settings → Secrets and variables → Actions**, set all three repository secrets:
   在仓库 **Settings → Secrets and variables → Actions** 中设置三个密钥：
   - `IOS_DIST_P12_BASE64`: base64 of the team's Apple Distribution `.p12`; the existing AusFlow certificate value can be copied by its custodian (GitHub does **not** show existing secret values). Never commit the certificate.  
     团队发布证书 `.p12` 的 base64；可请 AusFlow 密钥保管人复制原始值（GitHub **不会**显示已有密钥的值）。不要提交证书。
   - `IOS_DIST_P12_PASSWORD`: password for that `.p12`; likewise reuse the AusFlow value from its custodian.  
     该 `.p12` 的密码；同样请保管人提供 AusFlow 所用的值。
   - `AGENTCOLLECT_ADHOC_PROFILE_BASE64`: base64 of the **new agentCollect** profile; do not use AusFlow's profile. Linux: `base64 -w0 agentcollect.mobileprovision`; macOS: `base64 -i agentcollect.mobileprovision`.  
     **新 agentCollect** 描述文件的 base64，不能用 AusFlow 的描述文件。Linux：`base64 -w0 agentcollect.mobileprovision`；macOS：`base64 -i agentcollect.mobileprovision`。

Before secrets are configured, preflight (typecheck/lint/tests) succeeds and warns about missing secrets; the signed build is skipped.  
未配置密钥时预检（类型检查、lint、测试）仍会通过并警告缺失的密钥；签名构建会跳过。

**Public repository notice · 公开仓库提醒：** the install page and IPA are **publicly downloadable**. Only registered devices can install the IPA, but download is not restricted. This app currently contains only **mock owner/status data and public Queensland address/parcel data**; do not embed CRM credentials or private records in a test build.  
安装页与 IPA **可公开下载**；仅登记设备能安装，但任何人都能下载文件。当前应用仅包含**模拟业主/状态及昆士兰公开地址/地块数据**；勿把 CRM 凭据或私人记录放入测试版。

## 1. Get the UDID · 获取 UDID

The UDID is not the serial number or IMEI and is not shown in iPhone Settings. On the iPhone, open <https://udid.tech> in **Safari** (not WeChat/Chrome), tap **Get My UDID → Allow** to download a configuration profile, then open **Settings → Profile Downloaded** (or **General → VPN & Device Management**) → **Install**. Safari shows the UDID; copy it as text and send it with the device name to the admin. Avoid the site's unrelated signing/store ads; delete its temporary profile from VPN & Device Management after use if still present. Newer iPhones use a 25-character hyphenated UDID; older ones use 40 hex characters.  
UDID 不是序列号或 IMEI，也不会显示在 iPhone 设置里。用 iPhone **Safari**（不要用微信/Chrome）打开 <https://udid.tech>，点 **Get My UDID → Allow** 下载描述文件，再到**设置 → 已下载描述文件**（或**通用 → VPN 与设备管理**）→ **安装**。Safari 会显示 UDID；以文字形式连设备名发给管理员。不要点网页上无关的签名/商店广告；如临时描述文件仍在设置中，用完后删除。新款 iPhone 的 UDID 为含连字符的 25 位，旧款为 40 位十六进制字符。

## 2. Register device and update profile · 注册设备并更新描述文件

On developer.apple.com, select team `8J42Q87M5P` → **Certificates, Identifiers & Profiles → Devices → +**. Select iOS, enter a name and the exact UDID, then **Continue → Register**. Apple limits registration to 100 iPhones per membership year (disabled devices still count).  
在开发者后台选择团队 `8J42Q87M5P` → **Certificates, Identifiers & Profiles → Devices → +**；选 iOS，输入设备名与准确 UDID，点 **Continue → Register**。每会员年度最多注册 100 台 iPhone（停用仍占名额）。

Registration does not update a profile automatically: **Profiles → agentcollect adhoc → Edit → select new device(s) → Save → Download**. Replace `AGENTCOLLECT_ADHOC_PROFILE_BASE64` with the new file's base64 (do not commit `.mobileprovision`). Example: `base64 -w0 agentcollect.mobileprovision | gh secret set AGENTCOLLECT_ADHOC_PROFILE_BASE64 -R belimked/011.aus_assent` (on macOS use `base64 -i`). Alternatively paste the encoded value through repository Settings → Secrets and variables → Actions.  
注册设备不会自动更新描述文件：**Profiles → agentcollect adhoc → Edit → 勾选新设备 → Save → Download**。用新文件的 base64 更新 `AGENTCOLLECT_ADHOC_PROFILE_BASE64`（不要提交 `.mobileprovision`）。例如 Linux 运行上面的命令；macOS 使用 `base64 -i`；也可在 GitHub 仓库密钥设置页粘贴编码值。

## 3. Rebuild and install · 重新构建并安装

Run **Actions → Mobile iOS Ad Hoc → Run workflow** on `main`, or `gh workflow run mobile-ios-adhoc.yml -R belimked/011.aus_assent --ref main`. Pushes to main that change `mobile/**` (except `mobile/docs/**`) or the workflow also build. Check the signing step's device count and expiry. A new profile requires a **new IPA**; old IPAs cannot install on newly added devices. The IPA remains an Actions artifact for 30 days. After deploy, open <https://belimked.github.io/011.aus_assent/> in iPhone **Safari**, tap **Install agentCollect**, confirm **Install** and wait for the home-screen icon.  
在 `main` 上运行 **Actions → Mobile iOS Ad Hoc → Run workflow**，也可用上述 `gh` 命令。推送修改 `mobile/**`（`mobile/docs/**` 除外）或工作流的代码到 main 也会触发。检查签名步骤的设备数和到期时间。更新描述文件后必须**重新构建 IPA**，旧包无法在新增设备上安装。IPA 在 Actions 保留 30 天。部署完成后用 iPhone **Safari** 打开安装页，点 **Install agentCollect**、确认 **Install**，等待主屏幕图标安装完成。

On first launch, if iOS asks for Developer Mode, tap **OK**, then **Settings → Privacy & Security → Developer Mode → On → restart → Turn On**. The switch may only appear after tapping the app icon once. Reopen the install page to install an update. Allow location for the nearby-property demo; no Rex sign-in is implemented yet.  
首次打开若提示开发者模式，点**好**，进入**设置 → 隐私与安全性 → 开发者模式 → 开启 → 重启 → 打开**。开关可能要先点一次应用图标才出现。更新时重新打开安装页安装。允许定位可体验附近房产演示；尚未实现 Rex 登录。

## 4. Troubleshooting and renewal · 排错与续期

| Symptom · 现象 | Fix · 处理 |
| --- | --- |
| Unable to install · 无法安装 | Verify device UDID in the *agentCollect* profile, then update secret and rebuild; use Safari and iOS 16.4+. · 检查新描述文件中的 UDID、更新密钥并重建；使用 Safari 和 iOS 16.4+。 |
| No Developer Mode · 无开发者模式 | Tap the app icon once, then check Settings. · 先点一次应用图标，再检查设置。 |
| No map-property tap · 无法点击地图房产 | Apple map click callbacks require iOS 18+. · Apple 地图点击回调需要 iOS 18+。 |
| Preflight green but no build · 预检通过却没构建 | Missing signing secrets; read the preflight warning and complete one-time setup. · 缺少签名密钥；查看预检警告并完成首次设置。 |
| Archive/export profile error · 签名导出报错 | Profile must match `mobile/app.json` bundle ID, team, Apple Distribution cert and Ad Hoc method. · 描述文件须与包 ID、团队、发布证书和 Ad Hoc 类型匹配。 |
| App won't open · 应用无法打开 | Enable Developer Mode; check profile/certificate expiry. · 开启开发者模式并检查证书/描述文件有效期。 |

The distribution certificate and Ad Hoc profile typically expire after a year. Before expiry, renew the certificate (`IOS_DIST_P12_BASE64`, `IOS_DIST_P12_PASSWORD` if changed), regenerate the profile with the new certificate and devices, update `AGENTCOLLECT_ADHOC_PROFILE_BASE64`, then rebuild. The profile expiry is printed in the build log.  
发布证书及 Ad Hoc 描述文件通常一年到期。到期前续签证书（必要时更新 `IOS_DIST_P12_BASE64`、`IOS_DIST_P12_PASSWORD`），用新证书和设备重新生成描述文件，更新 `AGENTCOLLECT_ADHOC_PROFILE_BASE64` 后重建。构建日志会打印到期日期。

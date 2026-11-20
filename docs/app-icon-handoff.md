# AxleoCollect 新图标：开发交接文档

本文档给接手的开发者（或模型）使用，独立可读，不需要任何前置对话。

## 1. 背景

AxleoCollect 是给房产中介代理用的移动端 app（Expo / React Native，代码在 `mobile/`）。原图标是 Axleo 字标里带"轴"的字母 o（圆环、中心点、横杆），看不出和房产有关。

新图标保留圆环、中心点和横杆，把圆环内侧的圆孔换成房子轮廓：

- 白色实心圆盘，中间镂空出一个尖顶房子；
- 橙色圆点在房子墙体中间；
- 白色横杆从圆点向右穿出圆盘。

底色用深蓝 `#13233F`，和原 app 图标一致。

## 2. 当前状态

| 项 | 状态 |
|---|---|
| `mobile/assets/icon.png`（1024，iOS 和商店用，不透明满铺） | 已替换，未提交 |
| `mobile/assets/adaptive-icon.png`（1024，Android 前景，透明底） | 已替换，未提交 |
| `mobile/assets/splash-icon.png`（512，圆角方块，透明底） | 已替换，未提交 |
| `mobile/assets/icon-source.svg`（矢量源文件，新增） | 已添加，未提交 |
| `mobile/app.json` | 不需要改，路径和 `adaptiveIcon.backgroundColor: #13233F` 都保持原样 |
| app 内 `Logo` 组件 | **未改，仍是字母 "P"**，见任务 A |
| 真机或构建验证 | **未做**，目前只看过渲染预览，见任务 B |
| 后台 `web/` 的 favicon 和 logo | 未改，是否要改待定，见任务 C |

三张 PNG 是用无头 Chromium 把 SVG 截图生成的。生成脚本在 `.tmp/icons/final.mjs`，这个目录被 gitignore，换一个检出就没有了；需要重新出图时按第 3 节的几何数据重做即可。

## 3. 图形规格

颜色：

| 名称 | 值 | 用途 |
|---|---|---|
| Core Ink | `#13233F` | 底色、房子镂空 |
| White | `#FFFFFF` | 圆盘、横杆 |
| Signal Orange | `#E8622C` | 中心点 |

几何数据，坐标系是品牌包用的 64 单位空间：

```svg
<circle cx="32" cy="32" r="28.5" fill="#FFFFFF"/>
<polygon points="32,16 44.5,27 44.5,44 19.5,44 19.5,27"
         fill="#13233F" stroke="#13233F" stroke-width="2.5" stroke-linejoin="round"/>
<line x1="32" y1="33.5" x2="60.5" y2="33.5"
      stroke="#FFFFFF" stroke-width="7" stroke-linecap="round"/>
<circle cx="32" cy="33.5" r="6.5" fill="#E8622C"/>
```

绘制顺序必须是上面这个顺序（圆盘、房子、横杆、圆点）。

图形的包围盒是 x 3.5 到 64、y 3.5 到 60.5，中心 (33.75, 32)，最长边 60.5。各资源里图形最长边占画布的比例：

| 资源 | 占比 | 说明 |
|---|---|---|
| `icon.png` | 66% | 满铺深蓝底，不要自己加圆角，iOS 会裁 |
| `splash-icon.png` | 66% | 深蓝圆角方块，圆角半径是边长的 27.7%，方块外透明 |
| `adaptive-icon.png` | 48% | 只有图形，透明底；保证落在 Android 66% 安全区内 |

把图形放进一个正方形 SVG、占 66% 时，可以直接用这个 viewBox，不需要再算变换：

```
viewBox="-12.08 -13.83 91.67 91.67"
```

## 4. 待办任务

### 任务 A：app 内 Logo 组件换成新图形（必做）

位置：`mobile/src/components/ui/index.tsx:20`，`Logo` 组件。现在是深蓝圆角方块里放一个白色字母 "P"。

改法：

- 保留外层 `Pressable` 及其全部 props 和行为：`onPress`、`lg`、`testID`、`disabled={!onPress}`、`accessibilityRole`。
- 保留尺寸和圆角：默认 36 见方、圆角 10；`lg` 时 56 见方、圆角 16；底色 `colors.fg`（就是 `#13233F`）。
- 把里面的 `<Text>P</Text>` 换成 `react-native-svg` 画的图形。项目已经装了 `react-native-svg` 15.16.0，不需要加依赖。
- `Svg` 宽高等于方块边长，`viewBox` 用第 3 节给的值，四个图元照抄第 3 节。
- 房子的 `fill` 和 `stroke` 用 `colors.fg`，橙色用字面量 `#E8622C`。注意主题里的 `colors.accent` 是 `#FF6B2C`，是另一个颜色，不要用它。

约束：

- `Logo` 被 `Home.tsx`、`Login.tsx`、`Upcoming.tsx`、`dk/DK.tsx`、`ofi/OfiList.tsx` 使用，签名不能变。
- e2e 用到 `testID="open-me"`，不能丢（见 `mobile/e2e/README.md:103`）。
- 这个文件的代码风格是一行一个组件，改动保持同样风格。

验证：在 `mobile/` 下跑 `npm run typecheck`、`npm run lint`、`npm test`。

### 任务 B：构建验证并提交（必做）

1. 确认 `mobile/assets/` 下四个文件的改动都在工作区（`git status --short mobile/assets`）。
2. 图标和启动页要重新打原生包才生效，Expo Go 和热更新看不到。按现有流程出一个包，在真机上检查：
   - iOS 主屏图标、设置和搜索里的小图标；
   - Android 主屏图标，圆形和圆角方两种遮罩下图形都没有被裁；
   - 启动页：浅灰底 `#F6F7FA` 上居中显示深蓝圆角方块，宽 120。
3. iOS Ad Hoc 的安装页不用改代码：`.github/workflows/mobile-ios-adhoc.yml:180-181` 在 CI 里用 `sips` 从 `assets/icon.png` 生成 `icon-57.png` 和 `icon-512.png`，下次跑工作流会自动带上新图标。跑完后打开安装页确认一下。
4. 提交。发布流程按 `/root/.pi/agent/AGENTS.md` 的"服务器访问"一节走。

### 任务 C：后台 web 的 favicon 和 logo（待确认，先不要做）

后台现在用的是另一套标识：金色房子，品牌名写的是 "Place Sunnybank"（中介公司自己的名字），不是 Axleo。位置：

- `web/index.html:7`：内联 SVG favicon，深蓝 `#0f2a44` 底配金色 `#c9a227` 房子；
- `web/src/App.vue:7`：侧边栏 `.logo`；
- `web/src/views/AdminLogin.vue:5`：登录页 `.logo`；
- `web/src/views/Cockpit.vue:5`：`.logo`，金色是 `#e6c35c`。

要不要统一成新图标是产品决定，没有结论之前不要动。

## 5. 不要做的事

- 不要改 `mobile/app.json` 里的图标路径和背景色。
- 不要动客户品牌包 `Axleo-Brand-Kit.zip` 里的文件。
- 不要顺手调整安装页 `mobile/ci/install/index.html` 的配色，它不在这次范围内。

## 6. 未决事项

- 新图标偏离了客户给的 Brand Kit，**还没有得到客户确认**。上架前需要客户点头；如果客户不同意，用 `git checkout -- mobile/assets` 即可恢复原图标（`icon-source.svg` 是新增文件，需要手动删）。
- 品牌包里 Collect 产品的官方图标是金色底 `#E3A23B` 配白色图形，app 一直用的是深蓝底。这次沿用深蓝底，没有改这一点。

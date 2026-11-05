# 语音转文字（STT）供应商调研：价格、语言、数据存放地

> 对应 `feedback-q11-answers.md` 中的 **App Q2** 和待办 **R4**。
> 抓取日期：**2026-11-05**（UTC）。价格均来自官网页面或官方价格 API，原文币种照录；没有核实的数字标 **未核实**。
> 汇率（ECB 数据，经 frankfurter.dev，2026-11-04）：**1 USD = 1.4255 AUD**，**1 USD = 6.6723 CNY**，**1 CNY = 0.21364 AUD**。
> 本文不是法律意见。APP 8 / PIPL 的结论需要律师确认。

---

## 1. 结论与推荐

### 前三名（适合"门口口述、中英粤混说、澳洲个人信息"）

| 排名 | 方案 | 为什么 | 每音频小时（AUD） | 主要风险 |
|---|---|---|---|---|
| **1** | **Microsoft Azure AI Speech，Australia East** | Australia East 支持实时转写、Fast transcription 和批量转写（官方区域表已核实）；语言有 en-AU、zh-CN、zh-HK、yue-CN；企业合同和 DPA 成熟；按秒计费 | 实时 1.43；Fast 0.51；批量 0.26 | 每次请求一般要指定 locale，**同一句话里中英混说的效果要实测**（可以用连续语种识别或多语言模式，但对 zh/en 句内切换没有专门承诺）；实时转写最贵 |
| **2** | **Deepgram Nova-3 Multilingual，AU 端点 `api.au.deepgram.com`** | 官方保证请求**只在澳洲处理、不会回退到其他区域**；`language=multi` 支持句内切换，Nova-3 多语种包含普通话、粤语（zh-HK）和 en-AU；按秒计费，没有最低时长；有 200 USD 赠送额度 | 预录 0.45；流式 0.79 | 默认参加 Model Improvement Program，**必须每个请求加 `mip_opt_out=true`**；中文和粤语的实际准确率未知 |
| **3** | **Speechmatics，AU1（`au1.asr.api.speechmatics.com` / `au.rt.speechmatics.com`）** | 有澳洲区域，并且 Melia 1 在 AU1 可用；Melia 1 能自动切换语种（句中切换）；Enhanced 模型有 `cmn_en`（普通话+英语双语包），也支持粤语 `yue`；按秒计费；价格最低 | Melia 1：预录 0.17，实时 0.29；Enhanced：0.54 / 0.57 | Melia 1 的 56 种语言具体包括哪些（是否包括粤语）没有逐条核实；`cmn_en` 双语包不包括粤语 |

**备选（应该一起参加实测）：**
- **AWS Transcribe，悉尼 ap-southeast-2**：数据在澳洲，价格低（批量 0.51 AUD/h），支持 en-AU、zh-CN、zh-HK（粤语），并有"多语种识别（multi-language identification）"。缺点是中英句内混说的效果一般需要实测。
- **OpenAI gpt-4o-transcribe / gpt-4o-mini-transcribe**：业内普遍认为中英混说效果较好（**未核实，需实测**）。但它的"澳洲数据驻留"**只保证存储在澳洲，推理不在澳洲**（官方表格写的是 Processing: No），而且需要销售审批 MAM/ZDR。所以按 APP 8 仍然属于跨境披露，只建议用于对比测试，或在合同到位后再用。
- **中国大陆厂商（阿里、腾讯、讯飞、百度、火山）**：中文、粤语、方言和中英混说是它们的强项，价格也最低，**但数据在中国大陆处理**。把澳洲居民的个人信息发过去，属于 APP 8 跨境披露，还会进入 PIPL 和数据出境制度的管辖范围。**不建议用于生产环境**。阿里云国际站新加坡（"International"部署范围）可以作为测试基准，但仍然是境外处理，而且供应商是中国公司。

### 必须实测：20–30 段测试方案（来自 App Q2）

价格和语言列表都不能说明**混说的实际准确率**。建议这样测：

| 类别 | 数量 | 内容 |
|---|---|---|
| A 澳洲英语 | 6 | 门口户外（风、车流），澳洲口音，包含门牌、街名（Sunnybank、Mains Rd 等）、电话号码、价格（"1.2 million"） |
| B 普通话夹英语 | 8 | "明年 maybe sell"、"他说 open home 周六"、"budget 大概 one point five" |
| C 粤语夹英语 | 6 | 粤语为主，夹英语房产词汇和数字 |
| D 纯普通话 | 3 | 安静和嘈杂各有 |
| E 极端噪声或短句 | 3–5 | 10 秒左右的短句、强风、多人说话 |

- **录音来源：**由员工或志愿者按真实场景扮演，**不要用真实客户的录音**，避免在选型阶段就发生跨境披露。用手机 App 实际的采样率和编码录音。
- **参考文本：**由双语人员人工转写，统一简繁体和数字写法。
- **指标：**混合错误率 MER（中文按字算 CER，英文按词算 WER）；**关键字段准确率**（姓名、电话、地址、价格、时间）；延迟（实时转写看首字和最终结果的时间，批量看整段返回时间）；按第 6 节估算的成本；交给 LLM 抽取字段后的最终字段正确率。
- **候选和配置：**
  - Azure：分别测 en-AU、zh-CN、zh-HK 单语，以及连续语种识别
  - Deepgram：nova-3 `language=multi`，AU 端点
  - Speechmatics：Melia 1，以及 Enhanced 的 `cmn_en`，AU1
  - AWS：multi-language ID，悉尼
  - OpenAI：gpt-4o-transcribe 和 mini（只用测试数据）
  - 可选：阿里 qwen3-asr-flash 新加坡（只用测试数据）
- 每个配置跑全部音频；记录配置和版本；结果写到 `docs/` 下。

---

## 2. 国际供应商对比表

"每音频小时"一列由原价换算（÷60 或 ×3600），AUD 按上面的汇率折算。

| 供应商 / 模型 | 英语（en-AU） | 普通话 | 粤语 | 中英句内混说 | 流式 / 批量 | 官方计费单位与标价 | 每音频小时（原币） | ≈AUD/h | 免费额度 | 澳洲区域 |
|---|---|---|---|---|---|---|---|---|---|---|
| **Azure AI Speech** 标准实时（S1） | ✅ en-AU | ✅ zh-CN | ✅ zh-HK、yue-CN | 有语种识别和多语言模式；句内切换**需实测** | 流式 | **按秒计费**，标价 1.00 USD/小时 | 1.00 USD | 1.43 | F0 层每月 5 小时 | ✅ Australia East（实时、Fast、批量均支持） |
| Azure Fast transcription | ✅ | ✅ | ✅ | 同上；Azure LLM Speech 可自动检测语种 | 同步批量（快速返回） | 0.36 USD/小时（另有 Promo 档 0.10 USD/小时，生效日期 2026-09-01，结束日期未知） | 0.36 USD | 0.51 | — | ✅ |
| Azure 批量转写 | ✅ | ✅ | ✅ | 同上 | 异步批量 | 0.18 USD/小时 | 0.18 USD | 0.26 | — | ✅ |
| **AWS Transcribe** 批量 | ✅ en-AU | ✅ zh-CN | ✅ zh-HK（yue-HK） | 有 multi-language identification（批量和流式都支持），输出多语种转写 | 批量 | **按秒，无最低时长**；悉尼 0.0001 USD/秒（= 0.006 USD/分钟） | 0.36 USD | 0.51 | 前 12 个月每月 60 分钟 | ✅ ap-southeast-2 悉尼 |
| AWS Transcribe 流式 | ✅ | ✅ | ✅ | 同上 | 流式 | 悉尼 0.0001667 USD/秒（= 0.01 USD/分钟） | 0.60 USD | 0.86 | 同上 | ✅ |
| **Google Cloud STT V2**（含 Chirp / Chirp 2 / Chirp 3） | ✅ en-AU | ✅ cmn-Hans-CN | ✅ yue-Hant-HK（Chirp 3 中为 **Preview**） | Chirp 3 可用 `auto` 自动检测"主要语言"，**句内切换没有承诺** | 流式、同步、批量 | **按分钟标价，每次请求向上取整到 1 秒**；0–50 万分钟 0.016 USD/分钟；Dynamic Batch 0.003 USD/分钟 | 0.96 USD（动态批量 0.18） | 1.37（0.26） | 每月 60 分钟 | ❌ **没有澳洲区域**。STT V2 的区域只有 us、eu、us-central1、europe-west4、asia-southeast1（新加坡，Chirp/Chirp 2）、global；Chirp 3 只在 us 和 eu 多区域 |
| **OpenAI gpt-4o-transcribe** | ✅（不分地区口音） | ✅ | 支持（**未核实**） | 普遍认为较好（**未核实，需实测**） | 文件上传；Realtime 另计 | 按 token：输入 2.50、输出 10.00 USD/百万 token；官方估算 0.006 USD/分钟 | ≈0.36 USD | ≈0.51 | — | ⚠️ `au.api.openai.com`：**只在澳洲存储，推理不在澳洲**；需要 MAM 或 ZDR 审批（联系销售） |
| OpenAI gpt-4o-mini-transcribe | ✅ | ✅ | 同上 | 同上 | 同上 | 输入 1.25、输出 5.00 USD/百万 token；估算 0.003 USD/分钟 | ≈0.18 USD | ≈0.26 | — | 同上 |
| OpenAI gpt-transcribe（新） | ✅ | ✅ | **未核实** | **未核实** | 文件 | 估算 0.0045 USD/分钟 | ≈0.27 USD | ≈0.38 | — | 同上；数据驻留端点对 2026-03-05 之后发布的模型加价 10% |
| OpenAI whisper-1 | ✅ | ✅ | 支持（**未核实**） | 一般 | 文件 | 0.006 USD/分钟 | 0.36 USD | 0.51 | — | 同上 |
| OpenAI gpt-live-transcribe / gpt-realtime-whisper | ✅ | ✅ | **未核实** | **未核实** | 实时 | 0.017 USD/分钟 | 1.02 USD | 1.45 | — | 同上 |
| **Deepgram Nova-3 Multilingual** | ✅ en-AU | ✅ zh / zh-CN | ✅ zh-HK | ✅ 文档写明 `language=multi` 支持句内切换（Nova-3 多语种共 55 种） | 预录和流式 | **按秒，无最低时长，不按块取整**；按需付费：预录 0.0052、流式 0.0092 USD/分钟（Growth 档 0.0043 / 0.0078） | 预录 0.312 / 流式 0.552 USD | 0.45 / 0.79 | 200 USD 赠送额度，不过期 | ✅ `api.au.deepgram.com`，**保证在澳洲处理，不回退到其他区域**（Whisper 模型在 AU 端点不可用） |
| Deepgram Nova-3 Monolingual | ✅ | ✅（单语） | ✅（单语） | ❌ | 预录和流式 | 预录 0.0043、流式 0.0077 USD/分钟 | 0.258 / 0.462 USD | 0.37 / 0.66 | 同上 | ✅ |
| **AssemblyAI Universal-3.5 Pro** | ✅ Australian English | ✅ Mandarin | ❌（只有 3.6 Pro Preview 和 Universal-2 支持粤语） | ✅ 原生句内切换，官方示例里有英语和普通话 | 异步批量 | 0.21 USD/小时 | 0.21 USD | 0.30 | 预录 185 小时，流式 333 小时 | ❌ 只有 US（默认）和 EU |
| AssemblyAI Universal-2 | ✅ | ✅ | ✅ yue | 有，需开启 `code_switching`（旧版） | 异步批量 | 0.15 USD/小时 | 0.15 USD | 0.21 | 同上 | ❌ |
| AssemblyAI Universal-3.6 Pro Realtime | ✅ | ✅ | ✅（32 种语言之一） | **未核实** | 流式 | 0.45 USD/小时（Universal-Streaming 0.15 USD/小时，只有英语和 6 种欧洲语言） | 0.45 USD | 0.64 | 同上 | ❌ |
| **Speechmatics Melia 1** | ✅（Global English，包含澳洲口音） | ✅ | 是否在 Melia 的 56 种语言内**未逐条核实** | ✅ "switches language mid-sentence" | 预录和实时 | **按秒计费（以 credit 结算）**；预录 0.12 USD/小时，实时 0.20 USD/小时 | 0.12 / 0.20 USD | 0.17 / 0.29 | 100 USD 赠送额度 | ✅ AU1（Melia 1 在 EU1、US1、AU1 可用） |
| Speechmatics Enhanced | ✅ | ✅ `cmn` | ✅ `yue` | ✅ `cmn_en` 普通话加英语双语包（不包括粤语） | 预录和实时 | 0.38 / 0.40 USD/小时（Standard 0.23 USD/小时） | 0.38 / 0.40 USD | 0.54 / 0.57 | 同上 | ✅ AU1 |
| **ElevenLabs Scribe v2** | ✅ | ✅ | ✅（文档提到 Cantonese） | 有"Smart language detection"；句内切换**未核实** | 批量 | 0.22 USD/小时（keyterm 加 0.05 USD/小时） | 0.22 USD | 0.31 | 订阅附带额度 | ❌ 默认美国；企业版可选 EU、印度、新加坡 |
| ElevenLabs Scribe v2 Realtime | ✅ | ✅ | ✅ | **未核实** | 实时 | 0.39 USD/小时 | 0.39 USD | 0.56 | 同上 | ❌ |
| **Rev.ai** Reverb Foreign Language | 英语用 Reverb 0.20 USD/小时 | ✅ 页面写 Chinese | **未核实** | **未核实** | 批量 | 0.30 USD/小时；**按秒取整，每次最低 15 秒** | 0.30 USD | 0.43 | 有免费额度（数量**未核实**） | ❌（区域**未核实**） |

---

## 3. 中国大陆供应商对比表

注："元"指人民币。"每日用量梯度"表示按当天总用量所在档位，对当天全部用量统一计价。

| 供应商 / 产品 | 普通话 | 粤语 | 英语 | 中英混说 | 流式 / 批量 | 官方计费单位与标价 | 每音频小时（元） | ≈AUD/h | 免费额度 | 资源包（节选） | 数据地域 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| **阿里云 智能语音交互** 实时语音识别 | ✅ | ✅（模型可选，**未核实**） | ✅ | **未核实** | 流式 | 按小时，**每日用量梯度**：0–299 小时 3.50 元/小时，300–999 小时 3.00，…，5000 小时以上 1.20 | 3.50 | 0.75 | 新用户部分服务试用 3 个月 | 30 小时 100 元；1,000 小时 1,800 元（1.80 元/小时）；1 年有效 | 中国大陆 |
| 阿里云 录音文件识别 / 极速版 / 闲时版 | ✅ | — | ✅ | — | 批量 | 2.50 / 3.30 / 1.00 元/小时（每日梯度） | 2.50 | 0.53 | 同上 | 40 小时 100 元；1,000 小时 1,200 元 | 中国大陆 |
| 阿里云 一句话识别 | ✅ | — | ✅ | — | 短句（单次时长上限**未核实**，记忆为 60 秒） | **按次**：3.50 元/千次 | 不按时长计 | — | — | 30 千次 100 元 | 中国大陆 |
| **阿里云百炼 Fun-ASR / qwen3-asr-flash**（录音文件） | ✅ | 支持方言和粤语（**未核实**） | ✅ | 支持多语种混合（**未核实**） | 批量 | **按秒**：北京 0.00022 元/秒；新加坡 0.00026 元/秒（国际站 0.000035 USD/秒）；qwen3-asr-flash 另有美国弗吉尼亚 0.000035 USD/秒 | 北京 0.792 元；新加坡 0.126 USD | 0.17；0.18 | 10 小时（36,000 秒），90 天内有效 | 无 | 北京 = 中国大陆；新加坡、美国 = 境外 |
| 百炼 fun-asr-realtime / qwen3-asr-flash-realtime | ✅ | **未核实** | ✅ | **未核实** | 流式 | 北京 0.00033 元/秒；新加坡 0.00066 元/秒（0.00009 USD/秒） | 北京 1.188 元；新加坡 0.324 USD | 0.25；0.46 | 10 小时，90 天 | 无 | 同上 |
| 百炼 Paraformer-v2 / realtime-v2 | ✅ | **未核实** | ✅ | **未核实** | 批量 / 流式 | 0.00008 / 0.00024 元/秒（**只有北京**） | 0.288 / 0.864 元 | 0.06 / 0.18 | **每月** 10 小时 | 无 | 中国大陆 |
| **腾讯云 ASR** 录音文件识别（大模型 2.0 版） | ✅ | 见右 | ✅ | `16k_zh_en` 引擎：中文、英语、粤语和多种方言（官方文档） | 批量 | 按小时：0.80 元/小时（无梯度，月结） | 0.80 | 0.17 | 标准录音文件识别每月 10 小时（大模型版无免费额度） | 60 小时 48 元；1,000 小时 780 元 | 中国大陆 |
| 腾讯云 实时语音识别（大模型 2.0 版） | ✅ | ✅ | ✅ | 同上 | 流式 | 1.0 元/小时（日结）；**跨境版** 2.5 元/小时 | 1.0（跨境 2.5） | 0.21（0.53） | 无 | 60 小时 60 元；1,000 小时 950 元 | 中国大陆。"跨境"指在国内站为海外客户提供服务 |
| 腾讯云 实时语音识别（标准） | ✅ | ✅ `16k_yue` | ✅ `16k_en` | 单语引擎 | 流式 | 每日梯度 3.20 元/小时起；跨境 9.94 元/小时 | 3.20 | 0.68 | 每月 5 小时 | 30 小时 90 元；1,000 小时 1,800 元 | 中国大陆 |
| 腾讯云 录音文件识别（标准） | ✅ | ✅ | ✅ | — | 批量 | 每月梯度：0–12 万小时 1.75 元/小时 | 1.75 | 0.37 | 每月 10 小时 | 60 小时 90 元；1,000 小时 1,200 元 | 中国大陆 |
| 腾讯云 一句话识别 | ✅ | ✅ | ✅ | — | 短句 | **按次**：3.20 元/千次（每日梯度） | — | — | 每月 5,000 次 | 30 千次 90 元 | 中国大陆 |
| **科大讯飞** 实时语音转写大模型 | ✅ | ✅（"中英文 + 202 种方言免切"） | ✅ | ✅ 页面写"方言、语种免切" | 流式 | **只卖时长套餐**（1 年有效，同时限制并发路数）：40 小时 198 元；1,000 小时 4,000 元；5,000 小时 17,500 元；…；更大用量商务咨询 | 4.95（40 小时包）/ 4.00（1,000 小时包） | 1.06 / 0.85 | 个人 5 小时，企业 50 小时（1 年） | 见左 | 中国大陆（国际站**未核实**） |
| 讯飞 录音文件转写大模型 | ✅ | ✅ | ✅ | ✅（"方言、语种免切"） | 批量 | 时长套餐：80 小时 198 元；2,000 小时 4,000 元；10,000 小时 17,500 元 | 2.48 / 2.00 | 0.53 / 0.43 | 个人 5 小时，企业 50 小时 | 见左 | 中国大陆 |
| 讯飞 语音识别大模型（≤60 秒短音频） | ✅ | ✅ | ✅ | ✅ | 短句 | **按次套餐**：中文 100 万次 2,300 元；多语种 40 万次 1,680 元 | — | — | 个人 2 万次，企业 20 万次（页面另有"36 元"档，含义**未核实**） | — | 中国大陆 |
| 讯飞 实时转写标准版 / 语音听写 | ✅ | 方言需另开（**未核实**） | ✅ | — | 流式 | 标准版：40 小时 198 元，600 小时 2,000 元；听写：50 万次 1,300 元 | 4.95 / 3.33 | — | 5 小时 / 50 小时 | — | 中国大陆 |
| **百度智能云** 实时语音识别（普通话、英语） | ✅ | 价格页没列出粤语 | ✅ | **未核实** | 流式 | 后付费 3 元/小时（另一档 3.3 元/小时，对应哪个接口页面未能确认） | 3.0 | 0.64 | 有测试额度（数量**未核实**） | 1,000 小时 1,800 元 | 中国大陆 |
| 百度 音频文件转写 | ✅ | — | ✅ | **未核实** | 批量 | 后付费 2 元/小时 | 2.0 | 0.43 | 同上 | 1,000 小时 1,200 元 | 中国大陆 |
| 百度 短语音识别标准版 | ✅ | ✅ | ✅ | ❌（每次请求选一种语言） | 短句 | **按次**，每月阶梯：≤6,000 千次 3.4 元/千次（极速版 4.2） | — | — | 同上 | 1,000 千次 2,400 元 | 中国大陆 |
| **火山引擎** 豆包流式语音识别模型 2.0 | ✅ | ✅（默认模型包括粤语和多种方言） | ✅（文档只写 en-US） | ✅ 默认就是"中英文"模型 | 流式 | 1 元/小时 | 1.0 | 0.21 | **未核实** | 30 小时 28 元；1,000 小时 900 元 | 中国大陆（BytePlus 国际版**未核实**） |
| 火山 豆包录音文件识别模型 2.0 | ✅ | ✅ | ✅ | ✅ | 批量 | 0.8 元/小时 | 0.8 | 0.17 | **未核实** | 30 小时 23 元；1,000 小时 750 元 | 中国大陆 |
| 火山 大模型流式 / 录音文件识别（1.0） | ✅ | ✅ | ✅ | ✅ | 流式 / 批量 | 4.5 / 2.3 元/小时 | 4.5 / 2.3 | 0.96 / 0.49 | — | 1,000 小时 4,000 / 2,000 元 | 中国大陆 |

---

## 4. 计费方式说明

| 计费方式 | 采用的供应商 | 对本场景（10–90 秒短音频）的影响 |
|---|---|---|
| **按秒**（无最低时长） | AWS（官方原话："one-second increments, no minimum"）、Azure（"per second billing"）、Google V2（每次请求向上取整到 1 秒）、Deepgram（明确不按块取整）、Speechmatics（"billed to the second"）、阿里百炼（元/秒）、OpenAI whisper（按分钟标价，实际计量**未核实**） | 最适合短音频，没有浪费 |
| **按秒取整、单次最低 15 秒** | Rev.ai | 10–14 秒的短句会按 15 秒算；平均 45 秒时影响很小 |
| **按音频 token** | OpenAI gpt-4o(-mini)-transcribe（输入音频 token 加输出文本 token）；阿里 qwen-audio-3.1 系列 | 成本随说话密度和输出长度变化，只能用官方"估算每分钟"价格 |
| **按小时，每日用量梯度** | 阿里智能语音交互、腾讯实时识别 | 按当天总用量所在档位统一计价；本场景每天约 7.5 小时，始终在最低档 |
| **按小时，每月用量梯度** | 腾讯录音文件识别（0–12 万小时 1.75 元） | 同上，始终在最低档 |
| **按次 / 千次** | 阿里和腾讯一句话识别、百度短语音、讯飞语音识别大模型和听写 | 13,200 次/月 × 约 3.2–3.5 元/千次，约 42–46 元/月；**单次有时长上限（常见 60 秒，未核实）**，90 秒的音频要切段 |
| **预付时长包或次数包**（1 年有效） | 讯飞（只卖套餐）、腾讯、阿里、百度、火山 | 必须预估全年用量；讯飞包同时限定并发路数 |
| **并发 / QPS 月费** | 腾讯：实时识别 100–200 元/路/月；阿里极速版超过 10 路后 200 元/路/月；火山有并发增购包 | 默认并发够用时不产生；高峰需要多路时要加钱 |
| **Commitment tier（承诺用量）** | Azure（例如 2K 小时/月档） | 本场景约 165 小时/月，用不上 |

---

## 5. 数据存储与合规

### 5.1 国际供应商

| 供应商 | 能否在澳洲处理 | 默认数据保留 / 训练 | 能否关闭 | 结论 |
|---|---|---|---|---|
| **Azure Speech** | ✅ Australia East：实时、Fast、批量转写都支持（官方区域表） | 实时和 Fast 转写不保存音频（**未核实，来自记忆**）；批量转写的结果放在自己指定的存储里 | 企业 DPA | **首选**：在澳洲处理，资源归自己的订阅 |
| **AWS Transcribe** | ✅ 悉尼 | AWS 可能用内容改进 AI 服务（**未核实，来自记忆**） | 可以在 AWS Organizations 设置 AI services opt-out policy（**未核实**） | 可用；上线前必须设置 opt-out |
| **Google STT V2** | ❌ 没有澳洲区域（最近的是新加坡 asia-southeast1，只有 Chirp/Chirp 2） | V1 价格表区分 "with data logging"（0.016 USD/分钟）和 "without data logging"（0.024 USD/分钟） | 可以不开 data logging | 属于跨境（新加坡、美国或欧盟） |
| **OpenAI** | ⚠️ `au.api.openai.com`：**只在澳洲存储，推理不在澳洲** | `/v1/audio/transcriptions`：不用于训练；滥用监控保留为 "None"；可开 ZDR | 澳洲驻留需要 MAM 或 ZDR 审批，以及修改合同 | 属于跨境（推理在境外） |
| **Deepgram** | ✅ `api.au.deepgram.com`，**保证在澳洲处理，不回退** | **默认参加 Model Improvement Program**（会保留部分数据用于训练） | 每个请求加 `mip_opt_out=true`，之后只在处理期间保留。opt-out 是否加价**未核实** | 可用；必须加 opt-out |
| **AssemblyAI** | ❌ 只有 US 和 EU；账户和用量数据存在美国；排障时可能由境外人员访问 | — | — | 跨境 |
| **Speechmatics** | ✅ AU1（批量和实时都有） | **未核实** | **未核实** | 可用；要看 DPA |
| **ElevenLabs** | ❌ 默认美国；企业版可选 EU、印度、新加坡 | 有 Zero Retention Mode（企业或医疗场景） | 企业版 | 跨境 |
| **Rev.ai** | ❌（**未核实**） | **未核实** | — | 跨境 |

**APP 8 的要点：**数据只要在澳洲境外被"访问或处理"，一般就算跨境披露（"只存储在澳洲"并不够）。APP entity 要合理确保境外接收方遵守 APP，并且要对境外接收方的违规承担责任（s16C）。在澳洲处理的选项（Azure Australia East、AWS 悉尼、Deepgram AU、Speechmatics AU1）能大大降低这方面的风险。不论选哪家，隐私政策都要写明使用第三方语音转写服务，并签 DPA。

### 5.2 中国大陆供应商

| 供应商 | 境内处理 | 境外节点 | 风险 |
|---|---|---|---|
| 阿里云百炼 | 北京（华北 2） | **有**：新加坡（"International"部署范围）、美国弗吉尼亚（只有 qwen3-asr-flash）；国际站有美元价格 | 用国际站新加坡：属于 APP 8 跨境（新加坡），但不经过中国大陆；供应商是中国公司，要评估中国法律下的数据调取风险 |
| 阿里云智能语音交互（NLS） | 中国大陆 | 国际站有没有同等服务**未核实** | 境内 |
| 腾讯云 ASR | 中国大陆（示例区域 ap-shanghai） | 国际站 tencentcloud.com 有 ASR 文档，价格**未抓取**；国内站的"跨境"版只是"为海外客户提供服务"的定价，**不等于在境外处理** | 境内 |
| 科大讯飞 | 中国大陆 | 国际站**未核实** | 境内 |
| 百度智能云 | 中国大陆 | **未核实** | 境内 |
| 火山引擎 | 中国大陆 | BytePlus 国际版有没有 ASR **未核实** | 境内 |

> ⚠️ **风险提示：**把澳洲居民的语音（包含姓名、电话、地址、出售意向等个人信息）发到中国大陆服务器：
> 1. 属于 **APP 8 跨境披露**，agentCollect 和中介公司要对境外违规负责；隐私政策还必须披露数据会发往中国。
> 2. 数据进入中国境内后，受 **PIPL、数据安全法和网络安全法**管辖（包括政府调取数据的可能性）；如果以后需要把转写结果传回澳洲，还可能涉及**数据出境**规则（安全评估、标准合同或认证。豁免条件要律师确认）。
> 3. 涉及华人社区客户，舆论和信任风险也较高。
> **建议：**大陆厂商只用于**不含真实个人信息**的测试对比，不进入生产环境。

---

## 6. 场景月费估算

场景：20 名中介 × 每天 30 段 × 平均 45 秒 × 每月 22 个工作日 = **每月 13,200 段，594,000 秒 = 165 音频小时**。
上面这些供应商大多按秒计费，所以计费时长就是 165 小时。Rev 每段最低 15 秒，平均 45 秒时基本不增加费用；如果有很多 10 秒左右的短句，最多多算约 50%（这部分短句）。下表**没有扣除免费额度**，用标价，汇率见文首。

| 方案 | 单价 | 月费（原币） | 月费 ≈AUD | 备注 |
|---|---|---|---|---|
| Speechmatics Melia 1 预录（AU1） | 0.12 USD/h | 19.80 USD | **28** | 实时 0.20 USD/h：33 USD，约 47 AUD |
| 阿里百炼 fun-asr / qwen3-asr 新加坡 | 0.126 USD/h | 20.79 USD | 30 | 跨境 |
| AssemblyAI Universal-3.5 Pro | 0.21 USD/h | 34.65 USD | 49 | 跨境（US/EU） |
| ElevenLabs Scribe v2 | 0.22 USD/h | 36.30 USD | 52 | 跨境 |
| Azure 批量（Australia East） | 0.18 USD/h | 29.70 USD | **42** | 异步，延迟可能是分钟级 |
| Azure Fast transcription（Australia East） | 0.36 USD/h | 59.40 USD | **85** | 适合"录完马上出结果" |
| Azure 实时（Australia East） | 1.00 USD/h | 165.00 USD | **235** | 边说边出字 |
| AWS Transcribe 批量（悉尼） | 0.36 USD/h | 59.40 USD | 85 | 流式 0.60 USD/h：99 USD，约 141 AUD |
| Deepgram Nova-3 Multi 预录（AU） | 0.312 USD/h | 51.48 USD | **73** | 流式 0.552 USD/h：91 USD，约 130 AUD；200 USD 赠送额度约够前 3.9 个月 |
| OpenAI gpt-4o-transcribe | ≈0.36 USD/h | ≈59.40 USD | 85 | mini 约 30 USD，约 42 AUD |
| Google STT V2 标准 | 0.96 USD/h | 158.40 USD | 226 | Dynamic Batch 约 30 USD，约 42 AUD（延迟高） |
| Rev.ai Reverb Foreign Language | 0.30 USD/h | 49.50 USD | 71 | 15 秒最低计费 |
| 腾讯 录音文件识别大模型 2.0 | 0.8 元/h | 132 元 | 28 | 境内 |
| 腾讯 实时识别大模型 2.0 | 1.0 元/h | 165 元 | 35 | 境内 |
| 火山 豆包录音文件识别 2.0 / 流式 2.0 | 0.8 / 1.0 元/h | 132 / 165 元 | 28 / 35 | 境内 |
| 阿里 智能语音交互 实时 | 3.5 元/h | 577.5 元 | 123 | 买 1,000 小时包（1,800 元/年）折合约 300 元/月，约 64 AUD |
| 讯飞 实时转写大模型 | 全年约 1,980 小时，需要 2 个 1,000 小时包，共 8,000 元/年 | 约 667 元/月 | 约 142 | 录音文件转写大模型：2,000 小时包 4,000 元/年，约 333 元/月，约 71 AUD |
| 百度 实时 / 音频文件转写 | 3.0 / 2.0 元/h | 495 / 330 元 | 106 / 71 | 境内 |
| 按次计费（阿里、腾讯一句话识别、百度短语音） | 3.2–3.5 元/千次 | 42–46 元 | 9–10 | 单次有时长上限（**未核实**），90 秒音频要切段 |

**结论：**本场景用量很小，各家月费都在 **30–240 AUD** 之间。**价格不是决定因素，数据地域和混说准确率才是。**推荐的三家（Azure、Deepgram AU、Speechmatics AU1）月费约 28–235 AUD，差别主要在用实时还是批量。

---

## 7. 来源与核实状态

抓取日期都是 2026-11-05（UTC），汇率数据日期是 2026-11-04。✅ = 从官方页面或 API 直接读到；⚠️ = 部分内容未读到；❌ = 页面打不开，或价格不公开。

| 供应商 | 来源 URL | 状态 |
|---|---|---|
| Azure 价格 | `https://prices.azure.com/api/retail/prices?$filter=armRegionName eq 'australiaeast' and contains(productName,'Speech')`（Retail Prices API，Australia East，USD） | ✅（S1 STT 1.00、Batch 0.18、Fast 0.36、Fast Promo 0.10、Custom 1.20 USD/h）。官网价格页 `azure.microsoft.com/.../speech-services/` 的价格是前端渲染的，静态 HTML 显示为 "$-"，但能确认"per second billing"和 F0 每月 5 小时 |
| Azure 语言 / 区域 | `learn.microsoft.com/en-us/azure/ai-services/speech-service/language-support`、`.../regions` | ✅ en-AU、zh-CN、zh-HK、yue-CN；australiaeast 支持实时、Fast、批量 |
| AWS 价格 | `https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/transcribe/current/region_index.json`，以及 ap-southeast-2 的 index（publicationDate 2026-09-11）；`aws.amazon.com/transcribe/pricing/` | ✅ 悉尼批量 0.0001 USD/秒、流式 0.0001667 USD/秒；按秒、无最低时长；免费 60 分钟/月（12 个月） |
| AWS 语言 | `docs.aws.amazon.com/transcribe/latest/dg/supported-languages.html`、`lang-id-batch.html`、`lang-id-stream.html` | ✅ |
| AWS 数据 opt-out | — | **未核实**（来自记忆） |
| Google 价格 | `cloud.google.com/speech-to-text/pricing` | ✅（V2 0.016 USD/分钟起；Dynamic Batch 0.003；按 1 秒取整；60 分钟免费） |
| Google 区域 / 语言 | `cloud.google.com/speech-to-text/v2/docs/speech-to-text-supported-languages`、`.../chirp_3-model` | ✅（没有澳洲区域；Chirp 3 只在 us/eu；粤语为 Preview）。`/docs/locations` 页面只介绍 Locations API，没有静态列表 |
| OpenAI 价格 | `platform.openai.com/docs/pricing` | ✅（`openai.com/api/pricing/` 被拦截，没有内容） |
| OpenAI 数据 | `platform.openai.com/docs/guides/your-data` | ✅（澳洲：Storage Yes / Processing No；transcriptions 符合 ZDR 条件） |
| OpenAI 粤语、混说效果 | — | **未核实** |
| Deepgram 价格 | `deepgram.com/pricing`（页面内嵌的 schema.org Offer 数据） | ✅（第一次连接 SSL 失败，重试后成功） |
| Deepgram 区域 / 语言 / MIP | `developers.deepgram.com/reference/regional-endpoints.md`、`/docs/models-languages-overview`、`/docs/multilingual-code-switching`、`/docs/the-deepgram-model-improvement-partnership-program.md` | ✅。`/docs/deepgram-australia-endpoint` 返回 404。MIP opt-out 是否影响价格**未核实** |
| AssemblyAI | `assemblyai.com/pricing`、`/docs/pre-recorded-audio/select-the-region.md`、`/docs/pre-recorded-audio/code-switching.md`、`/docs/pre-recorded-audio/supported-languages` | ✅ |
| Speechmatics | `speechmatics.com/pricing`、`docs.speechmatics.com/speech-to-text/languages`、`docs.speechmatics.com/get-started/authentication` | ✅（Melia 1 具体支持哪些语言没有逐条核实；数据保留**未核实**） |
| ElevenLabs | `elevenlabs.io/pricing/api`、`/docs/overview/administration/data-residency`、`/docs/overview/capabilities/speech-to-text` | ✅（第一次 SSL 失败，重试成功；句内切换**未核实**） |
| Rev.ai | `rev.ai/pricing` | ✅ 价格和 15 秒最低计费；区域、粤语支持**未核实** |
| 阿里云 智能语音交互 | `help.aliyun.com/zh/isi/product-overview/billing-10` | ✅ |
| 阿里云百炼（中国站 / 国际站） | `help.aliyun.com/zh/model-studio/model-pricing`、`alibabacloud.com/help/en/model-studio/model-pricing` | ✅ 价格。语言能力页（`qwen-speech-recognition`）没能提取到语种列表，粤语和混说支持**未核实** |
| 腾讯云 ASR | `cloud.tencent.com/document/product/1093/35686`（计费）、`/1093/48982`（实时识别引擎） | ✅（第一次抓到压缩内容，加 `--compressed` 后成功）。国际站价格**未抓取** |
| 科大讯飞 | `xfyun.cn/services/rtasr`、`/services/lfasr`、`/services/speech_big_model`、`/services/voicedictation` | ✅ 套餐价。按量计费不公开，大用量要"商务咨询"；国际站**未核实** |
| 百度智能云 | `cloud.baidu.com/doc/SPEECH/s/Tldjm0i4c`（文档更新时间 2025-12-11） | ✅ 价格。3.3 元/小时那一档对应哪个接口未能确认；混说**未核实** |
| 火山引擎 | `volcengine.com/docs/6561/1359370`（计费）、`/docs/6561/1354869`（大模型识别 API） | ✅ 价格和语种参数；免费额度、BytePlus **未核实** |
| 汇率 | `api.frankfurter.dev/v1/latest?base=USD&symbols=AUD,CNY`（ECB，2026-11-04） | ✅ |

**打不开、需要重试，或价格不公开的：**`openai.com/api/pricing`（被拦截，改用 platform 文档）；`deepgram.com`、`elevenlabs.io`、`cloud.google.com`、`docs.cloud.google.com`、`api.frankfurter.app` 偶尔 SSL 握手失败，重试或换域名后成功；讯飞大用量价格只能商务咨询；Azure 官网价格页的价格由前端渲染（改用 Retail Prices API）。全程直连，没有用 SOCKS 代理。

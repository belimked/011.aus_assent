# STT + 抽取供应商试用

目的：比较房产中介门口 DK、开放日 OFI、询盘回电的澳洲英语/普通话/粤语混说识别率，并测试 Claude Haiku 4.5 抽取字段；只用虚构样本。选型基线为 Azure AI Speech Fast Australia East + Bedrock `au.` Claude Haiku 4.5。

## 拿到 key 后

1. Node 22；复制 `tools/stt-trial/.env.example` 为 `.env` 并填 key。实时 Azure 可在 `tools/stt-trial/` 执行 `npm install`（仅此 provider 依赖 SDK；不装会清楚报错并跳过）。设各家消费上限。
2. 按 `tools/stt-trial/RECORDING.md` 录 28 条虚构音频到 `audio/<ID>.m4a`；即兴偏离脚本时改 `refs/<ID>.txt`。禁止真实客户录音/图片。
3. `node tools/stt-trial/prep.mjs` （需 `/usr/local/bin/ffmpeg` 和 `ffprobe`）。
4. `node tools/stt-trial/run.mjs all`；可选 `--providers azure-fast,deepgram`、`--only B01,B02`、`--mock`；TTS 用 `run.mjs tts`，图片抽取用 `run.mjs extract --images`（先跑 all 或 batch）。`all` 包括 batch、stream、tts、extract；TTS 也可单独跑。无 key 自动跳过。
5. `node tools/stt-trial/score.mjs tools/stt-trial/.out/<时间戳>`。报告、原始响应、转写和估算成本在该目录。单独 extract 读取上次转写；同一次 all 则读本次数据。

账号：Azure 创建 **Speech** 资源并选择 Australia East；Deepgram 选 **AU** 端点且所有请求必须 `mip_opt_out=true`；Speechmatics 用 **AU1**；Bedrock 在 **ap-southeast-2** 开通 Haiku 4.5 模型访问，**只用 `au.` inference profile，拒绝 `global.`**。各家设置预算/告警，核查 DPA、日志和数据留存。账号开通后逐条核对 `config.mjs` 中 TODO、实际模型 ID、计费及自动语言识别能力。

评判：先 B/C 类 MER 和姓名/电话/地址/价格/时间关键词，再 batch 总延迟、stream 首字和说完到终稿延迟，最后每 30 秒 AUD 与抽取字段准确率；抽样人工复核粤语异体字和含义，不以 mock 报告选型。MER 的内置繁简/数字规则只覆盖常见字和数字。

## 生产接口草案（仅文档，不实现）

- `POST /api/v1/stt/transcribe`：multipart 音频 + `langs`，服务端转发给可配置供应商；不留存音频，鉴权并限制大小/时长。
- `POST /api/v1/stt/token`：为手机端实时识别/翻译签发短期凭据（Azure issueToken、Deepgram 临时 key、Speechmatics 临时 key），手机直连供应商，不经我们转发音频；服务端限频、限制权限与有效期。
- `POST /api/v1/ai/extract`：文字或图片 → 固定 JSON 字段（contactName、phone、email、address、suburb、intent、priceMin、priceMax、timeframe、notes、language、confidence），人工确认后写 Rex。

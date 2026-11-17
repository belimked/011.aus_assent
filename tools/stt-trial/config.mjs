// 集中供应商可变参数。拿到账号后核对：URL、模型、locale、费率、区域及数据保留/DPA。
export const config = {
  azure: { region: 'australiaeast', fastPath: '/speechtotext/transcriptions:transcribe?api-version=2024-11-15', ttsPath: '/cognitiveservices/v1', voices: { 'en-AU': 'en-AU-NatashaNeural', 'zh-CN': 'zh-CN-XiaoxiaoNeural', 'zh-HK': 'zh-HK-HiuGaaiNeural' }, variants: { en: ['en-AU'], cmn: ['zh-CN'], yue: ['zh-HK'], multi: ['en-AU','zh-CN','zh-HK'], auto: null } }, // TODO: 2024-11-15 API 版本（官网示例已更新为 2025-10-15）、无 locales 多语种及多 locales 同请求需账号实测
  deepgram: { batch: 'https://api.au.deepgram.com/v1/listen', stream: 'wss://api.au.deepgram.com/v1/listen', params: { model: 'nova-3', language: 'multi', smart_format: 'true', mip_opt_out: 'true' } }, // TODO: AU 流式 token 子协议和 language=multi 实际权限拿到账号后核对
  speechmatics: { batch: 'https://au1.asr.api.speechmatics.com/v2/jobs', stream: 'wss://au.rt.speechmatics.com/v2', tempKey: 'https://mp.speechmatics.com/v1/jwts?type=rt', variants: { cmn_en: { language: 'cmn_en', operating_point: 'enhanced' }, yue: { language: 'yue', operating_point: 'enhanced' }, en: { language: 'en', operating_point: 'enhanced' }, auto: { language: 'auto', operating_point: 'melia-1' } } }, // TODO: Melia auto/yue 支持及 AU1 能力需账号核对（模型名 melia-1 官方 models.md 已确认）
  bedrock: { region: 'ap-southeast-2', modelId: 'au.anthropic.claude-haiku-4-5-20251001-v1:0', sonnetId: 'au.anthropic.claude-sonnet-4-5-20250929-v1:0' }, // TODO: au. inference profile ID/可用性及 Sonnet 具体 ID 在账号内核对
  pricing: { usdAud: 1.55, auPremium: 0.125, azureFastHour: 0.36, azureRtHour: 1, deepgramBatchMinute: 0.0052, deepgramStreamMinute: 0.0092, speechmaticsMeliaBatchHour: 0.12, speechmaticsMeliaStreamHour: 0.20, speechmaticsEnhancedBatchHour: 0.38, speechmaticsEnhancedStreamHour: 0.40, haikuInputMillion: 1, haikuOutputMillion: 5 }, // TODO: 所有价格、汇率及溢价均为预算估算，拿到账号后核对账单；TTS 价格未核对
};
export function audioCost(provider, mode, seconds, variant = '') {
  const p = config.pricing;
  const rate = provider === 'azure-fast' ? p.azureFastHour / 3600 : provider === 'azure-rt' ? p.azureRtHour / 3600 : provider === 'deepgram' ? (mode === 'stream' ? p.deepgramStreamMinute : p.deepgramBatchMinute) / 60 : provider === 'speechmatics' ? (variant === 'auto' ? (mode === 'stream' ? p.speechmaticsMeliaStreamHour : p.speechmaticsMeliaBatchHour) : (mode === 'stream' ? p.speechmaticsEnhancedStreamHour : p.speechmaticsEnhancedBatchHour)) / 3600 : 0;
  return seconds * rate * p.usdAud * (1 + p.auPremium);
}
export function tokenCost(input, output) { const p = config.pricing; return (input * p.haikuInputMillion + output * p.haikuOutputMillion) / 1e6 * p.usdAud * (1 + p.auPremium); }

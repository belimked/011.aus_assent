import { config } from '../config.mjs';
import { pace } from '../util.mjs';
export async function stream(buffer, { onPartial = () => {} } = {}) {
  let sdk; try { sdk = await import('microsoft-cognitiveservices-speech-sdk'); } catch { throw new Error('azure-rt 未安装 Speech SDK：cd tools/stt-trial && npm install（此 provider 跳过）'); }
  const region = process.env.AZURE_SPEECH_REGION || config.azure.region; const speech = sdk.SpeechConfig.fromSubscription(process.env.AZURE_SPEECH_KEY, region);
  speech.setProperty(sdk.PropertyId.SpeechServiceConnection_LanguageIdMode, 'Continuous');
  const lid = sdk.AutoDetectSourceLanguageConfig.fromLanguages(['en-AU','zh-CN','zh-HK']);
  const push = sdk.AudioInputStream.createPushStream(sdk.AudioStreamFormat.getWaveFormatPCM(16000, 16, 1));
  const recognizer = sdk.SpeechRecognizer.FromConfig(speech, lid, sdk.AudioConfig.fromStreamInput(push));
  const start = performance.now(); let firstPartialMs = null, ended, text = '', raw = [];
  try {
    recognizer.recognizing = (_, e) => { if (e.result.text) { firstPartialMs ??= performance.now() - start; onPartial(e.result.text); raw.push({ type: 'partial', text: e.result.text }); } };
    recognizer.recognized = (_, e) => { if (e.result.text) { text += (text ? ' ' : '') + e.result.text; raw.push({ type: 'final', text: e.result.text }); } };
    await new Promise((resolve, reject) => recognizer.startContinuousRecognitionAsync(resolve, reject));
    await pace(buffer, c => push.write(c)); ended = performance.now(); push.close();
    await new Promise((resolve, reject) => { const timer = setTimeout(() => reject(new Error('Azure RT 超时')), 20000); recognizer.sessionStopped = () => { clearTimeout(timer); resolve(); }; recognizer.canceled = (_, e) => { clearTimeout(timer); reject(new Error(`Azure RT canceled: ${e.errorDetails}`)); }; });
    return { text, firstPartialMs, finalAfterEndMs: performance.now() - ended, raw };
  } finally { await new Promise(resolve => recognizer.stopContinuousRecognitionAsync(resolve, resolve)); recognizer.close(); }
}

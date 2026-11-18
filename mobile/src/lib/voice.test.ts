import {authHeader} from '../store/sync';
import {speak,translate,voiceLang,VoiceError} from './voice';
jest.mock('../store/sync',()=>({apiUrl:'https://example.test/api/v1',authHeader:jest.fn(()=>({Authorization:'Bearer test'}))}));
jest.mock('expo-file-system',()=>({File:jest.fn(),Paths:{cache:{}}}));
jest.mock('expo-audio',()=>({createAudioPlayer:jest.fn(),setAudioModeAsync:jest.fn(async()=>undefined)}));
const fetchMock=jest.fn();
beforeEach(()=>{globalThis.fetch=fetchMock;fetchMock.mockReset();(authHeader as jest.Mock).mockReturnValue({Authorization:'Bearer test'});});
test('sheet language codes map to API languages',()=>{
  expect(['en-AU','zh-CN','yue-HK'].map(code=>voiceLang(code as Parameters<typeof voiceLang>[0]))).toEqual(['en','zh','yue']);
});
test('translate sends authenticated JSON and returns translation metadata',async()=>{
  const result={text:'你好',from:'en',to:'zh',model:'test',latencyMs:42};
  fetchMock.mockResolvedValue({ok:true,json:async()=>result});
  await expect(translate('Hello','en','zh')).resolves.toEqual(result);
  expect(fetchMock).toHaveBeenCalledWith('https://example.test/api/v1/ai/translate',expect.objectContaining({method:'POST',body:JSON.stringify({text:'Hello',from:'en',to:'zh'}),headers:{Authorization:'Bearer test','Content-Type':'application/json'}}));
});
test.each([[401,'offline'],[429,'rateLimit'],[502,'upstream'],[503,'unavailable'] ] as const)('translate maps HTTP %i to %s',async(status,kind)=>{
  fetchMock.mockResolvedValue({ok:false,status});
  await expect(translate('Hi','en','yue')).rejects.toMatchObject({kind});
});
test.each([[401,'offline'],[429,'rateLimit'],[502,'upstream'],[503,'unavailable']] as const)('speak maps HTTP %i to %s without touching audio',async(status,kind)=>{
  fetchMock.mockResolvedValue({ok:false,status});
  await expect(speak('Hi','en')).rejects.toMatchObject({kind});
  expect(fetchMock).toHaveBeenCalledWith('https://example.test/api/v1/tts',expect.objectContaining({body:JSON.stringify({text:'Hi',lang:'en'})}));
});
test('speak requires sign-in',async()=>{
  (authHeader as jest.Mock).mockReturnValue(null);
  await expect(speak('Hi','en')).rejects.toBeInstanceOf(VoiceError);
  expect(fetchMock).not.toHaveBeenCalled();
});

import {File} from 'expo-file-system';
import {getState,update} from '../store';
import {authHeader} from '../store/sync';
import {aiImage,uploadPhoto,uploadPendingPhotos} from './photos';

jest.mock('../store/sync',()=>({apiUrl:'https://example.test/api/v1',authHeader:jest.fn(()=>({Authorization:'Bearer test'}))}));
jest.mock('../store',()=>({getState:jest.fn(),update:jest.fn()}));
jest.mock('expo-file-system',()=>({File:jest.fn(),Paths:{document:{}}}));
jest.mock('expo-audio',()=>({createAudioPlayer:jest.fn(),setAudioModeAsync:jest.fn()}));
jest.mock('expo-image-manipulator',()=>({manipulateAsync:jest.fn(),SaveFormat:{JPEG:'jpeg'}}));
const fetchMock=jest.fn();
const state={properties:[{id:'p1',photos:[{id:'photo_123',d:'2026-11-09',uri:'file:///house.jpg',uploaded:false},{id:'remote_1',d:'2026-11-09',uri:'https://other/image.jpg',uploaded:false}]}]};
beforeEach(()=>{fetchMock.mockReset();globalThis.fetch=fetchMock;(authHeader as jest.Mock).mockReturnValue({Authorization:'Bearer test'});(File as unknown as jest.Mock).mockImplementation(()=>({bytes:async()=>new Uint8Array([1,2,3]),exists:true}));(getState as jest.Mock).mockReturnValue(state);(update as jest.Mock).mockImplementation(fn=>fn(state));state.properties[0].photos[0].uploaded=false;});
const bodyText=(body:Uint8Array)=>Array.from(body,c=>String.fromCharCode(c)).join('');
test('uploadPhoto sends authenticated multipart file, propertyId and id',async()=>{
  fetchMock.mockResolvedValue({ok:true,json:async()=>({id:'photo_123'})});
  await expect(uploadPhoto('file:///house.jpg','p1','photo_123')).resolves.toEqual({id:'photo_123'});
  const [url,init]=fetchMock.mock.calls[0];expect(url).toBe('https://example.test/api/v1/photos');expect(init.method).toBe('POST');expect(init.headers.Authorization).toBe('Bearer test');expect(init.headers['Content-Type']).toMatch(/^multipart\/form-data; boundary=/);
  expect(bodyText(init.body)).toContain('name="propertyId"\r\n\r\np1');expect(bodyText(init.body)).toContain('name="id"\r\n\r\nphoto_123');expect(bodyText(init.body)).toContain('name="file"; filename="photo.jpg"\r\nContent-Type: image/jpeg\r\n\r\n\x01\x02\x03');
});
test('aiImage sends ctx and optional lang and maps service errors',async()=>{
  fetchMock.mockResolvedValueOnce({ok:true,json:async()=>({fields:{},confidence:{},model:'test',latencyMs:1})}).mockResolvedValueOnce({ok:false,status:503});
  await aiImage('file:///card.jpg','card','zh');expect(fetchMock.mock.calls[0][0]).toBe('https://example.test/api/v1/ai/image');
  const text=bodyText(fetchMock.mock.calls[0][1].body);expect(text).toContain('name="ctx"\r\n\r\ncard');expect(text).toContain('name="lang"\r\n\r\nzh');
  await expect(aiImage('file:///card.jpg','note')).rejects.toMatchObject({kind:'unavailable'});
});
test('pending uploads mark successful file photos and skip remote uris',async()=>{
  fetchMock.mockResolvedValue({ok:true,json:async()=>({id:'photo_123'})});await uploadPendingPhotos();
  expect(fetchMock).toHaveBeenCalledTimes(1);expect(state.properties[0].photos[0].uploaded).toBe(true);expect(state.properties[0].photos[1].uploaded).toBe(false);
});
test('failed pending upload stays pending',async()=>{
  fetchMock.mockResolvedValue({ok:false,status:502});await uploadPendingPhotos();expect(state.properties[0].photos[0].uploaded).toBe(false);
});

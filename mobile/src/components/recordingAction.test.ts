import {recordingAction} from './recordingAction';
test('tap starts recording, second tap stops, transcription blocks another start',()=>{
 expect(recordingAction('idle')).toBe('start');
 expect(recordingAction('recording')).toBe('stop');
 expect(recordingAction('transcribing')).toBe('wait');
 expect(recordingAction('idle')).toBe('start');
});

import {Platform} from 'react-native';
import {RecordingPresets,IOSOutputFormat} from 'expo-audio';
export const recordingOptions={...RecordingPresets.HIGH_QUALITY,isMeteringEnabled:true,...(Platform.OS==='ios'?{extension:'.wav',sampleRate:16000,numberOfChannels:1,ios:{...RecordingPresets.HIGH_QUALITY.ios,outputFormat:IOSOutputFormat.LINEARPCM,sampleRate:16000,linearPCMBitDepth:16,linearPCMIsBigEndian:false,linearPCMIsFloat:false}}:{})};

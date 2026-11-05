import { View, Text } from 'react-native'
import { useApp } from '@/lib/app-state'
export default function Mic() { const { t } = useApp(); return <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}><Text>🎙 {t('holdTalk')} — {t('soon')}</Text></View> }

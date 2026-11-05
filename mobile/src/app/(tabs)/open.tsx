import { View, Text } from 'react-native'
import { useApp } from '@/lib/app-state'
export default function OpenHome() { const { t } = useApp(); return <View style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}><Text>{t('open')} — {t('soon')}</Text></View> }

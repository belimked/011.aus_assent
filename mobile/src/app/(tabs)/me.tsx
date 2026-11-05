import { View, Text, Pressable } from 'react-native'
import { useApp } from '@/lib/app-state'
import { colors } from '@/lib/theme'
export default function Me() { const { t, language, setLanguage } = useApp(); return <View style={{ flex: 1, padding: 24, paddingTop: 70, gap: 20, backgroundColor: colors.bg }}><Text style={{ fontSize: 28, fontWeight: '700' }}>{t('me')}</Text><Text>{t('language')}</Text><Pressable onPress={() => setLanguage(language === 'en' ? 'zh' : 'en')} style={{ backgroundColor: '#fff', padding: 18, borderRadius: 12 }}><Text>English / 简体中文: {language === 'en' ? 'English' : '简体中文'}</Text></Pressable><Text>Rex CRM — {t('soon')}</Text></View> }

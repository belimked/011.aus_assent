import { useState } from 'react'
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native'
import { PropertyCard } from '@/components/PropertyCard'
import { ATTRIBUTION, properties, type Property } from '@/data/properties'
import { useApp } from '@/lib/app-state'
import { nearest } from '@/lib/geometry'
import { colors } from '@/lib/theme'
export default function Home() {
  const { t, position, demo } = useApp()
  const [selected, setSelected] = useState<Property>()
  const [message, setMessage] = useState('')
  const property = nearest(properties, position)
  const tiles = ['property', 'owner', 'tag', 'note', 'photo', 'logCall'] as const
  return <ScrollView style={styles.root} contentContainerStyle={styles.content}>
    <Text style={styles.brand}>Place Sunnybank <Text style={{ color: colors.primary }}>· agentCollect</Text></Text>
    <Text style={styles.heading}>{t('home')}</Text>
    <Pressable style={styles.card} onPress={() => setSelected(property)}><Text style={styles.label}>{t('youAt')} {demo ? '(demo)' : ''}</Text>
      <Text style={styles.address}>{property?.number} {property?.street}</Text><Text>{property?.suburb}, QLD {property?.suburb === 'Runcorn' ? '4113' : '4109'}</Text>
      <Text style={styles.label}>{property?.owner} (mock) · {t('held')} {property?.yearsHeld} {t('years')}</Text></Pressable>
    <Pressable style={styles.talk} onLongPress={() => setMessage(t('soon'))} onPress={() => setMessage(t('soon'))}><Text style={styles.talkText}>🎙  {t('holdTalk')}</Text></Pressable>
    <View style={styles.grid}>{tiles.map((tile, i) => <Pressable key={tile} style={styles.tile} onPress={() => tile === 'property' ? setSelected(property) : setMessage(t('soon'))}><Text style={styles.emoji}>{['⌂', '♙', '⌗', '✎', '▣', '☎'][i]}</Text><Text>{t(tile)}</Text></Pressable>)}</View>
    {!!message && <Text style={styles.label}>{message}</Text>}
    <Text style={styles.credit}>{ATTRIBUTION}</Text>
    <PropertyCard property={selected} onClose={() => setSelected(undefined)} />
  </ScrollView>
}
const styles = StyleSheet.create({ root: { flex: 1, backgroundColor: colors.bg }, content: { padding: 22, paddingTop: 58, gap: 20 }, brand: { fontSize: 17, fontWeight: '700' }, heading: { fontSize: 30, fontWeight: '800', color: colors.text }, card: { backgroundColor: '#fff', borderRadius: 18, padding: 22, gap: 7 }, label: { color: colors.muted }, address: { fontSize: 23, fontWeight: '700' }, talk: { backgroundColor: colors.red, padding: 24, borderRadius: 20, alignItems: 'center' }, talkText: { color: '#fff', fontSize: 22, fontWeight: '700' }, grid: { flexDirection: 'row', flexWrap: 'wrap', gap: 12 }, tile: { width: '30%', flexGrow: 1, backgroundColor: '#fff', borderRadius: 16, alignItems: 'center', padding: 20, gap: 10 }, emoji: { fontSize: 26, color: colors.primary }, credit: { fontSize: 11, color: colors.muted, marginTop: 20 } })

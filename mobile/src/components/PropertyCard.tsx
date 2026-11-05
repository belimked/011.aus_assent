import { Modal, Pressable, StyleSheet, Text, View } from 'react-native'
import type { Property } from '@/data/properties'
import { tenureColor, maskMobile } from '@/lib/geometry'
import { useApp } from '@/lib/app-state'
import { colors } from '@/lib/theme'

export function PropertyCard({ property, onClose }: { property?: Property; onClose: () => void }) {
  const { t } = useApp()
  return <Modal transparent visible={!!property} animationType="slide" onRequestClose={onClose}>
    <Pressable style={styles.backdrop} onPress={onClose} />
    {property && <View style={styles.sheet}>
      <Pressable onPress={onClose} accessibilityLabel="Close" style={styles.close}><Text>✕</Text></Pressable>
      <Text style={styles.title}>{property.unit ? `${property.unit}/` : ''}{property.number} {property.street}{property.units > 1 ? ` · ${property.units} units` : ''}</Text>
      <Text style={styles.sub}>{property.suburb}, QLD {property.suburb === 'Runcorn' ? '4113' : '4109'}</Text>
      <Text style={styles.sub}>Lot/Plan {property.lotplan} · {property.area ?? '—'} m²</Text>
      <View style={styles.row}><Text style={styles.owner}>{property.owner} (mock)</Text><View style={[styles.badge, { backgroundColor: tenureColor(property.yearsHeld) }]}><Text style={{ color: property.yearsHeld >= 8 ? '#fff' : '#111' }}>{t('held')} {property.yearsHeld} {t('years')}</Text></View></View>
      <Text style={styles.sub}>{maskMobile()}</Text>
      <View style={styles.row}><Pressable style={styles.action}><Text style={styles.actionText}>{t('contact')} · {t('soon')}</Text></Pressable><Pressable style={styles.action}><Text style={styles.actionText}>{t('save')} · {t('soon')}</Text></Pressable></View>
    </View>}
  </Modal>
}
const styles = StyleSheet.create({ backdrop: { flex: 1, backgroundColor: '#0007' }, sheet: { padding: 22, paddingBottom: 42, backgroundColor: '#fff', borderTopLeftRadius: 24, borderTopRightRadius: 24 }, close: { alignSelf: 'flex-end', padding: 8 }, title: { fontSize: 22, fontWeight: '700', color: colors.text }, sub: { color: colors.muted, marginTop: 8 }, row: { flexDirection: 'row', alignItems: 'center', gap: 10, marginTop: 16, flexWrap: 'wrap' }, owner: { fontSize: 17, fontWeight: '600' }, badge: { borderRadius: 12, padding: 8 }, action: { backgroundColor: colors.primary, borderRadius: 12, padding: 12 }, actionText: { color: '#fff' } })

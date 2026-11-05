import { AppleMaps, GoogleMaps, type CameraMoveEvent } from 'expo-maps'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native'
import { PropertyCard } from '@/components/PropertyCard'
import { ATTRIBUTION, DEMO_POSITION, properties, visibleParcels, visibleProperties, type Property } from '@/data/properties'
import { useApp } from '@/lib/app-state'
import { colors } from '@/lib/theme'
import type { Bounds } from '@/lib/geometry'

const initial = { coordinates: { latitude: DEMO_POSITION.lat, longitude: DEMO_POSITION.lng }, zoom: 17 }
const defaultBounds: Bounds = { south: DEMO_POSITION.lat - 0.008, north: DEMO_POSITION.lat + 0.008, west: DEMO_POSITION.lng - 0.008, east: DEMO_POSITION.lng + 0.008 }
const statusColor = { green: colors.green, red: colors.red, blue: colors.blue }
export default function MapScreen() {
  const { t, position, demo, locate } = useApp()
  const apple = useRef<AppleMaps.MapView>(null), google = useRef<GoogleMaps.MapView>(null)
  const moveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [view, setView] = useState({ bounds: defaultBounds, zoom: 17 })
  const [satellite, setSatellite] = useState(false)
  const [selected, setSelected] = useState<Property>()
  const active = view.zoom >= 15
  const markers = useMemo(() => active ? visibleProperties(view.bounds) : [], [active, view])
  const parcels = useMemo(() => view.zoom >= 17 ? visibleParcels(view.bounds) : [], [view])
  useEffect(() => () => { if (moveTimer.current) clearTimeout(moveTimer.current) }, [])
  function moved(e: CameraMoveEvent) {
    if (moveTimer.current) clearTimeout(moveTimer.current)
    moveTimer.current = setTimeout(() => {
      const lat = e.coordinates.latitude ?? DEMO_POSITION.lat, lng = e.coordinates.longitude ?? DEMO_POSITION.lng
      setView({ zoom: e.zoom, bounds: { south: lat - e.latitudeDelta / 2, north: lat + e.latitudeDelta / 2, west: lng - e.longitudeDelta / 2, east: lng + e.longitudeDelta / 2 } })
    }, 180)
  }
  const coords = (p: { lat: number; lng: number }) => ({ latitude: p.lat, longitude: p.lng })
  const polygonProps = parcels.map(p => ({ id: p.id, coordinates: p.ring.map(coords), color: '#0b5d8a22', lineColor: '#0b5d8a99', lineWidth: 1 }))
  const select = (id?: string) => setSelected(properties.find(p => p.id === id))
  useEffect(() => {
    const target = { coordinates: coords(position), zoom: 17 }
    void apple.current?.setCameraPosition(target)
    void google.current?.setCameraPosition(target)
  }, [position])
  return <View style={styles.root}>
    {Platform.OS === 'ios' ? <AppleMaps.View ref={apple} style={StyleSheet.absoluteFill} cameraPosition={initial} onCameraMove={moved}
      markers={[...markers.map(p => ({ id: p.id, coordinates: coords(p), title: `${p.number} ${p.street}${p.units > 1 ? ` · ${p.units} units` : ''}`, tintColor: statusColor[p.status] })), ...(!demo ? [{ id: 'user-location', coordinates: coords(position), title: 'You', tintColor: '#111827' }] : [])]}
      polygons={polygonProps} onMarkerClick={e => select(e.id)} onPolygonClick={e => select(properties.find(p => p.lotplan === e.id)?.id)}
      properties={{ mapType: satellite ? AppleMaps.MapType.HYBRID : AppleMaps.MapType.STANDARD }} /> :
      Platform.OS === 'android' ? <GoogleMaps.View ref={google} style={StyleSheet.absoluteFill} cameraPosition={initial} onCameraMove={moved}
        markers={[...markers.map(p => ({ id: p.id, coordinates: coords(p), title: `${p.number} ${p.street}`, snippet: p.units > 1 ? `${p.units} units` : undefined })), ...(!demo ? [{ id: 'user-location', coordinates: coords(position), title: 'You' }] : [])]}
        circles={markers.map(p => ({ id: p.id, center: coords(p), radius: 9, color: statusColor[p.status] + '99', lineColor: statusColor[p.status] }))}
        polygons={polygonProps} onMarkerClick={e => select(e.id)} onCircleClick={e => select(e.id)} onPolygonClick={e => select(properties.find(p => p.lotplan === e.id)?.id)}
        properties={{ mapType: satellite ? GoogleMaps.MapType.HYBRID : GoogleMaps.MapType.NORMAL }} /> : <Text>Native map requires iOS or Android</Text>}
    <View style={styles.tools}><Pressable style={styles.button} onPress={() => setSatellite(s => !s)}><Text>{satellite ? 'Map' : 'Satellite'}</Text></Pressable>
      <Pressable style={styles.button} onPress={() => { void locate(); const target = { coordinates: coords(position), zoom: 17 }; void apple.current?.setCameraPosition(target); void google.current?.setCameraPosition(target) }}><Text>{t('near')}</Text></Pressable></View>
    {!active && <Text style={styles.hint}>{t('zoom')}</Text>}
    <View style={styles.footer}><Text style={styles.small}>{demo ? `${t('demo')} · ` : ''}{ATTRIBUTION}</Text></View>
    <PropertyCard property={selected} onClose={() => setSelected(undefined)} />
  </View>
}
const styles = StyleSheet.create({ root: { flex: 1, backgroundColor: colors.bg }, tools: { position: 'absolute', top: 55, right: 16, gap: 10 }, button: { backgroundColor: '#fff', borderRadius: 12, padding: 13, elevation: 3 }, hint: { position: 'absolute', top: '45%', alignSelf: 'center', backgroundColor: '#fff', padding: 14, borderRadius: 14 }, footer: { position: 'absolute', bottom: 8, alignSelf: 'center', backgroundColor: '#fffd', borderRadius: 6, padding: 4 }, small: { fontSize: 10, color: colors.muted } })

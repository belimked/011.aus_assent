import { Tabs } from 'expo-router'
import { Pressable, Text, View } from 'react-native'
import { useApp } from '@/lib/app-state'
import { colors } from '@/lib/theme'

export default function TabLayout() {
  const { t } = useApp()
  const tabs = [ ['index', 'home', '⌂'], ['map', 'map', '⌖'], ['mic', 'mic', '●'], ['open', 'open', '◇'], ['me', 'me', '♙'] ] as const
  return <Tabs screenOptions={{ headerShown: false, tabBarActiveTintColor: colors.primary, tabBarStyle: { height: 76, paddingBottom: 12, backgroundColor: '#fff' } }}>
    {tabs.map(([name, label, icon]) => <Tabs.Screen key={name} name={name} options={{ title: t(label), tabBarButton: name === 'mic' ? props => <Pressable onPress={props.onPress} accessibilityLabel={props.accessibilityLabel} style={{ flex: 1, justifyContent: 'center', alignItems: 'center' }}><View style={{ width: 56, height: 56, marginTop: -24, borderRadius: 28, backgroundColor: colors.red, justifyContent: 'center', alignItems: 'center' }}><Text style={{ color: '#fff', fontSize: 31 }}>●</Text></View></Pressable> : undefined, tabBarIcon: ({ color }) => <Text style={{ color, fontSize: 25 }}>{icon}</Text> }} />)}
  </Tabs>
}

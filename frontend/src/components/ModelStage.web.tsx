import { Suspense } from 'react'
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native'
import { Canvas } from '@react-three/fiber'
import { Bounds, OrbitControls, useGLTF } from '@react-three/drei'
import type { Group } from 'three'
import { colors, radius } from '../helpers/theme'

const modelAsset = require('../../assets/models/first_class_wagon.glb')

function WagonModel() {
  const { scene } = useGLTF(modelAsset) as unknown as { scene: Group }
  return <primitive object={scene} />
}

function LoadingScene() {
  return (
    <View style={styles.loading}>
      <ActivityIndicator color={colors.primary} />
      <Text style={styles.loadingText}>Загружаем вагон…</Text>
    </View>
  )
}

export function ModelStage() {
  return (
    <View style={styles.card}>
      <View style={styles.copy} pointerEvents="none">
        <Text style={styles.kicker}>ИНТЕРАКТИВНАЯ СЦЕНА</Text>
        <Text style={styles.title}>Вагон первого класса</Text>
        <Text style={styles.hint}>Перетащите, чтобы осмотреть</Text>
      </View>
      <Suspense fallback={<LoadingScene />}>
        <Canvas camera={{ position: [5, 4, 6], fov: 42 }} style={styles.canvas}>
          <color attach="background" args={['#E9EEF0']} />
          <ambientLight intensity={1.8} />
          <directionalLight position={[5, 8, 6]} intensity={2.6} />
          <directionalLight position={[-4, 3, -4]} intensity={1.2} />
          <Bounds fit clip observe margin={1.25}>
            <WagonModel />
          </Bounds>
          <OrbitControls enablePan={false} minDistance={2} maxDistance={18} />
        </Canvas>
      </Suspense>
      <View style={styles.badge} pointerEvents="none"><Text style={styles.badgeText}>360°</Text></View>
    </View>
  )
}

const styles = StyleSheet.create({
  card: { height: 245, borderRadius: radius.lg, overflow: 'hidden', backgroundColor: '#E9EEF0', marginBottom: 16 },
  canvas: { flex: 1 },
  copy: { position: 'absolute', zIndex: 2, left: 18, top: 17 },
  kicker: { color: colors.primary, fontSize: 9, fontWeight: '900', letterSpacing: 1.1 },
  title: { color: colors.ink, fontSize: 19, fontWeight: '900', marginTop: 4 },
  hint: { color: colors.muted, fontSize: 10, marginTop: 3 },
  badge: { position: 'absolute', right: 14, bottom: 14, backgroundColor: 'rgba(23,33,38,0.82)', borderRadius: radius.pill, paddingHorizontal: 11, paddingVertical: 7 },
  badgeText: { color: colors.surface, fontSize: 10, fontWeight: '900' },
  loading: { position: 'absolute', inset: 0, alignItems: 'center', justifyContent: 'center' },
  loadingText: { color: colors.muted, fontSize: 11, marginTop: 8 },
})

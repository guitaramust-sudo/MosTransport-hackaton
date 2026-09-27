import { useGLTF } from '@react-three/drei'
import type { Group } from 'three'
import { blanketAsset } from '../helpers/gameAssets'

useGLTF.preload(blanketAsset)

export function WagonBlanket({ onPress }: { onPress?: () => void }) {
  const { scene } = useGLTF(blanketAsset) as unknown as { scene: Group }
  return <group position={[-1.3, 1.7, -7.5]} rotation={[-Math.PI / 2, 0, 0]}
    onPointerDown={onPress ? (event) => { event.stopPropagation(); onPress() } : undefined}>
    <primitive object={scene} />
  </group>
}

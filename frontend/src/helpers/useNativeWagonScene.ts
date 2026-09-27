import { useGLTF } from '@react-three/drei/native'
import { Mesh, MeshPhysicalMaterial, type Group } from 'three'
import { wagonAsset } from './gameAssets'

export function useNativeWagonScene(): Group {
  const { scene } = useGLTF(wagonAsset) as unknown as { scene: Group }

  // Three.js uses a multisampled render target for transmission even when
  // Canvas antialiasing is off. Expo GL does not implement that target.
  scene.traverse((object) => {
    if (!(object instanceof Mesh)) return
    const materials = Array.isArray(object.material) ? object.material : [object.material]
    for (const material of materials) {
      if (!(material instanceof MeshPhysicalMaterial) || material.transmission <= 0) continue
      material.opacity = Math.min(material.opacity, 1 - material.transmission)
      material.transmission = 0
      material.transparent = true
      material.depthWrite = false
      material.needsUpdate = true
    }
  })

  return scene
}

// Web entry of the three.js platform layer. Metro picks three.native.ts on
// iOS/Android, so shared 3D scenes import from here and run on both.
import { useGLTF } from '@react-three/drei'
import type { Group } from 'three'
import { wagonAsset } from './gameAssets'

export { Canvas, useFrame, useThree } from '@react-three/fiber'
export { OrthographicCamera, useAnimations, useGLTF } from '@react-three/drei'

export const canvasGl = { antialias: true }

export function useWagonScene(): Group {
  return (useGLTF(wagonAsset) as unknown as { scene: Group }).scene
}

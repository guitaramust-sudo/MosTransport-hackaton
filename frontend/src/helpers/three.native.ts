// Native entry of the three.js platform layer (Expo GL). See three.ts.
export { Canvas, useFrame, useThree } from '@react-three/fiber/native'
export { OrthographicCamera, useAnimations, useGLTF } from '@react-three/drei/native'
export { useNativeWagonScene as useWagonScene } from './useNativeWagonScene'

// Expo GL has no multisampled render target; keep antialiasing off.
export const canvasGl = { antialias: false }

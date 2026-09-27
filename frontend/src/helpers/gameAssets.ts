import { Asset } from 'expo-asset'

export const wagonAsset = require('../../assets/models/TOOOPblend.glb')
export const blanketAsset = require('../../assets/models/blanket.glb')
export const conductorAsset = require('../../assets/characters/conductor.glb')
export const forestAsset = require('../../assets/images/background.png')

// Each character contains IdleSeated (the updated seat pose), SitDown,
// StandUp and Walk. Keep them lazy: the scene only loads models in use.
export const passengerAssets = [
  require('../../assets/characters/passenger_new_01.glb'),
  require('../../assets/characters/passenger_new_02.glb'),
  require('../../assets/characters/passenger_new_03.glb'),
  require('../../assets/characters/passenger_new_04.glb'),
  require('../../assets/characters/passenger_new_05.glb'),
  require('../../assets/characters/passenger_new_06.glb'),
  require('../../assets/characters/passenger_male.glb'),
  require('../../assets/characters/passenger_female.glb'),
]

export function passengerAssetFor(id: string) {
  // Stable across renders and sessions without depending on seat assignment.
  let hash = 0
  for (const char of id) hash = (hash * 31 + char.charCodeAt(0)) >>> 0
  return passengerAssets[hash % passengerAssets.length]
}

const gameAssets = [wagonAsset, blanketAsset, conductorAsset, forestAsset]

export async function preloadGameAssets() {
  await Asset.loadAsync(gameAssets)
}

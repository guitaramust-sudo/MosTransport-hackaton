import { Asset } from 'expo-asset'

export const wagonAsset = require('../../assets/models/TOOOPblend.glb')
export const blanketAsset = require('../../assets/models/blanket.glb')
export const conductorAsset = require('../../assets/characters/conductor.glb')

const gameAssets = [wagonAsset, blanketAsset, conductorAsset]

export async function preloadGameAssets() {
  await Asset.loadAsync(gameAssets)
}

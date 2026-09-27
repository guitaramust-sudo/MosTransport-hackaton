import { Asset } from 'expo-asset'

export const wagonAsset = require('../../assets/models/TOOOPblend.glb')
export const blanketAsset = require('../../assets/models/blanket.glb')
export const conductorAsset = require('../../assets/characters/conductor.glb')
export const forestAsset = require('../../assets/images/background.png')

const gameAssets = [wagonAsset, blanketAsset, conductorAsset, forestAsset]

export async function preloadGameAssets() {
  await Asset.loadAsync(gameAssets)
}

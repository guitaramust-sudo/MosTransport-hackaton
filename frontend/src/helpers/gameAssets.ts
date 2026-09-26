import { Asset } from 'expo-asset'

export const wagonAsset = require('../../assets/models/first_class_wagon.glb')
export const conductorAsset = require('../../assets/characters/conductor.glb')

const gameAssets = [wagonAsset, conductorAsset]

export async function preloadGameAssets() {
  await Asset.loadAsync(gameAssets)
}

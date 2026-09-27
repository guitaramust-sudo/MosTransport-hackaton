import { Box3, Mesh, type Material, type Object3D } from 'three'
import { buildNavGrid, setActiveNavGrid } from './navGrid'

// Model parts that stand in the walkway and block the route. GLTFLoader turns
// spaces in node names into underscores, so match either.
const HIDDEN_PARTS = /^(Central[ _]service[ _]alcove[ _]angled[ _]privacy[ _]wall|Partly[ _]open[ _]interior[ _]sliding[ _]door|Interior[ _]sliding[ _]door[ _]glass)/

const EXTERIOR_PART = /^VSM[ _]/
// Front bulkhead with the inter-car door is at z≈8.8; the nose shell starts at 8.64.
const NOSE_START_Z = 8.55

/**
 * One-time fix-ups for the wagon model, shared by native and web:
 * - hide parts that block the walkway;
 * - give stacked floor layers (floor, perimeter insert, runner, borders, LED
 *   strips — a few millimetres apart) an explicit draw order, so they don't
 *   z-fight and flicker on devices with a low-precision depth buffer;
 * - build the walkability grid characters path-find on.
 */
export function prepareWagonScene(source: Object3D): Object3D {
  // Work on a copy of the graph (geometry is shared) so other screens that
  // show the same cached model — e.g. the exterior preview — stay untouched.
  const cached = source.userData.vsmGameScene as Object3D | undefined
  if (cached) {
    setActiveNavGrid(cached.userData.vsmNavGrid ?? null)
    return cached
  }
  const scene = source.clone(true)
  scene.traverse((object) => { if (HIDDEN_PARTS.test(object.name)) object.visible = false })

  const box = new Box3()
  const floorLayers: Array<{ mesh: Mesh; top: number }> = []
  scene.updateMatrixWorld(true)
  scene.traverse((object) => {
    if (!(object instanceof Mesh)) return
    box.setFromObject(object)
    // The head-car nose and cab shell stand in front of the front vestibule
    // and cover it from the game camera; the cab itself is not playable.
    if (EXTERIOR_PART.test(object.name) && box.min.z >= NOSE_START_Z) {
      object.visible = false
      return
    }
    const flat = box.max.y - box.min.y < 0.08 && box.max.y < 0.3 && box.min.y > 0.1
    if (flat) floorLayers.push({ mesh: object, top: box.max.y })
  })
  floorLayers.sort((a, b) => a.top - b.top)
  floorLayers.forEach(({ mesh }, rank) => {
    const offset = (material: Material) => {
      const copy = material.clone()
      copy.polygonOffset = true
      copy.polygonOffsetFactor = -rank
      copy.polygonOffsetUnits = -rank * 4
      return copy
    }
    mesh.material = Array.isArray(mesh.material) ? mesh.material.map(offset) : offset(mesh.material)
  })

  const grid = buildNavGrid(scene)
  scene.userData.vsmNavGrid = grid
  source.userData.vsmGameScene = scene
  setActiveNavGrid(grid)
  return scene
}

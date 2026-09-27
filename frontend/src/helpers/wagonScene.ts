import { Box3, Mesh, type Material, type Object3D } from 'three'
import { buildNavGrid, setActiveNavGrid } from './navGrid'

// Model parts that stand in the walkway and block the route, plus the modelling
// preview ground plane. GLTFLoader turns spaces in node names into underscores.
const HIDDEN_PARTS = new RegExp('^(' + [
  'Central service alcove angled privacy wall',
  'Central service alcove angled walnut rail',
  'Central service alcove entry front return',
  'Partly open interior sliding door',
  'Interior sliding door glass',
  'Preview ground',
  // Exported at the origin instead of under the head car. Its dark top face
  // sits above the wood floor by the wardrobe and looks like a black hole.
  'VSM head car front powered bogie frame',
].map((name) => name.replace(/ /g, '[ _]')).join('|') + ')')

const EXTERIOR_PART = /^VSM[ _]/
const ORIGIN_TOLERANCE = 0.03
const FLOOR_TOP_Y = 0.25

function isStrayAtOrigin(box: Box3) {
  const cx = (box.min.x + box.max.x) / 2
  const cy = (box.min.y + box.max.y) / 2
  const cz = (box.min.z + box.max.z) / 2
  const wholeCar = box.max.x - box.min.x > 3 && box.max.z - box.min.z > 3
  return !wholeCar && Math.abs(cx) < ORIGIN_TOLERANCE && Math.abs(cy) < ORIGIN_TOLERANCE && Math.abs(cz) < ORIGIN_TOLERANCE && box.max.y > FLOOR_TOP_Y
}
// Front bulkhead with the inter-car door is at z≈8.8; the nose/cab shell starts at 8.64.
const NOSE_START_Z = 8.55

/**
 * One-time fix-ups for the wagon model, shared by native and web:
 * - hide parts that block the walkway and the modelling preview ground;
 * - make the cab shell fadable so it can reveal the front vestibule;
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
  const cabMeshes: Mesh[] = []
  scene.updateMatrixWorld(true)
  scene.traverse((object) => {
    if (!(object instanceof Mesh)) return
    box.setFromObject(object)
    // The model ships stray copies of parts (corner shells, a vestibule wall,
    // window frames, a luggage niche...) left at the world origin. They stand
    // in the middle of the aisle next to the wardrobe and poke through the floor.
    if (isStrayAtOrigin(box)) {
      object.visible = false
      return
    }
    // The head-car nose and cab shell cover the front vestibule from the game
    // camera. Keep them, with their own materials, so they can be faded out
    // while the conductor is in the vestibule (see CabFade).
    if (EXTERIOR_PART.test(object.name) && box.min.z >= NOSE_START_Z) {
      const fadable = (material: Material) => {
        const copy = material.clone()
        copy.transparent = true
        return copy
      }
      object.material = Array.isArray(object.material) ? object.material.map(fadable) : fadable(object.material)
      cabMeshes.push(object)
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
  scene.userData.vsmCabMeshes = cabMeshes
  source.userData.vsmGameScene = scene
  setActiveNavGrid(grid)
  return scene
}

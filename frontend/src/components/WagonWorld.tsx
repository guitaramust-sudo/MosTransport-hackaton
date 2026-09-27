import { Suspense, useEffect, useMemo, useRef, useState, type MutableRefObject } from 'react'
import { ActivityIndicator, PanResponder, Pressable, StyleSheet, View } from 'react-native'
import { LoopOnce, LoopRepeat, Mesh, RepeatWrapping, SRGBColorSpace, Vector3, type Texture, type AnimationAction, type AnimationClip, type Group, type OrthographicCamera as ThreeOrthographicCamera } from 'three'
import { clone as cloneSkeleton } from 'three/examples/jsm/utils/SkeletonUtils.js'
import { conductorAsset, forestAsset, passengerAssetFor, wagonAsset } from '../helpers/gameAssets'
import { Canvas, canvasGl, OrthographicCamera, useAnimations, useFrame, useGLTF, textureSource, useTexture, useThree, useWagonScene } from '../helpers/three'
import { colors } from '../helpers/theme'
import { AISLE_MAX_Z, AISLE_MIN_Z, aislePoint, wagonAnchorLabels, interpolateWagonActor, pointAlong, pointsOfInterestFor, routeBetween, servicePointFor, wagonAnchorPositions, wagonSituationIcon } from '../helpers/wagonMap'
import type { WagonActor, WagonAnchor, WagonSeat, WagonSituationType, WagonSnapshot } from '../types'
import { Text } from './Typography'
import { WagonBlanket } from './WagonBlanket'
import { prepareWagonScene } from '../helpers/wagonScene'
import { getActiveNavGrid, isWalkable } from '../helpers/navGrid'

useGLTF.preload(wagonAsset)
useGLTF.preload(conductorAsset)

interface WagonWorldProps {
  snapshot: WagonSnapshot
  disabled?: boolean
  onAnchorPress: (anchor: WagonAnchor, nearby: boolean, fromJoystick: boolean) => void
  /** Lessons show their own task card, so the generic hint can be turned off. */
  showHint?: boolean
  /** Reports whether the conductor has walked away from the anchor the server has them at. */
  onAwayChange?: (away: boolean) => void
}

interface PlayerPosition { x: number; y: number; z: number }
interface Marker { anchor: WagonAnchor; x: number; y: number; visible: boolean; icon: string; active: boolean }
/** Joystick deflection in screen space, each axis -1..1 (y grows downwards). */
interface JoystickState { active: boolean; x: number; y: number }
const JOYSTICK_RADIUS = 48
// Drags shorter than this remain taps on interactive objects.
const JOYSTICK_DEAD_ZONE = 10
// A tap still counts as a click after this much pointer travel (px).
const CLICK_SLOP = 6
// Direction offsets (radians) tried when the straight step is blocked.
const STEER_ANGLES = [0, 0.35, -0.35, 0.7, -0.7, 1.05, -1.05, 1.4, -1.4]
const AISLE_X = 0.47
const FLOOR_Y = 0.245
const WALK_SPEED = 2.6

// The conductor always stands on the aisle centerline next to an anchor.
function conductorPoint(anchor: WagonAnchor) {
  const point = aislePoint(wagonAnchorPositions[anchor].z)
  return { x: point.x, y: FLOOR_Y, z: point.z }
}

function interpolateConductor(actor: WagonActor) {
  if (!actor.moving) return { ...conductorPoint(actor.at), progress: 1 }
  const timing = interpolateWagonActor(actor)
  const point = pointAlong(routeBetween(conductorPoint(actor.moving.from), conductorPoint(actor.moving.to)), timing.progress)
  return { x: point.x, y: FLOOR_Y, z: point.z, progress: timing.progress }
}

function Passenger({ seat, type, onPress }: { seat: WagonSeat; type?: WagonSituationType; onPress: () => void }) {
  const group = useRef<Group>(null)
  const interaction = useRef<Group>(null)
  const model = useGLTF(passengerAssetFor(seat.passenger_def_id)) as unknown as { scene: Group; animations: AnimationClip[] }
  // GLTFLoader caches the source scene. Every passenger needs independent bones
  // and animation actions, even when two seats use the same character variant.
  const scene = useMemo(() => cloneSkeleton(model.scene) as Group, [model.scene])
  const { actions } = useAnimations(model.animations, scene) as unknown as { actions: Record<string, AnimationAction | null> }
  const currentAction = useRef<string | null>(null)
  const previousMode = useRef<string | null>(null)
  const sitUntil = useRef(0)
  const seatBlend = useRef(seat.actor.at.startsWith('seat_') && !seat.actor.moving ? 1 : 0)

  useEffect(() => {
    scene.traverse((object) => { if (object instanceof Mesh) object.frustumCulled = false })
    return () => Object.values(actions).forEach((action) => action?.stop())
  }, [actions, scene])

  useFrame(({ clock }, delta) => {
    if (!group.current) return
    const position = interpolateWagonActor(seat.actor)
    const seated = seat.actor.at.startsWith('seat_') && !seat.actor.moving
    // Every seat faces +Z. The seated clip shifts the hips 46 cm toward the
    // backrest, so its root belongs at the front edge of the chair.
    seatBlend.current += ((seated ? 1 : 0) - seatBlend.current) * Math.min(1, delta * 5)
    group.current.position.set(position.x, position.y - 0.08 * seatBlend.current, position.z + 0.52 * seatBlend.current)
    if (interaction.current) interaction.current.position.z = -0.46 * seatBlend.current
    const mode = seat.actor.moving ? 'moving' : seated ? 'seated' : 'standing'
    if (mode !== previousMode.current) {
      if (mode === 'seated' && previousMode.current !== null) sitUntil.current = Date.now() + 2000
      previousMode.current = mode
    }
    const elapsed = seat.actor.moving ? Date.now() - Date.parse(seat.actor.moving.started_at) : 0
    const animation = mode === 'seated'
      ? (Date.now() < sitUntil.current ? 'SitDown' : 'IdleSeated')
      : mode === 'moving'
        ? (seat.actor.moving?.from.startsWith('seat_') && elapsed < 2000 ? 'StandUp' : 'Walk')
        : 'StandUp'
    if (animation !== currentAction.current) {
      if (currentAction.current) actions[currentAction.current]?.fadeOut(0.15)
      const action = actions[animation]
      if (action) {
        action.reset()
        const repeats = animation === 'IdleSeated' || animation === 'Walk'
        action.setLoop(repeats ? LoopRepeat : LoopOnce, repeats ? Infinity : 1)
        action.clampWhenFinished = animation === 'SitDown' || animation === 'StandUp'
        action.fadeIn(0.15).play()
        currentAction.current = animation
      }
    }
    group.current.rotation.y = seated ? 0 : position.heading
    group.current.rotation.z = type === 'cold' ? Math.sin(clock.elapsedTime * 9) * 0.02 : 0
  })

  return (
    <group ref={group} onClick={(event) => { event.stopPropagation(); if (event.delta <= CLICK_SLOP) onPress() }}>
      <primitive object={scene} />
      <group ref={interaction}>
        <mesh position={[0, 0.58, 0]}><sphereGeometry args={[0.42, 12, 12]} /><meshBasicMaterial transparent opacity={0} /></mesh>
        {type && <mesh position={[0, 1.72, 0]}><sphereGeometry args={[0.075, 16, 16]} /><meshStandardMaterial color={type === 'zone_intrusion' ? colors.critical : colors.loyalty} emissive={type === 'zone_intrusion' ? colors.critical : colors.primary} emissiveIntensity={1.1} /></mesh>}
      </group>
    </group>
  )
}

function Conductor({ actor, playerPosition, joystick }: { actor: WagonActor; playerPosition: MutableRefObject<PlayerPosition>; joystick: MutableRefObject<JoystickState> }) {
  const { camera } = useThree()
  const screenRight = useRef(new Vector3())
  const screenUp = useRef(new Vector3())
  const model = useGLTF(conductorAsset) as unknown as { scene: Group; animations: AnimationClip[] }
  const group = useRef<Group>(null)
  const previousMoving = useRef(false)
  const visualPosition = useRef<PlayerPosition>(interpolateConductor(actor))
  const localMode = useRef(false)
  const { actions } = useAnimations(model.animations, model.scene) as unknown as { actions: Record<string, AnimationAction | null> }

  useEffect(() => {
    model.scene.scale.setScalar(1.35)
    model.scene.traverse((object) => { if (object instanceof Mesh) object.frustumCulled = false })
    actions.Idle?.reset().play()
    return () => Object.values(actions).forEach((action) => action?.stop())
  }, [actions, model.scene])

  useFrame((_, delta) => {
    if (!group.current) return
    let moving = false

    if (actor.moving) {
      // The server records which anchor was reached; it must not move the model.
      // The visible position is controlled only by the joystick.
      localMode.current = true
    } else {
      const stick = joystick.current
      const strength = Math.min(1, Math.hypot(stick.x, stick.y))
      if (stick.active && strength > 0.15) {
        // Screen directions projected onto the floor, so "up" on the stick is
        // "away from the camera" whichever way the camera looks.
        screenRight.current.setFromMatrixColumn(camera.matrixWorld, 0).setY(0).normalize()
        screenUp.current.setFromMatrixColumn(camera.matrixWorld, 1).setY(0).normalize()
        const dx = screenRight.current.x * stick.x - screenUp.current.x * stick.y
        const dz = screenRight.current.z * stick.x - screenUp.current.z * stick.y
        const length = Math.hypot(dx, dz) || 1
        const step = WALK_SPEED * strength * delta
        const from = visualPosition.current
        const grid = getActiveNavGrid()
        // Standing in a blocked cell (e.g. pushed there by a seat) must not trap
        // the conductor: any step out is allowed until they are back on free floor.
        const stuck = grid ? !isWalkable(grid, visualPosition.current) : false
        const walkable = (x: number, z: number) => stuck || !grid || isWalkable(grid, { x, z })
        // Try the stick direction first, then gradually turned ones, so the
        // conductor slides along walls and into narrow passages instead of stopping.
        const heading = Math.atan2(dx / length, dz / length)
        let next: { x: number; z: number } | null = null
        for (const turn of STEER_ANGLES) {
          const angle = heading + turn
          const candidate = { x: from.x + Math.sin(angle) * step, z: from.z + Math.cos(angle) * step }
          if (walkable(candidate.x, candidate.z)) { next = candidate; break }
        }
        localMode.current = true
        group.current.rotation.y = Math.atan2(dx, dz)
        if (next) {
          visualPosition.current = { x: next.x, y: FLOOR_Y, z: next.z }
          moving = true
        }
      } else if (!localMode.current) {
        visualPosition.current = conductorPoint(actor.at)
      }
    }

    group.current.position.set(visualPosition.current.x, visualPosition.current.y, visualPosition.current.z)
    playerPosition.current = { ...visualPosition.current }
    if (moving !== previousMoving.current) {
      previousMoving.current = moving
      if (moving) { actions.Idle?.fadeOut(0.15); actions.Walk?.reset().fadeIn(0.15).play() }
      else { actions.Walk?.fadeOut(0.15); actions.Idle?.reset().fadeIn(0.15).play() }
    }
  })
  return <group ref={group}><primitive object={model.scene} /></group>
}

function CameraFollow({ playerPosition }: { playerPosition: MutableRefObject<PlayerPosition> }) {
  const { size } = useThree()
  // Portrait (phones): look down the wagon, the aisle runs vertically.
  // Landscape (PC): look from the side so the wagon runs horizontally.
  const landscape = size.width > size.height * 1.1
  const zoom = landscape ? size.height / 4.2 : size.width / 3.75
  const camera = useRef<ThreeOrthographicCamera>(null)
  const desiredPosition = useRef(new Vector3())
  const lookAt = useRef(new Vector3())

  useFrame((_, delta) => {
    if (!camera.current) return

    const followedZ = Math.max(AISLE_MIN_Z + 1.5, Math.min(AISLE_MAX_Z - 1.5, playerPosition.current.z))
    const smoothing = 1 - Math.exp(-6 * delta)
    if (landscape) desiredPosition.current.set(AISLE_X + 2.0, 18, followedZ)
    else desiredPosition.current.set(3.2, 18, followedZ + 9.4)
    camera.current.position.lerp(desiredPosition.current, smoothing)
    lookAt.current.set(landscape ? AISLE_X - 0.4 : 0, FLOOR_Y, followedZ)
    camera.current.lookAt(lookAt.current)
    camera.current.zoom = zoom
    camera.current.updateProjectionMatrix()
  })

  return <OrthographicCamera ref={camera} makeDefault position={[0, 18, 10]} zoom={zoom} near={4} far={45} />
}

// One forest tile covers FOREST_TILE_W x FOREST_TILE_W * (1024 / 618) world units (image aspect).
const FOREST_TILE_W = 9
const FOREST_SIZE = { width: 48, length: 140 }
const FOREST_SPEED = 0.18 // tiles per second: the train visibly moving

/** Forest ground under the wagon, scrolling backwards along the track. */
function ForestGround() {
  const texture = useTexture(textureSource(forestAsset)) as Texture
  useMemo(() => {
    texture.wrapS = RepeatWrapping
    texture.wrapT = RepeatWrapping
    texture.colorSpace = SRGBColorSpace
    texture.repeat.set(FOREST_SIZE.width / FOREST_TILE_W, FOREST_SIZE.length / (FOREST_TILE_W * (1024 / 618)))
    texture.needsUpdate = true
  }, [texture])
  useFrame((_, delta) => { texture.offset.y = (texture.offset.y + delta * FOREST_SPEED) % 1 })
  return (
    <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.6, 0]} raycast={() => null}>
      <planeGeometry args={[FOREST_SIZE.width, FOREST_SIZE.length]} />
      <meshBasicMaterial map={texture} toneMapped={false} />
    </mesh>
  )
}

// The cab shell sits over the front vestibule in the top-down view: fade it
// while the conductor is in there, show it fully everywhere else.
const CAB_FADE_FROM_Z = 6.2
function CabFade({ scene, playerPosition }: { scene: Group; playerPosition: MutableRefObject<PlayerPosition> }) {
  const opacity = useRef(1)
  useFrame((_, delta) => {
    const meshes = (scene.userData.vsmCabMeshes ?? []) as Mesh[]
    const target = playerPosition.current.z > CAB_FADE_FROM_Z ? 0.15 : 1
    if (Math.abs(opacity.current - target) < 0.005) return
    opacity.current += (target - opacity.current) * Math.min(1, delta * 6)
    for (const mesh of meshes) {
      const materials = Array.isArray(mesh.material) ? mesh.material : [mesh.material]
      for (const material of materials) {
        material.opacity = opacity.current
        material.depthWrite = opacity.current > 0.95
      }
    }
  })
  return null
}

// Farther than this from the server anchor, actions tied to that spot are hidden.
const AWAY_DISTANCE = 0.9
function AwayWatcher({ actor, playerPosition, onAwayChange }: { actor: WagonActor; playerPosition: MutableRefObject<PlayerPosition>; onAwayChange?: (away: boolean) => void }) {
  const away = useRef(false)
  useFrame(() => {
    if (!onAwayChange || actor.moving) return
    const anchor = conductorPoint(actor.at)
    const next = Math.hypot(anchor.x - playerPosition.current.x, anchor.z - playerPosition.current.z) > AWAY_DISTANCE
    if (next !== away.current) { away.current = next; onAwayChange(next) }
  })
  return null
}

function poiIcon(anchor: WagonAnchor) {
  if (anchor === 'service_point' || anchor === 'service_zone') return '+'
  if (anchor === 'staff_zone' || anchor === 'cab_entrance_boundary') return '⌁'
  if (anchor === 'sanitary_zone') return 'WC'
  return '•'
}

const restrictedAnchors = new Set<WagonAnchor>(['staff_zone', 'cab_entrance_boundary'])
// Keep markers out of the bottom action area (inspect / finish buttons, hint)
// so a marker never sits under a button and swallows or loses the tap.
const MARKER_BOTTOM_SAFE = 170

function MarkerProjector({ snapshot, onProject }: { snapshot: WagonSnapshot; onProject: (value: Marker[]) => void }) {
  const point = useRef(new Vector3())
  const previous = useRef('')
  useFrame(({ camera, size }) => {
    const situations = new Map(snapshot.active_situations.map((item) => [item.seat_anchor, item]))
    const anchors = [...snapshot.active_situations.map((item) => item.seat_anchor), ...pointsOfInterestFor(snapshot.wagon_state.class_id)] as WagonAnchor[]
    const markers = anchors.map((anchor) => {
      const seat = snapshot.wagon_state.seats.find((item) => item.anchor === anchor)
      const position = seat && situations.has(anchor) ? interpolateWagonActor(seat.actor) : wagonAnchorPositions[anchor]
      point.current.set(position.x, anchor.startsWith('seat_') ? 1.7 : 0.65, position.z).project(camera)
      const situation = situations.get(anchor)
      const x = Math.round((point.current.x * 0.5 + 0.5) * size.width)
      const y = Math.round((-point.current.y * 0.5 + 0.5) * size.height)
      return { anchor, x, y, visible: point.current.z > -1 && point.current.z < 1 && x > 18 && x < size.width - 18 && y > 150 && y < size.height - MARKER_BOTTOM_SAFE, icon: situation ? wagonSituationIcon(situation.type) : poiIcon(anchor), active: Boolean(situation) }
    })
    const signature = markers.map((item) => `${item.anchor}:${item.x}:${item.y}:${item.visible}:${item.icon}`).join('|')
    if (signature !== previous.current) { previous.current = signature; onProject(markers) }
  })
  return null
}

function Scene({ snapshot, disabled, onAnchorPress, onProject, joystick, playerPosition, onAwayChange }: Omit<WagonWorldProps, 'onAnchorPress'> & { onAnchorPress: (anchor: WagonAnchor) => void; onProject: (value: Marker[]) => void; joystick: MutableRefObject<JoystickState>; playerPosition: MutableRefObject<PlayerPosition> }) {
  const modelScene = useWagonScene()
  // Game copy of the model: walkway blockers and the cab shell hidden, floor
  // layers ordered, walkability grid built.
  const wagonScene = useMemo(() => prepareWagonScene(modelScene), [modelScene])
  const situations = new Map(snapshot.active_situations.map((item) => [item.seat_anchor, item.type]))
  return (
    <>
      <color attach="background" args={['#DCE8F4']} />
      <ambientLight intensity={2.1} />
      <directionalLight position={[4, 9, -2]} intensity={2.5} />
      <Suspense fallback={null}><ForestGround /></Suspense>
      <primitive object={wagonScene} />
      <WagonBlanket onPress={() => { if (!disabled) onAnchorPress(servicePointFor(snapshot.wagon_state.class_id)) }} />
      {snapshot.wagon_state.seats.map((seat) => <Suspense key={seat.anchor} fallback={null}><Passenger seat={seat} type={situations.get(seat.anchor)} onPress={() => { if (!disabled && situations.has(seat.anchor)) onAnchorPress(seat.anchor) }} /></Suspense>)}
      <Conductor actor={snapshot.wagon_state.player} playerPosition={playerPosition} joystick={joystick} />
      <CabFade scene={wagonScene as Group} playerPosition={playerPosition} />
      <AwayWatcher actor={snapshot.wagon_state.player} playerPosition={playerPosition} onAwayChange={onAwayChange} />
      <MarkerProjector snapshot={snapshot} onProject={onProject} />
      <CameraFollow playerPosition={playerPosition} />
    </>
  )
}

export function WagonWorld({ snapshot, disabled, onAnchorPress, showHint = true, onAwayChange }: WagonWorldProps) {
  const [markers, setMarkers] = useState<Marker[]>([])
  // The onboarding hint goes away once the player has used the joystick or interacted.
  const [interacted, setInteracted] = useState(false)
  const handleAnchorPress = (anchor: WagonAnchor, fromJoystick = false) => {
    setInteracted(true)
    const active = snapshot.active_situations.find((item) => item.seat_anchor === anchor)
    const seat = active ? snapshot.wagon_state.seats.find((item) => item.anchor === anchor) : undefined
    const target = conductorPoint(seat?.actor.at ?? anchor)
    const nearby = Math.hypot(target.x - playerPosition.current.x, target.z - playerPosition.current.z) <= AWAY_DISTANCE
    onAnchorPress(anchor, nearby, fromJoystick)
  }

  // Shadow joystick: appears where a drag starts, grey and translucent, and
  // disappears on release. Taps can interact with nearby objects, never walk.
  const playerPosition = useRef<PlayerPosition>(interpolateConductor(snapshot.wagon_state.player))
  const joystick = useRef<JoystickState>({ active: false, x: 0, y: 0 })
  const [stick, setStick] = useState<{ ox: number; oy: number; kx: number; ky: number } | null>(null)
  const latest = useRef({ snapshot, disabled, handleAnchorPress })
  latest.current = { snapshot, disabled, handleAnchorPress }
  const releaseStick = () => {
    joystick.current = { active: false, x: 0, y: 0 }
    setStick(null)
    // Stopping next to a point of interest or a waiting passenger approaches it.
    const { snapshot: snap, handleAnchorPress: approach } = latest.current
    const here = playerPosition.current
    const candidates: WagonAnchor[] = [
      ...pointsOfInterestFor(snap.wagon_state.class_id),
      ...snap.active_situations.map((item) => item.seat_anchor),
    ]
    let nearest: { anchor: WagonAnchor; distance: number } | null = null
    for (const anchor of candidates) {
      if (snap.wagon_state.player.at === anchor) continue
      const seat = snap.wagon_state.seats.find((item) => item.anchor === anchor)
      const point = conductorPoint(seat?.actor.at ?? anchor)
      const distance = Math.hypot(point.x - here.x, point.z - here.z)
      if (distance < 0.9 && (!nearest || distance < nearest.distance)) nearest = { anchor, distance }
    }
    if (nearest) approach(nearest.anchor, true)
  }
  const pan = useMemo(() => PanResponder.create({
    onMoveShouldSetPanResponderCapture: (_, gesture) => !latest.current.disabled && Math.hypot(gesture.dx, gesture.dy) > JOYSTICK_DEAD_ZONE,
    onPanResponderGrant: (event, gesture) => {
      setInteracted(true)
      const ox = event.nativeEvent.locationX - gesture.dx
      const oy = event.nativeEvent.locationY - gesture.dy
      setStick({ ox, oy, kx: 0, ky: 0 })
    },
    onPanResponderMove: (_, gesture) => {
      const length = Math.hypot(gesture.dx, gesture.dy)
      const scale = length > JOYSTICK_RADIUS ? JOYSTICK_RADIUS / length : 1
      const kx = gesture.dx * scale
      const ky = gesture.dy * scale
      joystick.current = { active: true, x: kx / JOYSTICK_RADIUS, y: ky / JOYSTICK_RADIUS }
      setStick((current) => (current ? { ...current, kx, ky } : current))
    },
    onPanResponderRelease: releaseStick,
    onPanResponderTerminate: releaseStick,
    onPanResponderTerminationRequest: () => false,
  }), [])
  return (
    <View style={styles.container} {...pan.panHandlers}>
      <Canvas style={styles.canvas} shadows gl={canvasGl}>
        <Suspense fallback={null}><Scene snapshot={snapshot} disabled={disabled} onAnchorPress={handleAnchorPress} onProject={setMarkers} joystick={joystick} playerPosition={playerPosition} onAwayChange={onAwayChange} /></Suspense>
      </Canvas>
      {stick && (
        <View pointerEvents="none" style={[styles.stickBase, { left: stick.ox - JOYSTICK_RADIUS - 8, top: stick.oy - JOYSTICK_RADIUS - 8 }]}>
          <View style={[styles.stickKnob, { transform: [{ translateX: stick.kx }, { translateY: stick.ky }] }]} />
        </View>
      )}
      {markers.map((marker) => marker.visible && (
        <Pressable key={marker.anchor} accessibilityRole="button" accessibilityLabel={wagonAnchorLabels[marker.anchor]} hitSlop={8} disabled={disabled} onPress={() => handleAnchorPress(marker.anchor)} style={[styles.marker, marker.active && styles.markerActive, restrictedAnchors.has(marker.anchor) && styles.markerRestricted, { left: marker.x - 20, top: marker.y - 20 }]}>
          <Text style={[styles.markerText, !marker.active && styles.markerTextQuiet]}>{marker.icon}</Text>
        </Pressable>
      ))}
      {showHint && !interacted && <View pointerEvents="none" style={styles.tip}><Text style={styles.tipText}>Двигайтесь джойстиком, чтобы подойти к пассажиру</Text></View>}
      {!snapshot && <ActivityIndicator style={StyleSheet.absoluteFill} color={colors.primary} />}
    </View>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, overflow: 'hidden', backgroundColor: '#DCE8F4' }, canvas: { flex: 1 },
  marker: { position: 'absolute', width: 40, height: 40, borderRadius: 20, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,.88)', borderWidth: 2, borderColor: colors.primary, shadowColor: '#06113F', shadowOpacity: .2, shadowRadius: 5, elevation: 5 },
  markerActive: { width: 46, height: 46, borderRadius: 23, marginLeft: -3, marginTop: -3, backgroundColor: '#FFFFFF', borderWidth: 3, borderColor: colors.loyalty },
  markerRestricted: { borderColor: colors.critical }, markerText: { fontSize: 20, color: colors.primary, fontWeight: '900' }, markerTextQuiet: { fontSize: 16 },
  tip: { position: 'absolute', left: 20, right: 20, bottom: 104, alignItems: 'center' },
  stickBase: { position: 'absolute', width: (JOYSTICK_RADIUS + 8) * 2, height: (JOYSTICK_RADIUS + 8) * 2, borderRadius: JOYSTICK_RADIUS + 8, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(60, 64, 72, 0.22)', borderWidth: 1.5, borderColor: 'rgba(255, 255, 255, 0.35)' },
  stickKnob: { width: 52, height: 52, borderRadius: 26, backgroundColor: 'rgba(40, 44, 52, 0.45)', borderWidth: 1.5, borderColor: 'rgba(255, 255, 255, 0.5)' },
  tipText: { color: '#FFFFFF', fontSize: 12, fontWeight: '700', backgroundColor: 'rgba(16,26,61,.72)', paddingHorizontal: 14, paddingVertical: 8, borderRadius: 99 },
})

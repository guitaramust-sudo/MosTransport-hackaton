import { Suspense, useEffect, useRef, useState, type MutableRefObject } from 'react'
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native'
import { Canvas, useFrame, useThree } from '@react-three/fiber/native'
import { OrthographicCamera, useAnimations, useGLTF } from '@react-three/drei/native'
import { Mesh, Vector3, type AnimationAction, type AnimationClip, type Group, type OrthographicCamera as ThreeOrthographicCamera } from 'three'
import { conductorAsset, wagonAsset } from '../helpers/gameAssets'
import { colors } from '../helpers/theme'
import type { ScenarioEvent } from '../types'

useGLTF.preload(wagonAsset)
useGLTF.preload(conductorAsset)

const AISLE_X = 0.47
const AISLE_MIN_X = 0.16
const AISLE_MAX_X = 0.78
const AISLE_MIN_Z = -6.2
const AISLE_MAX_Z = 6.2
const FLOOR_Y = 0.245
const questAnchors: Record<string, { x: number; y: number; z: number }> = {
  medical: { x: -0.32, y: 1.5, z: 3.6 },
  climate: { x: 1.26, y: 1.5, z: -3.6 },
}
const eventTargets: Record<string, number> = { medical: 3.6, climate: -3.6 }

type GameQuest = Pick<ScenarioEvent, 'id' | 'title' | 'location' | 'priority'>

interface GameWorldProps {
  targetEventId: string
  moveRequest: number
  onArrive: () => void
  quests: GameQuest[]
  onQuestPress: (questId: string) => void
}

interface PlayerPosition {
  x: number
  y: number
  z: number
}

interface ProjectedQuest {
  id: string
  x: number
  y: number
  visible: boolean
}

function QuestProjector({ quests, onProject }: {
  quests: GameQuest[]
  onProject: (quests: ProjectedQuest[]) => void
}) {
  const point = useRef(new Vector3())
  const elapsed = useRef(0)
  const previous = useRef('')

  useFrame(({ camera, size }, delta) => {
    elapsed.current += delta
    if (elapsed.current < 0.08) return
    elapsed.current = 0

    const projected = quests.map((quest) => {
      const anchor = questAnchors[quest.id] ?? { x: AISLE_X, y: 1.5, z: 0 }
      point.current.set(anchor.x, anchor.y, anchor.z).project(camera)
      const x = (point.current.x * 0.5 + 0.5) * size.width
      const y = (-point.current.y * 0.5 + 0.5) * size.height
      return {
        id: quest.id,
        x,
        y,
        visible: point.current.z > -1 && point.current.z < 1 && x > -80 && x < size.width + 80 && y > 170 && y < size.height - 90,
      }
    })
    const signature = projected.map((quest) => `${quest.id}:${Math.round(quest.x)}:${Math.round(quest.y)}:${quest.visible}`).join('|')
    if (signature !== previous.current) {
      previous.current = signature
      onProject(projected)
    }
  })

  return (
    <>
      {quests.map((quest) => {
        const anchor = questAnchors[quest.id] ?? { x: AISLE_X, y: 1.5, z: 0 }
        return (
          <group key={quest.id} position={[anchor.x, 0.255, anchor.z]}>
            <mesh rotation={[-Math.PI / 2, 0, 0]}>
              <ringGeometry args={[0.2, 0.28, 24]} />
              <meshBasicMaterial color={quest.priority === 'critical' ? '#F05A67' : '#8B4EE9'} depthWrite={false} />
            </mesh>
          </group>
        )
      })}
    </>
  )
}

function World({ targetEventId, moveRequest, onArrive, playerPosition }: GameWorldProps & {
  playerPosition: MutableRefObject<PlayerPosition>
}) {
  const wagon = useGLTF(wagonAsset) as unknown as { scene: Group }
  const conductor = useGLTF(conductorAsset) as unknown as { scene: Group; animations: AnimationClip[] }
  const character = useRef<Group>(null)
  const target = useRef({ x: AISLE_X, z: 0 })
  const moving = useRef(false)
  const arrivalSent = useRef(true)
  const [destination, setDestination] = useState({ x: AISLE_X, z: 0 })
  const { actions } = useAnimations(conductor.animations, conductor.scene) as unknown as {
    actions: Record<string, AnimationAction | null>
  }

  const startMoving = (x: number, z: number, notifyOnArrival: boolean) => {
    const next = {
      x: Math.max(AISLE_MIN_X, Math.min(AISLE_MAX_X, x)),
      z: Math.max(AISLE_MIN_Z, Math.min(AISLE_MAX_Z, z)),
    }
    target.current = next
    setDestination(next)
    moving.current = true
    arrivalSent.current = !notifyOnArrival
    actions.Idle?.fadeOut(0.18)
    actions.Walk?.reset().setEffectiveTimeScale(1).setEffectiveWeight(1).fadeIn(0.18).play()
  }

  useEffect(() => {
    character.current?.position.set(AISLE_X, FLOOR_Y, 0)
    conductor.scene.position.set(0, 0, 0)
    conductor.scene.scale.setScalar(1.35)
    conductor.scene.traverse((object) => {
      if (object instanceof Mesh) object.frustumCulled = false
    })

    actions.Idle?.reset().setEffectiveWeight(1).fadeIn(0.2).play()
    return () => Object.values(actions).forEach((action) => action?.stop())
  }, [actions, conductor.scene])

  useEffect(() => {
    if (moveRequest === 0) return
    startMoving(AISLE_X, eventTargets[targetEventId] ?? 0, true)
  }, [actions, moveRequest, targetEventId])

  useFrame((_, delta) => {
    if (!character.current) return

    playerPosition.current.x = character.current.position.x
    playerPosition.current.y = character.current.position.y
    playerPosition.current.z = character.current.position.z

    if (!moving.current) return
    const dx = target.current.x - character.current.position.x
    const dz = target.current.z - character.current.position.z
    const distance = Math.hypot(dx, dz)
    character.current.rotation.y = Math.atan2(dx, dz)
    const step = Math.min(distance, delta * 2.25)

    if (distance > 0) {
      character.current.position.x += (dx / distance) * step
      character.current.position.z += (dz / distance) * step
      playerPosition.current.x = character.current.position.x
      playerPosition.current.z = character.current.position.z
    }

    if (distance < 0.04) {
      character.current.position.set(target.current.x, FLOOR_Y, target.current.z)
      moving.current = false
      actions.Walk?.fadeOut(0.18)
      actions.Idle?.reset().fadeIn(0.18).play()
      if (!arrivalSent.current) {
        arrivalSent.current = true
        onArrive()
      }
    }
  })

  return (
    <group>
      <primitive object={wagon.scene} />
      <group ref={character}>
        <primitive object={conductor.scene} />
      </group>
      <mesh
        position={[AISLE_X, FLOOR_Y + 0.01, 0]}
        rotation={[-Math.PI / 2, 0, 0]}
        onPointerDown={(event) => {
          event.stopPropagation()
          startMoving(event.point.x, event.point.z, false)
        }}
      >
        <planeGeometry args={[AISLE_MAX_X - AISLE_MIN_X, AISLE_MAX_Z - AISLE_MIN_Z]} />
        <meshBasicMaterial transparent opacity={0} depthWrite={false} />
      </mesh>
      <mesh position={[destination.x, FLOOR_Y + 0.012, destination.z]} rotation={[-Math.PI / 2, 0, 0]}>
        <ringGeometry args={[0.12, 0.18, 24]} />
        <meshBasicMaterial color="#8B4EE9" transparent opacity={0.8} depthWrite={false} />
      </mesh>
    </group>
  )
}

function GameCamera({ playerPosition }: { playerPosition: MutableRefObject<PlayerPosition> }) {
  const { size } = useThree()
  const zoom = size.width / 3.75
  const camera = useRef<ThreeOrthographicCamera>(null)
  const desiredPosition = useRef(new Vector3())
  const lookAt = useRef(new Vector3())

  useFrame((_, delta) => {
    if (!camera.current) return

    const followedZ = Math.max(-4.5, Math.min(4.5, playerPosition.current.z))
    const smoothing = 1 - Math.exp(-6 * delta)
    desiredPosition.current.set(3.2, 18, followedZ + 9.4)
    camera.current.position.lerp(desiredPosition.current, smoothing)
    lookAt.current.set(0, FLOOR_Y, followedZ)
    camera.current.lookAt(lookAt.current)
    camera.current.updateProjectionMatrix()
  })

  return (
    <OrthographicCamera
      ref={camera}
      makeDefault
      position={[0, 18, 10]}
      zoom={zoom}
      near={0.1}
      far={100}
    />
  )
}

export function GameWorld(props: GameWorldProps) {
  const playerPosition = useRef<PlayerPosition>({ x: AISLE_X, y: FLOOR_Y, z: 0 })
  const [projectedQuests, setProjectedQuests] = useState<ProjectedQuest[]>([])

  return (
    <View style={styles.root}>
      <Suspense fallback={<View style={styles.loader}><ActivityIndicator color={colors.primary} size="large" /></View>}>
        <Canvas>
          <color attach="background" args={['#BFD7CF']} />
          <GameCamera playerPosition={playerPosition} />
          <ambientLight intensity={1.65} />
          <directionalLight position={[6, 14, 8]} intensity={2.2} />
          <World {...props} playerPosition={playerPosition} />
          <QuestProjector quests={props.quests} onProject={setProjectedQuests} />
        </Canvas>
      </Suspense>
      {projectedQuests.map((projected) => {
        const quest = props.quests.find((item) => item.id === projected.id)
        if (!quest || !projected.visible) return null
        return (
          <Pressable
            key={quest.id}
            onPress={() => props.onQuestPress(quest.id)}
            style={[
              styles.questCard,
              quest.priority === 'critical' && styles.questCardCritical,
              { left: projected.x - 68, top: projected.y - 76 },
            ]}
          >
            <View style={[styles.questBadge, quest.priority === 'critical' && styles.questBadgeCritical]}>
              <Text style={styles.questBadgeText}>{quest.priority === 'critical' ? '!' : '◆'}</Text>
            </View>
            <View style={styles.questCopy}>
              <Text numberOfLines={2} style={styles.questTitle}>{quest.title}</Text>
              <Text numberOfLines={1} style={styles.questLocation}>{quest.location}</Text>
            </View>
          </Pressable>
        )
      })}
    </View>
  )
}

const styles = StyleSheet.create({
  root: { position: 'absolute', inset: 0, backgroundColor: '#BFD7CF' },
  loader: { position: 'absolute', inset: 0, alignItems: 'center', justifyContent: 'center' },
  questCard: { position: 'absolute', width: 136, minHeight: 58, flexDirection: 'row', alignItems: 'center', padding: 8, borderRadius: 15, borderWidth: 2, borderColor: '#8B4EE9', backgroundColor: 'rgba(255,255,255,0.96)', elevation: 7 },
  questCardCritical: { borderColor: '#F05A67' },
  questBadge: { width: 32, height: 32, alignItems: 'center', justifyContent: 'center', borderRadius: 11, backgroundColor: '#8B4EE9' },
  questBadgeCritical: { backgroundColor: '#F05A67' },
  questBadgeText: { color: '#FFFFFF', fontSize: 16, fontWeight: '900' },
  questCopy: { flex: 1, marginLeft: 7 },
  questTitle: { color: colors.ink, fontSize: 10, lineHeight: 12, fontWeight: '900' },
  questLocation: { color: colors.muted, fontSize: 7, marginTop: 3 },
})

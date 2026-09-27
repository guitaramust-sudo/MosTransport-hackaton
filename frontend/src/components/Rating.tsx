import { useQuery } from '@tanstack/react-query'
import { ActivityIndicator, StyleSheet, View } from 'react-native'
import { api } from '../api/client'
import { useAppSelector } from '../app/store'
import { colors, radius, shadow, spacing, type } from '../helpers/theme'
import type { LeaderboardScope } from '../types'
import { Icon } from './Icon'
import { Card, Sheet, Stat } from './UI'
import { Text } from './Typography'

const scopeLabels: Record<LeaderboardScope, string> = { brigade: 'Бригада', depot: 'Депо', company: 'Компания' }

function useStanding(scope: LeaderboardScope, enabled: boolean) {
  const playerId = useAppSelector((state) => state.app.auth?.player.id)
  const query = useQuery({ queryKey: ['leaderboard', scope], queryFn: () => api.leaderboard(scope), enabled })
  const entry = query.data?.entries.find((item) => item.player_id === playerId) ?? null
  return { query, entry, size: query.data?.group_size ?? 0 }
}

function ScopeTile({ scope, enabled }: { scope: LeaderboardScope; enabled: boolean }) {
  const { query, entry, size } = useStanding(scope, enabled)
  if (!enabled) return <Stat label={scopeLabels[scope]} value="—" caption="нет группы" accent={colors.muted} />
  if (query.isPending) return <View style={styles.tileLoading}><ActivityIndicator color={colors.action} /></View>
  // Show only what the server actually returned; never invent a place.
  if (query.error || !entry || size < 2) return <Stat label={scopeLabels[scope]} value="—" caption="мало данных" accent={colors.muted} />
  return scope === 'brigade'
    ? <Stat label={scopeLabels[scope]} value={`${entry.rank}-е`} caption={`из ${size}`} />
    : <Stat label={scopeLabels[scope]} value={`${Math.round(entry.percentile)} %`} caption="перцентиль" />
}

export function RatingSheet({ visible, points, onClose }: { visible: boolean; points: number; onClose: () => void }) {
  const brigadeId = useAppSelector((state) => state.app.auth?.player.brigade_id)
  return (
    <Sheet visible={visible} title="Рейтинг" onClose={onClose}>
      <Card style={styles.pointsCard}>
        <View style={styles.pointsIcon}><Icon name="star" size={26} color={colors.action} /></View>
        <View style={styles.pointsBody}>
          <Text style={styles.points}>{points} очков</Text>
          <Text style={styles.pointsCaption}>За зачтённые прохождения</Text>
        </View>
      </Card>
      {visible && (
        <View style={styles.tiles}>
          <ScopeTile scope="brigade" enabled={!!brigadeId} />
          <ScopeTile scope="depot" enabled={false} />
          <ScopeTile scope="company" enabled />
        </View>
      )}
      <Text style={styles.note}>Рейтинг отражает игровые очки и не связан с зарплатой, аттестацией или повышением.</Text>
    </Sheet>
  )
}

const styles = StyleSheet.create({
  pointsCard: { flexDirection: 'row', alignItems: 'center', gap: spacing.md, ...shadow },
  pointsIcon: { width: 56, height: 56, borderRadius: radius.md, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.blueSoft },
  pointsBody: { flex: 1 },
  points: { fontSize: 28, lineHeight: 34, fontWeight: '700', color: colors.ink },
  pointsCaption: { ...type.secondary, color: colors.secondary, marginTop: 2 },
  tiles: { flexDirection: 'row', gap: spacing.xs, marginTop: spacing.xs },
  tileLoading: { flex: 1, minHeight: 104, alignItems: 'center', justifyContent: 'center', borderRadius: radius.card, borderWidth: 1, borderColor: colors.border },
  note: { ...type.secondary, color: colors.secondary, marginTop: spacing.md },
})

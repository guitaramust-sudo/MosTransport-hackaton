import { useEffect, useMemo, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import {
  makeChoice,
  navigate,
  selectEvent,
  useAppDispatch,
  useAppSelector,
} from "../app/store";
import { GameWorld } from "../components/GameWorld";
import { scenarios } from "../data/scenarios";
import { colors, radius, shadow } from "../helpers/theme";
import type { ScenarioChoice } from "../types";

export function SimulationPage() {
  const dispatch = useAppDispatch();
  const state = useAppSelector((root) => root.app);
  const scenario =
    scenarios.find((item) => item.id === state.scenarioId) ?? scenarios[0];
  const event = scenario.events[state.eventIndex];
  const [moveRequest, setMoveRequest] = useState(0);
  const [showChoices, setShowChoices] = useState(false);
  const [startedEventId, setStartedEventId] = useState<string | null>(null);
  const [clock, setClock] = useState({
    eventId: event?.id ?? "",
    remaining: event?.timer ?? 0,
  });
  const handledTimeout = useRef<string | null>(null);
  const [message, setMessage] = useState(
    "Выберите событие и подойдите к пассажиру",
  );

  const openEvents = useMemo(
    () =>
      scenario.events
        .map((item, index) => ({ item, index }))
        .filter(({ item }) => !state.resolvedEventIds.includes(item.id)),
    [scenario.events, state.resolvedEventIds],
  );

  useEffect(() => {
    setClock({ eventId: event?.id ?? "", remaining: event?.timer ?? 0 });
    handledTimeout.current = null;
  }, [event?.id, event?.timer]);

  useEffect(() => {
    if (
      !event?.timer ||
      startedEventId !== event.id ||
      state.resolvedEventIds.includes(event.id) ||
      clock.eventId !== event.id
    )
      return;
    const timer = setInterval(() => {
      setClock((current) =>
        current.eventId === event.id
          ? { ...current, remaining: Math.max(0, current.remaining - 1) }
          : current,
      );
    }, 1000);
    return () => clearInterval(timer);
  }, [
    clock.eventId,
    event?.id,
    event?.timer,
    startedEventId,
    state.resolvedEventIds,
  ]);

  useEffect(() => {
    if (
      !event?.timer ||
      clock.eventId !== event.id ||
      clock.remaining !== 0 ||
      startedEventId !== event.id ||
      state.resolvedEventIds.includes(event.id) ||
      handledTimeout.current === event.id
    )
      return;

    handledTimeout.current = event.id;
    setShowChoices(false);
    setStartedEventId(null);
    const timeoutChoice: ScenarioChoice = {
      id: "timeout",
      title: "Время на реакцию истекло",
      subtitle: "Ситуация ухудшилась из-за бездействия",
      safety: -15,
      loyalty: -4,
      competency: "Приоритизация",
      feedback:
        "Задержка изменила состояние ситуации. Критические сигналы требуют немедленной реакции.",
    };
    dispatch(makeChoice(timeoutChoice));
    setMessage("Время истекло: ситуация получила последствия");
  }, [
    clock.eventId,
    clock.remaining,
    dispatch,
    event?.id,
    event?.timer,
    startedEventId,
    state.resolvedEventIds,
  ]);

  const goToEvent = (index: number) => {
    dispatch(selectEvent(index));
    setShowChoices(false);
    setStartedEventId(null);
    setMoveRequest((value) => value + 1);
    setMessage("Проводница направляется к событию");
  };

  const choose = (choice: ScenarioChoice) => {
    dispatch(makeChoice(choice));
    setShowChoices(false);
    setStartedEventId(null);
    setMessage(choice.feedback);
  };

  return (
    <View style={styles.screen}>
      <GameWorld
        targetEventId={event.id}
        moveRequest={moveRequest}
        quests={openEvents.map(({ item }) => item)}
        onQuestPress={(questId) => {
          const quest = openEvents.find(({ item }) => item.id === questId);
          if (quest) goToEvent(quest.index);
        }}
        onArrive={() => {
          setStartedEventId(event.id);
          setShowChoices(true);
          setMessage("Вы на месте. Примите решение");
        }}
      />

      <View style={styles.topBar}>
        <Pressable
          style={styles.menuButton}
          onPress={() => dispatch(navigate("scenarios"))}
        >
          <Text style={styles.menuIcon}>‹</Text>
        </Pressable>
        <View style={styles.routeBlock}>
          <Text style={styles.route}>ВСМ 001 · МОСКВА → СПБ</Text>
          <Text style={styles.routeTime}>До прибытия 06:42</Text>
        </View>
        <View style={styles.level}>
          <Text style={styles.levelText}>7</Text>
        </View>
      </View>

      <View style={styles.metrics}>
        <View style={styles.metric}>
          <Text style={styles.metricIcon}>◆</Text>
          <View>
            <Text style={styles.metricValue}>{state.safety}</Text>
            <Text style={styles.metricLabel}>Safety</Text>
          </View>
        </View>
        <View style={styles.metric}>
          <Text style={[styles.metricIcon, styles.heart]}>♥</Text>
          <View>
            <Text style={styles.metricValue}>{state.loyalty}</Text>
            <Text style={styles.metricLabel}>Loyalty</Text>
          </View>
        </View>
        <View style={styles.metric}>
          <Text style={[styles.metricIcon, styles.coin]}>●</Text>
          <View>
            <Text style={styles.metricValue}>{state.score}</Text>
            <Text style={styles.metricLabel}>XP</Text>
          </View>
        </View>
      </View>

      <View style={styles.missionCard}>
        <View style={styles.missionIcon}>
          <Text style={styles.missionIconText}>
            {event.priority === "critical" ? "!" : "◆"}
          </Text>
        </View>
        <View style={styles.missionCopy}>
          <Text style={styles.missionLabel}>
            {event.priority === "critical"
              ? "СРОЧНАЯ ЗАДАЧА"
              : "ТЕКУЩАЯ ЗАДАЧА"}
          </Text>
          <Text numberOfLines={1} style={styles.missionTitle}>
            {event.title}
          </Text>
          <View style={styles.progress}>
            <View
              style={[
                styles.progressFill,
                {
                  width: `${(state.resolvedEventIds.length / scenario.events.length) * 100}%`,
                },
              ]}
            />
          </View>
        </View>
        {event.timer ? (
          <View style={styles.timer}>
            <Text style={styles.timerText}>{clock.remaining}</Text>
          </View>
        ) : null}
      </View>

      {false && (
        <View style={styles.eventPins}>
          {openEvents.map(({ item, index }) => (
            <Pressable
              key={item.id}
              onPress={() => goToEvent(index)}
              style={[
                styles.pin,
                item.priority === "critical" && styles.pinCritical,
                state.eventIndex === index && styles.pinActive,
              ]}
            >
              <Text style={styles.pinIcon}>
                {item.priority === "critical" ? "!" : "●"}
              </Text>
              <View style={styles.pinTextBlock}>
                <Text style={styles.pinTitle}>{item.title}</Text>
                <Text style={styles.pinLocation}>{item.location}</Text>
              </View>
              <Text style={styles.pinArrow}>›</Text>
            </Pressable>
          ))}
        </View>
      )}

      {!showChoices && (
        <View style={styles.statusPill}>
          <Text numberOfLines={2} style={styles.statusText}>
            {message}
          </Text>
        </View>
      )}

      {showChoices && (
        <View style={styles.sheet}>
          <View style={styles.sheetHandle} />
          <Text style={styles.sheetKicker}>{event.location}</Text>
          <Text style={styles.sheetTitle}>{event.title}</Text>
          <Text style={styles.sheetDescription}>{event.description}</Text>
          <View style={styles.choiceRow}>
            {event.choices.map((choice, index) => (
              <Pressable
                key={choice.id}
                style={styles.choice}
                onPress={() => choose(choice)}
              >
                <View style={styles.choiceNumber}>
                  <Text style={styles.choiceNumberText}>{index + 1}</Text>
                </View>
                <Text numberOfLines={3} style={styles.choiceText}>
                  {choice.title}
                </Text>
              </Pressable>
            ))}
          </View>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: "#BFD7CF" },
  topBar: {
    position: "absolute",
    top: 10,
    left: 12,
    right: 12,
    flexDirection: "row",
    alignItems: "center",
  },
  menuButton: {
    width: 44,
    height: 44,
    borderRadius: 15,
    backgroundColor: colors.surface,
    alignItems: "center",
    justifyContent: "center",
    ...shadow,
  },
  menuIcon: {
    color: colors.ink,
    fontSize: 31,
    lineHeight: 33,
    fontWeight: "500",
  },
  routeBlock: { flex: 1, alignItems: "center" },
  route: {
    color: colors.ink,
    fontSize: 11,
    fontWeight: "900",
    backgroundColor: "rgba(255,255,255,0.88)",
    paddingHorizontal: 12,
    paddingTop: 7,
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
  },
  routeTime: {
    color: colors.muted,
    fontSize: 10,
    backgroundColor: "rgba(255,255,255,0.88)",
    paddingHorizontal: 12,
    paddingBottom: 7,
    paddingTop: 2,
    borderBottomLeftRadius: 12,
    borderBottomRightRadius: 12,
  },
  level: {
    width: 44,
    height: 44,
    borderRadius: 15,
    backgroundColor: colors.primary,
    alignItems: "center",
    justifyContent: "center",
    borderWidth: 3,
    borderColor: "#FFD875",
  },
  levelText: { color: colors.surface, fontSize: 18, fontWeight: "900" },
  metrics: {
    position: "absolute",
    top: 62,
    left: 12,
    right: 12,
    flexDirection: "row",
    gap: 7,
  },
  metric: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "rgba(25,40,46,0.91)",
    borderRadius: 13,
    paddingVertical: 8,
    gap: 7,
  },
  metricIcon: { color: "#54D39F", fontSize: 15 },
  heart: { color: "#63AAEF" },
  coin: { color: "#FFD05A" },
  metricValue: { color: colors.surface, fontSize: 13, fontWeight: "900" },
  metricLabel: { color: "#AFC0C7", fontSize: 8, fontWeight: "700" },
  missionCard: {
    position: "absolute",
    top: 116,
    left: 18,
    right: 18,
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: "#8B4EE9",
    borderRadius: 18,
    padding: 12,
    borderWidth: 3,
    borderColor: "#7035CE",
    ...shadow,
  },
  missionIcon: {
    width: 45,
    height: 45,
    borderRadius: 13,
    backgroundColor: "#FFD22E",
    alignItems: "center",
    justifyContent: "center",
    borderWidth: 2,
    borderColor: "#FFF2A5",
  },
  missionIconText: { color: "#5A3A00", fontSize: 22, fontWeight: "900" },
  missionCopy: { flex: 1, marginLeft: 11 },
  missionLabel: {
    color: "#EADFFF",
    fontSize: 8,
    fontWeight: "900",
    letterSpacing: 0.8,
  },
  missionTitle: {
    color: colors.surface,
    fontSize: 14,
    fontWeight: "900",
    marginTop: 2,
  },
  progress: {
    height: 7,
    backgroundColor: "#5F28B6",
    borderRadius: radius.pill,
    marginTop: 7,
    overflow: "hidden",
  },
  progressFill: {
    height: "100%",
    backgroundColor: "#FFD22E",
    borderRadius: radius.pill,
  },
  timer: {
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: colors.surface,
    alignItems: "center",
    justifyContent: "center",
    marginLeft: 8,
  },
  timerText: { color: colors.primary, fontSize: 16, fontWeight: "900" },
  eventPins: { position: "absolute", top: 198, left: 12, right: 12, gap: 8 },
  pin: {
    flexDirection: "row",
    alignItems: "center",
    alignSelf: "flex-start",
    maxWidth: 235,
    backgroundColor: "rgba(255,255,255,0.94)",
    borderRadius: 16,
    padding: 9,
    borderWidth: 2,
    borderColor: "#D5E0E3",
    ...shadow,
  },
  pinCritical: { alignSelf: "flex-end", borderColor: "#F0A5AD" },
  pinActive: { borderColor: "#8B4EE9" },
  pinIcon: {
    width: 30,
    height: 30,
    textAlign: "center",
    textAlignVertical: "center",
    color: colors.primary,
    fontSize: 17,
    fontWeight: "900",
    backgroundColor: "#FBE8EB",
    borderRadius: 10,
    overflow: "hidden",
  },
  pinTextBlock: { flexShrink: 1, marginLeft: 8 },
  pinTitle: { color: colors.ink, fontSize: 11, fontWeight: "900" },
  pinLocation: { color: colors.muted, fontSize: 8, marginTop: 2 },
  pinArrow: { color: colors.muted, fontSize: 22, marginLeft: 7 },
  statusPill: {
    position: "absolute",
    bottom: 20,
    left: 30,
    right: 30,
    backgroundColor: "rgba(25,40,46,0.92)",
    borderRadius: radius.pill,
    paddingHorizontal: 18,
    paddingVertical: 12,
  },
  statusText: {
    color: colors.surface,
    textAlign: "center",
    fontSize: 11,
    fontWeight: "700",
    lineHeight: 15,
  },
  sheet: {
    position: "absolute",
    left: 10,
    right: 10,
    bottom: 10,
    backgroundColor: colors.surface,
    borderRadius: 24,
    padding: 16,
    ...shadow,
  },
  sheetHandle: {
    width: 36,
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.border,
    alignSelf: "center",
    marginBottom: 10,
  },
  sheetKicker: {
    color: colors.primary,
    fontSize: 9,
    fontWeight: "900",
    letterSpacing: 0.8,
  },
  sheetTitle: {
    color: colors.ink,
    fontSize: 19,
    fontWeight: "900",
    marginTop: 3,
  },
  sheetDescription: {
    color: colors.muted,
    fontSize: 11,
    lineHeight: 16,
    marginTop: 5,
  },
  choiceRow: { flexDirection: "row", gap: 7, marginTop: 13 },
  choice: {
    flex: 1,
    minHeight: 88,
    backgroundColor: colors.soft,
    borderRadius: 14,
    padding: 9,
    borderWidth: 1,
    borderColor: colors.border,
  },
  choiceNumber: {
    width: 23,
    height: 23,
    borderRadius: 8,
    backgroundColor: colors.dark,
    alignItems: "center",
    justifyContent: "center",
    marginBottom: 7,
  },
  choiceNumberText: { color: colors.surface, fontSize: 10, fontWeight: "900" },
  choiceText: {
    color: colors.ink,
    fontSize: 10,
    lineHeight: 14,
    fontWeight: "800",
  },
});

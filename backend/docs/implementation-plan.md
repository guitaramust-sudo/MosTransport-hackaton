# Backend implementation plan

This plan turns the requirements in the hackathon brief, QA summary, and VSM
game design specification v1.0 into independently reviewable changes. The
existing dialogue API stays available while the simulation grows. Game design
numbers and professional procedures remain demo assumptions until reviewed by
the relevant subject matter experts.

Each completed stage gets its own commit and push to `main`. Commit bodies must
describe behavior, compatibility, and verification.

| Stage | Work | Done when |
| --- | --- | --- |
| 1 | Secure admin access and mark demo content | Public registration cannot claim an admin email; demo scenarios cannot silently become approved professional training. Complete. |
| 2 | Branching scenario actions | A server-side action with a stable command ID and expected version applies configured conditions/effects atomically; at least two choices lead to different subsequent events. Complete for the demo template. |
| 3 | Concurrent events, observation, and wagon zones | Two events can coexist; an indirect cue is discovered through an action; movement and interaction have server-controlled spatial/time costs. Complete for the demo template. |
| 4 | Timers, causal log, and debrief | A server deadline changes event state once; the action/effect log explains every score change and an alternative; the whole session is scored from that log. Complete for the demo template. |
| 5 | Achievements and notifications | At least two observable achievements, one challenge, and new-scenario/challenge notifications are persisted and issued idempotently. Complete for the demo flow. |
| 6 | Competency points and leaderboard | Eligible results earn points in a namespace-specific ledger; group rank, size, and percentile use the full cohort; XP remains a separate progression metric. Complete with provisional demo coefficients. |
| 7 | Integration and release checks | Tests cover branching, concurrency, timeout, replay, duplicate commands/awards, authorization, and API compatibility; docs and demo flow match the code. Complete: Go test/race/vet/build and Docker API smoke flow passed. |

Stage 1 is a prerequisite for a publicly reachable demo. Stages 2–4 form one
playable vertical slice. The legacy dialogue flow can be retired only after the
new flow is integrated with the client and its behavior is covered by tests.

Release note: the bundled content and all scoring coefficients are provisional
demo material. The new simulation API is playable and verified independently;
the existing frontend still uses the legacy dialogue flow. An expert content
review and client integration are separate follow-up work before professional
training use.

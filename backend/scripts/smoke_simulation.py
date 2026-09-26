#!/usr/bin/env python3
"""Exercise the public demo flow against a running backend.

Usage: python3 scripts/smoke_simulation.py [http://localhost:8088]
Creates two temporary player accounts; it does not require admin credentials.
"""

import json
import sys
import urllib.error
import urllib.request
import uuid


BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8088").rstrip("/")


def request(method, path, token=None, body=None, expected=200):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as response:
            status, raw = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, raw = error.code, error.read()
    assert status == expected, f"{method} {path}: expected {expected}, got {status}: {raw[:500]!r}"
    return json.loads(raw) if raw else None


def register():
    ident = uuid.uuid4().hex
    response = request("POST", "/auth/register", body={
        "email": f"smoke-{ident}@example.invalid",
        "username": f"smoke-{ident[:8]}",
        "password": uuid.uuid4().hex,
    }, expected=201)
    assert response["player"]["role"] == "user"
    return response["player"]["id"], response["tokens"]["access_token"]


def run(token):
    view = request("POST", "/api/session/simulations", token, {}, 201)
    run_id = view["run"]["id"]
    variant = view["run"]["seed_variant"]
    steps = [
        {"event_id": "service_request", "choice_id": "check_availability"},
        {"event_id": "confirmed_request", "choice_id": "explain_next_step"},
        {"event_id": "seat_conflict", "choice_id": "check_tickets"},
        {"action_id": "move_to", "target": "luggage_zone"},
        {"action_id": "inspect"},
        {"event_id": "wet_floor", "choice_id": "report_spill"},
    ]
    first_command = None
    for index, step in enumerate(steps):
        command = {"command_id": str(uuid.uuid4()),
                   "expected_state_version": view["run"]["state_version"], **step}
        view = request("POST", f"/api/session/simulations/{run_id}/actions", token, command)
        if index == 0:
            first_command = command
    assert view["run"]["status"] == "finished"
    result = request("GET", f"/api/session/simulations/{run_id}/result", token)
    assert result["session_pass"] and len(result["debrief"]) == 6
    assert result["leaderboard_points_delta"] in (10, 20)
    assert request("POST", f"/api/session/simulations/{run_id}/actions", token, first_command)["run"]["state_version"] == 1
    assert request("GET", f"/api/session/simulations/{run_id}/result", token)["leaderboard_points_delta"] == result["leaderboard_points_delta"]
    return run_id, variant, result


def main():
    with urllib.request.urlopen(BASE + "/readyz", timeout=15) as response:
        assert response.status == 200
    player_id, token = register()
    _, other_token = register()
    first_id, first_variant, first = run(token)
    request("GET", f"/api/session/simulations/{first_id}", other_token, expected=404)
    _, second_variant, second = run(token)
    assert {first_variant, second_variant} == {"A", "B"}
    assert first["leaderboard_points_delta"] == 20
    assert second["leaderboard_points_delta"] == 10
    assert second["leaderboard_points_total"] == 30
    profile = request("GET", "/api/profile", token)
    assert profile["player"]["total_xp"] == 5
    assert profile["leaderboard_points_total"] == 30
    assert {"first_complete", "first_signal"}.issubset(profile["achievements"])
    challenge = request("GET", "/api/challenges/weekly", token)
    assert challenge["completed"] and challenge["seed_variants"] == 2
    notices = request("GET", "/api/notifications", token)["notifications"]
    assert {"new_scenario", "challenge_started", "challenge_completed"}.issubset({n["type"] for n in notices})
    board = request("GET", "/api/leaderboards?scope=company", token)
    assert any(e["player_id"] == player_id and e["leaderboard_points_total"] == 30 for e in board["entries"])
    assert board["group_size"] >= len(board["entries"])
    print("PASS: auth, branching, replay, ownership, rewards, notifications, challenge, leaderboard")


if __name__ == "__main__":
    main()

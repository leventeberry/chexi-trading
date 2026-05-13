#!/usr/bin/env python3
"""
End-to-end check against a running Docker API stack:
  signup → resend verification → TOTP enroll → MFA login → create org → invite user B → B registers → B accepts invite.

Requires:
  - Stack up: docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml [--env-file docker/.env] up -d --build
  - MFA_ENCRYPTION_KEY set on chexi-api (docker-compose.dev.yml supplies a dev default)
  - Redis worker processes invitation emails
  - EMAIL_PROVIDER=log: invitation token appears in `docker logs <chexi-api-container>`; EMAIL_PROVIDER=resend: deliveries also appear in the Resend dashboard.
    With EMAIL_REDIRECT_ALL_TO set on the API container, logs show `to=<redirect>`; this script reads EMAIL_REDIRECT_ALL_TO via `docker exec printenv` when matching invite lines.

Usage:
  python3 scripts/docker_e2e_signup_mfa_org_invite.py
  BASE_URL=http://127.0.0.1:8080 DOCKER_API_CONTAINER=chexi-trading-chexi-api-1 python3 scripts/docker_e2e_signup_mfa_org_invite.py
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import re
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request
from typing import Any


BASE_URL = os.environ.get("BASE_URL", "http://127.0.0.1:8080").rstrip("/")
DOCKER_API_CONTAINER = os.environ.get("DOCKER_API_CONTAINER", "chexi-trading-chexi-api-1")
PASSWORD = os.environ.get("E2E_PASSWORD", "Testpass1!x")


def totp_sha1(secret_b32: str, for_time: float | None = None) -> str:
    """RFC 6238 TOTP (6 digits, SHA1, 30s step) — matches github.com/pquerna/otp defaults."""
    secret_b32 = secret_b32.strip().replace(" ", "").upper()
    pad = "=" * (-len(secret_b32) % 8)
    key = base64.b32decode(secret_b32 + pad)
    t = int(time.time()) if for_time is None else int(for_time)
    counter = t // 30
    msg = struct.pack(">Q", counter)
    hm = hmac.new(key, msg, hashlib.sha1).digest()
    offset = hm[-1] & 0x0F
    code = struct.unpack(">I", hm[offset : offset + 4])[0] & 0x7FFFFFFF
    return str(code % 1_000_000).zfill(6)


def http_json(method: str, path: str, body: dict | None = None, token: str | None = None) -> tuple[int, Any]:
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(f"{BASE_URL}{path}", data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode()
            if resp.status == 204 or not raw.strip():
                return resp.status, {}
            return resp.status, json.loads(raw)
    except urllib.error.HTTPError as e:
        raw = e.read().decode()
        try:
            return e.code, json.loads(raw)
        except json.JSONDecodeError:
            return e.code, {"_raw": raw}


def assert_ok(code: int, body: Any, ctx: str) -> None:
    if code >= 400:
        raise SystemExit(f"FAIL {ctx}: HTTP {code} {body}")


def docker_logs_tail(container: str, lines: int = 400) -> str:
    out = subprocess.run(
        ["docker", "logs", "--tail", str(lines), container],
        capture_output=True,
        text=True,
        timeout=30,
    )
    return out.stdout + out.stderr


def container_env(name: str, container: str) -> str:
    """Best-effort read of env inside the API container (e.g. EMAIL_REDIRECT_ALL_TO for log matching)."""
    try:
        out = subprocess.run(
            ["docker", "exec", container, "printenv", name],
            capture_output=True,
            text=True,
            timeout=15,
        )
        if out.returncode != 0:
            return ""
        return out.stdout.strip()
    except (OSError, subprocess.TimeoutExpired):
        return ""


def verification_log_match_strings(mailbox_email: str, container: str) -> list[str]:
    """Substrings on the verification email log line (recipient or redirect inbox)."""
    out: list[str] = []
    for s in (
        mailbox_email,
        os.environ.get("EMAIL_REDIRECT_ALL_TO", "").strip(),
        container_env("EMAIL_REDIRECT_ALL_TO", container),
    ):
        if s and s not in out:
            out.append(s)
    return out


def extract_verification_raw_token(logs: str, mailbox_email: str, container: str) -> str | None:
    """Parse email verification raw token from docker logs (kind=verification)."""
    patterns = [
        r"verify-email\?token=([A-Za-z0-9_-]{20,})",
        r'"token"\s*:\s*"([A-Za-z0-9_-]{20,})"',
        r'"token":"([A-Za-z0-9_-]{20,})"',
        r'\\"token\\":\\"([A-Za-z0-9_-]{20,})\\"',
    ]
    match_any = verification_log_match_strings(mailbox_email, container)
    candidates: list[str] = []
    for line in logs.splitlines():
        if "kind=verification" not in line and '"kind":"verification"' not in line:
            continue
        if not any(m in line for m in match_any):
            continue
        for pat in patterns:
            m = re.search(pat, line)
            if m:
                candidates.append(m.group(1))
    if not candidates:
        return None
    return candidates[-1]


def invite_log_match_strings(invitee_email: str, container: str) -> list[str]:
    """Substrings that appear on the invite log line (real To: or redirect inbox when using EMAIL_REDIRECT_ALL_TO)."""
    out: list[str] = []
    for s in (
        invitee_email,
        os.environ.get("E2E_INVITE_LOG_EMAIL", "").strip(),
        os.environ.get("EMAIL_REDIRECT_ALL_TO", "").strip(),
        container_env("EMAIL_REDIRECT_ALL_TO", container),
    ):
        if s and s not in out:
            out.append(s)
    return out


def extract_invite_raw_token(logs: str, invitee_email: str, container: str) -> str | None:
    """Parse invite raw token from docker logs (organization_invitation line)."""
    patterns = [
        r'"token"\s*:\s*"([A-Za-z0-9_-]{20,})"',
        r'"token":"([A-Za-z0-9_-]{20,})"',
        r'JSON:\s*\{\s*"token"\s*:\s*"([A-Za-z0-9_-]{20,})"\s*\}',
        # Zerolog JSON-escapes quotes inside the body string (e.g. {\"token\":\"...\"})
        r'\\"token\\":\\"([A-Za-z0-9_-]{20,})\\"',
        r'\{"token":"([A-Za-z0-9_-]{20,})"\}',
    ]
    match_any = invite_log_match_strings(invitee_email, container)
    candidates: list[str] = []
    for line in logs.splitlines():
        if "organization_invitation" not in line:
            continue
        if not any(m in line for m in match_any):
            continue
        for pat in patterns:
            m = re.search(pat, line)
            if m:
                candidates.append(m.group(1))
    if not candidates:
        return None
    return candidates[-1]


def main() -> None:
    ts = int(time.time())
    email_a = os.environ.get("E2E_EMAIL_A", f"owner-{ts}@example.com")
    email_b = os.environ.get("E2E_EMAIL_B", f"member-{ts}@example.com")
    slug = f"e2e-org-{ts}"

    print("--- 1) Register user A ---")
    st, reg = http_json(
        "POST",
        "/api/v1/register",
        {
            "first_name": "Owner",
            "last_name": "E2E",
            "email": email_a,
            "password": PASSWORD,
        },
    )
    assert_ok(st, reg, "register A")
    jwt_a = None
    tok_obj = reg.get("token")
    if isinstance(tok_obj, dict) and tok_obj.get("jwt_token"):
        jwt_a = tok_obj["jwt_token"]
        print(f"    user A: {email_a} (JWT from register; EMAIL_ENABLED likely false)")
    else:
        print(f"    user A: {email_a}")

    print("--- 2) Resend verification email (generic ack) ---")
    st, resend = http_json("POST", "/api/v1/resend-verification", {"email": email_a})
    assert_ok(st, resend, "resend-verification")
    want_ack = "If an account exists for that email and verification is required, instructions were sent."
    if resend.get("message") != want_ack:
        raise SystemExit(f"unexpected resend message: {resend}")

    if not jwt_a:
        print("--- 2b) Verify email A (token from docker logs or inbox) ---")
        raw_v = None
        for attempt in range(15):
            time.sleep(1)
            logs = docker_logs_tail(DOCKER_API_CONTAINER, 600)
            raw_v = extract_verification_raw_token(logs, email_a, DOCKER_API_CONTAINER)
            if raw_v:
                break
        if not raw_v:
            print(docker_logs_tail(DOCKER_API_CONTAINER, 120))
            raise SystemExit("Could not parse email verification token from docker logs.")
        st, _ver = http_json("POST", "/api/v1/verify-email", {"token": raw_v})
        assert_ok(st, _ver, "verify-email A")
        st, login_a = http_json("POST", "/api/v1/login", {"email": email_a, "password": PASSWORD})
        assert_ok(st, login_a, "login A after verify")
        lo = login_a.get("token") or {}
        jwt_a = lo.get("jwt_token") if isinstance(lo, dict) else None
        if not jwt_a:
            raise SystemExit(f"expected jwt after login, got {login_a}")

    print("--- 3) MFA TOTP setup + confirm ---")
    st, setup = http_json("POST", "/api/v1/mfa/totp/setup", {}, jwt_a)
    assert_ok(st, setup, "mfa totp setup")
    secret = setup["secret"]

    code = totp_sha1(secret)
    st, conf = http_json("POST", "/api/v1/mfa/totp/confirm", {"code": code}, jwt_a)
    assert_ok(st, conf, "mfa totp confirm")

    print("--- 4) Login with password → MFA challenge → verify-mfa ---")
    st, login1 = http_json("POST", "/api/v1/login", {"email": email_a, "password": PASSWORD})
    assert_ok(st, login1, "login step 1")
    if not login1.get("mfa_required"):
        raise SystemExit(f"expected MFA challenge, got {login1}")
    challenge = login1["mfa_challenge_token"]
    code2 = totp_sha1(secret)
    st, login2 = http_json(
        "POST",
        "/api/v1/login/verify-mfa",
        {"mfa_challenge_token": challenge, "code": code2},
    )
    assert_ok(st, login2, "verify-mfa")
    jwt_a = login2["token"]["jwt_token"]

    print("--- 5) Create organization ---")
    st, org = http_json(
        "POST",
        "/api/v1/organizations",
        {"name": "E2E Org", "slug": slug},
        jwt_a,
    )
    assert_ok(st, org, "create org")
    org_id = org["id"]
    print(f"    org: {org_id} slug={slug}")

    print("--- 6) Invite user B (email job → log sink) ---")
    st, inv = http_json(
        "POST",
        f"/api/v1/organizations/{org_id}/invitations",
        {"email": email_b, "role": "member"},
        jwt_a,
    )
    assert_ok(st, inv, "create invitation")

    raw_token = None
    for attempt in range(15):
        time.sleep(1)
        logs = docker_logs_tail(DOCKER_API_CONTAINER, 500)
        raw_token = extract_invite_raw_token(logs, email_b, DOCKER_API_CONTAINER)
        if raw_token:
            break
    if not raw_token:
        print(docker_logs_tail(DOCKER_API_CONTAINER, 120))
        raise SystemExit(
            "Could not parse invitation token from docker logs. "
            "Ensure DOCKER_API_CONTAINER is correct (docker ps) and email worker ran."
        )

    print("--- 7) Register user B (same email as invite) ---")
    st, regb = http_json(
        "POST",
        "/api/v1/register",
        {
            "first_name": "Member",
            "last_name": "E2E",
            "email": email_b,
            "password": PASSWORD,
        },
    )
    assert_ok(st, regb, "register B")
    jwt_b = None
    tb = regb.get("token")
    if isinstance(tb, dict) and tb.get("jwt_token"):
        jwt_b = tb["jwt_token"]
    if not jwt_b:
        raw_vb = None
        for attempt in range(15):
            time.sleep(1)
            logs_b = docker_logs_tail(DOCKER_API_CONTAINER, 600)
            raw_vb = extract_verification_raw_token(logs_b, email_b, DOCKER_API_CONTAINER)
            if raw_vb:
                break
        if not raw_vb:
            raise SystemExit("Could not parse verification token for user B from docker logs.")
        st, _vb = http_json("POST", "/api/v1/verify-email", {"token": raw_vb})
        assert_ok(st, _vb, "verify-email B")
        st, login_b = http_json("POST", "/api/v1/login", {"email": email_b, "password": PASSWORD})
        assert_ok(st, login_b, "login B after verify")
        lb = login_b.get("token") or {}
        jwt_b = lb.get("jwt_token") if isinstance(lb, dict) else None
        if not jwt_b:
            raise SystemExit(f"expected jwt for B after login, got {login_b}")

    print("--- 8) User B accepts invitation ---")
    st, acc = http_json(
        "POST",
        "/api/v1/organizations/invitations/accept",
        {"token": raw_token},
        jwt_b,
    )
    assert_ok(st, acc, "accept invite")  # 204 No Content -> urllib might give empty body

    print("--- 9) Verify B sees the org ---")
    st, list_b = http_json("GET", "/api/v1/organizations", None, jwt_b)
    assert_ok(st, list_b, "B list orgs")
    ids = [o.get("id") for o in list_b]
    if org_id not in ids:
        raise SystemExit(f"B should see org {org_id}, got {ids}")

    print("")
    print("PASS: signup → resend verification → MFA → org → invite → register B → accept")
    print(f"  A: {email_a}")
    print(f"  B: {email_b}")
    print(f"  Org: {slug}")


if __name__ == "__main__":
    main()

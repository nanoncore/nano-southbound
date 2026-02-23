# OMCI Wi-Fi Management Spec (V-SOL)

Version: 1.0.0  
Last Updated: 2026-02-21  
Canonical Source: `nanoncore/docs/omci-wifi-spec.md`

## 1. Scope

This spec defines the cross-repository integration contract for Wi-Fi management on V-SOL via OMCI, alongside existing ACS/TR-069 flows.

Phase: MVP (V-SOL only).

## 2. Core Statement

The ONU profile is the switchboard that decides which plane owns Wi-Fi (`ACS`, `OMCI`, `UNMANAGED`) and what defaults/capabilities apply.

## 3. Management Model

### 3.1 Profile-owned manager selection

- Field: `wifiManagementMode = ACS | OMCI | UNMANAGED`
- Location: ONU profile
- Subscriber/ONU override: **not supported in MVP**

### 3.2 Profile-driven and ONU-driven OMCI resolution

V-SOL behavior may require both profile flags and ONU-level CLI commands.

Resolution rule for `wifiManagementMode=OMCI`:
1. Validate profile is OMCI-ready (logical precheck).
2. Validate runtime OLT state is OMCI-ready (execution precheck).
3. Apply ONU-level Wi-Fi commands.

If profile/runtime readiness cannot be guaranteed, fail with `PROFILE_NOT_OMCI_READY` (MVP).  
Optional future split: `PROFILE_RUNTIME_MISMATCH`.

## 4. Plane Guardrails

- Exactly one active manager at a time.
- One active Wi-Fi job per ONU **overall** (cross-plane), not per-plane.
- Mode changes are rejected if any active Wi-Fi job exists (ACS or OMCI).
- If mode is `OMCI`, ACS apply endpoints/actions are blocked.
- If mode is `ACS`, OMCI apply endpoints/actions are blocked.

## 5. Job Snapshot Invariant

At Wi-Fi job creation, persist immutable snapshots:
- `managerModeSnapshot`
- `profileIdSnapshot`
- `profileVersionSnapshot` (if available)
- `wifiManagementSnapshot`

Execution always uses snapshot values, never live-mutated profile fields.

## 6. Job States and Cancellation

### 6.1 OMCI job states

Active states:
- `OMCI_JOB_PENDING`
- `OMCI_JOB_SENT`

Terminal states:
- `OMCI_JOB_APPLIED` (only when verified by readback)
- `OMCI_JOB_SENT_OK` (sent successfully but not verifiable)
- `OMCI_JOB_FAILED`

### 6.2 ACS job states

Keep existing ACS lifecycle semantics (async inform verification).

### 6.3 Cancellation

- OMCI: `CANCEL_UNSUPPORTED` in MVP.
- ACS: optional cancellation before job is sent; otherwise unsupported.

## 7. Desired vs Observed Semantics

Unified status must include plane-neutral intent:
- `desiredConfig` (password excluded/redacted)
- `desiredAt`
- `desiredBy`

Observed truth must be explicit:
- `observedConfig` is non-null only when readback/verification exists.
- `observedSource = OMCI_READBACK | ACS_VERIFY | null`
- `observedAt`
- `verificationLevel = VERIFIED | NOT_VERIFIED | PENDING`

If OMCI readback is unavailable:
- `observedConfig=null`
- final state should be `OMCI_JOB_SENT_OK`
- include reason text in UI/API.

## 8. API/Southbound Contract

### 8.1 Plane-neutral apply entry

Cloud receives plane-neutral apply request and resolves manager from profile snapshot.

Agent/southbound resolution rule:
- Cloud sends `onuSerial` (vendor-agnostic).
- Agent/southbound resolves `ponPort` + `onuId` if vendor requires it.
- If lookup fails:
  - `ONU_OFFLINE`: ONU exists in OLT inventory but operational state is down.
  - `ONU_NOT_FOUND`: ONU serial cannot be found in OLT inventory.

### 8.2 WifiManager interface (southbound)

```text
GetWifiConfig(onu) -> WifiActionResult (optional per model)
SetWifiConfig(onu, config) -> WifiActionResult
SetWifiEnabled(onu, enabled) -> WifiActionResult (optional convenience)
```

Result shape:

```json
{
  "ok": true,
  "errorCode": null,
  "rawOutput": "<redacted cli output>",
  "observedConfig": null,
  "observedSource": null,
  "observedAt": null,
  "failedStep": null,
  "events": [
    {"step": "SET_SSID", "ok": true, "timestamp": "2026-02-21T10:00:00Z"}
  ]
}
```

## 9. Atomicity and Partial Apply

Apply of SSID/password/enabled should be atomic when platform supports transaction/commit.

If multiple commands are required and partial success occurs:
- End job as `OMCI_JOB_FAILED`
- `errorCode=PARTIAL_APPLY`
- Include `failedStep` and sanitized step events (`SET_SSID`, `SET_PASSWORD`, `ENABLE_WIFI`)

## 10. Idempotency and Rate Limiting

### 10.1 Config normalization for idempotency

Normalization inputs:
- `ssid`: trim leading/trailing whitespace
- preserve SSID case (SSID is case-sensitive)
- `enabled`: canonical boolean
- `password`: never store plaintext in key derivation

Use:
- `passwordHmac = HMAC(appKey, password)`
- `idempotencyKey = hash(onuSerial + normalizedConfig + plane + profileIdSnapshot + managerModeSnapshot)`

If same active job with same key exists: return existing job.

### 10.2 Rate limits

Recommended MVP safety throttles:
- apply: max 1 request / ONU / 10s
- readback: max 1 request / ONU / 10s (if read API enabled)

Return `RATE_LIMITED` on violations.

## 11. Timeout and Retry

- Default OMCI timeout per step: `30s` (tune from lab captures).
- On timeout:
  - terminal state: `OMCI_JOB_FAILED`
  - `errorCode=COMMAND_TIMEOUT`
  - set `failedStep`
- Auto retry: none in MVP.
- Manual retry: allowed.

## 12. Mode-switch Semantics

When switching manager mode after terminal jobs:
- `ACS -> OMCI`: ACS state becomes non-authoritative for Wi-Fi.
- `OMCI -> ACS`: ACS becomes authoritative; prior OMCI-applied state may drift.

Operational metadata:
- record audit event (`wifiManagerChangedAt` recommended)
- show banner for 24h: "Wi-Fi manager switched to OMCI" (or ACS)

## 13. Error Codes (MVP)

- `PROFILE_NOT_OMCI_READY`
- `ONU_OFFLINE`
- `ONU_NOT_FOUND`
- `PARTIAL_APPLY`
- `COMMAND_TIMEOUT`
- `READBACK_UNAVAILABLE`
- `CANCEL_UNSUPPORTED`
- `RATE_LIMITED`
- `INVALID_VALUE`
- `PERMISSION_DENIED`
- `INTERNAL_ERROR`

## 14. UI Contract

Wireless card must show:
- `Manager: ACS | OLT (OMCI) | Unmanaged`
- `Execution plane`
- `Verification level`
- `Last job`
- `Observed via`
- Plane-specific reason string (examples):
  - "Sent to OLT, readback unavailable on this ONU model."
  - "ONU offline on OLT."
  - "Profile not configured for OMCI Wi-Fi."

Header hint:
- `Manager: OLT (OMCI) - set by ONU Profile`
- include `Open profile` action.

If manager=OMCI:
- ACS observed fields must be omitted or marked non-authoritative.

## 15. Simulator Parity

Parity goal:
- Parser-compatible behavior is mandatory.
- Byte-close fixtures are nice-to-have.

Fixtures must capture:
- prompt line (`#`, `>`, `(config)#`)
- command echo
- raw output
- return indicator
- timing behavior (including timeout cases)

Required fixture cases:
- success
- ONU not found
- ONU offline
- command timeout
- partial apply

## 16. Acceptance Criteria (MVP)

### Scenario: Profile controls plane
- **Given** ONU profile has `wifiManagementMode=OMCI`
- **When** operator applies SSID/password
- **Then** OMCI commands execute through southbound
- **And** UI shows `Manager: OLT (OMCI)`
- **And** job ends `OMCI_JOB_APPLIED` or `OMCI_JOB_SENT_OK` based on verification support

### Scenario: Dual-management prevention
- **Given** ONU profile has `wifiManagementMode=OMCI`
- **When** ACS Wi-Fi apply is attempted
- **Then** request is blocked
- **And** user sees "Wi-Fi managed via OLT (OMCI)"

### Scenario: Truthful observed state
- **Given** ONU model has no OMCI readback support
- **When** apply succeeds
- **Then** `observedConfig` is `null`
- **And** status is `OMCI_JOB_SENT_OK`
- **And** `verificationLevel=NOT_VERIFIED`

### Scenario: Cross-plane active-job lock
- **Given** an active Wi-Fi job exists for an ONU
- **When** any new Wi-Fi apply is requested on any plane
- **Then** request is rejected until active job is terminal


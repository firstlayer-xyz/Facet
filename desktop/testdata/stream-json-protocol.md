# Verified `claude` stream-json protocol (Task 1 probe)

Binary: `claude` 2.1.172. Probed 2026-06-15 against the real CLI. Raw transcripts:
`/tmp/facet-stream-probe.ndjson`, `/tmp/facet-interrupt-probe.ndjson`,
`/tmp/facet-image-probe.ndjson` (not checked in; regenerate with the commands below).

Invocation under test:

```
claude -p --input-format stream-json --output-format stream-json --verbose \
  --session-id <uuid> --max-turns <n>
```

(Run with `CLAUDECODE` / `CLAUDE_CODE` unset so a nested Claude Code session
doesn't collide — matches the production env filtering.)

## GO decision — the load-bearing assumption HOLDS

A single process consumed TWO user frames piped to stdin and produced TWO
`result` events, both with the same `session_id` we passed. The process stayed
alive after the first `result` and processed the second frame. **The persistent
design is valid.**

Warm-cache payoff, straight from the `result.usage` fields:

| Turn | input_tokens | cache_creation | cache_read |
| ---- | ------------ | -------------- | ---------- |
| 1 (ALPHA) | 5840 | 3218 | 15551 |
| 2 (BRAVO) | **2** | 5994 | 18769 |

Turn 2 sent 2 fresh input tokens and read ~18.7k from cache — the cost win the
migration targets, confirmed.

## Input frame shape (stdin, newline-delimited, one object per line)

```json
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"..."}]}}
```

Images attach as base64 content blocks in the same `content` array (verified
`is_error:false`, model replied `SEEN`):

```json
{"type":"image","source":{"type":"base64","media_type":"image/png","data":"<base64-no-newlines>"}}
```

## Output event sequence (per the 2-turn probe)

```
system/hook_started
system/hook_response
system/init
system/thinking_tokens
assistant   content=thinking
assistant   content=text
rate_limit_event
result/success                 <- turn 1 ends
system/init                    <- turn 2 re-inits
assistant   content=text
result/success                 <- turn 2 ends
```

Top-level `type` values the reader must handle:
- `assistant` — `message.content[]` blocks of `type` `text`, `tool_use`, or
  `thinking`. Stream `text`; count `tool_use`; **ignore `thinking`** (no UI).
- `user` — tool results (drives the "thinking" indicator).
- `result` — turn end. Carries `session_id`, `is_error`, `result`, `subtype`,
  and rich `usage`. A new `result` per turn; the process continues.
- `system` — `subtype` of `init` (emitted at the start of EVERY turn, not just
  the first), `hook_started`, `hook_response`, `thinking_tokens`, or `error`.
  Only `error` is surfaced; the rest are silent by design.
- `rate_limit_event` — informational; ignored. The reader's `switch event["type"]` has no matching case and no `default`, so this type falls through silently (not logged).
- `error` — stream-level error; surface it.

## Interrupt: control frame IS honored → Task 8a

Writing this frame to stdin mid-turn stops the turn:

```json
{"type":"control_request","request_id":"int_1","request":{"subtype":"interrupt"}}
```

Observed: a `{"type":"control_response","response":{"subtype":"success","request_id":"int_1"}}`
acknowledgement, and the in-flight turn halted early (counted ~3 of 40).

**Critical caveat for the reader:** a self-initiated interrupt ends the turn
with a `result` whose `is_error:true` and `subtype:"error_during_execution"`.
When WE sent the interrupt, treat that result as a benign "stopped" (emit
`assistant:done`, not `assistant:error`). Track an "interrupt in flight" flag to
distinguish it from a genuine `error_during_execution`.

In the probe the process exited (code 1) after the interrupt because the pipe
closed (stdin EOF). In production stdin stays open, so the process should
survive for the next turn — but if it ever exits after an interrupt, the
crash-recovery path (lazy respawn with `--resume <session-id>`) recovers the
conversation transparently. Task 11 verifies process survival empirically.

## Reproduce

```bash
# Two-turn persistence + cache
printf '%s\n%s\n' \
  '{"type":"user","message":{"role":"user","content":[{"type":"text","text":"say the single word: ALPHA"}]}}' \
  '{"type":"user","message":{"role":"user","content":[{"type":"text","text":"say the single word: BRAVO"}]}}' \
  | env -u CLAUDECODE -u CLAUDE_CODE claude -p --input-format stream-json \
      --output-format stream-json --verbose \
      --session-id 11111111-2222-3333-4444-555555555555 --max-turns 2
```

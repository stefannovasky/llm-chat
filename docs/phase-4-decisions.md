# Phase 4 — Streaming: design decisions

## Architecture

1. **API shape**: `Stream(ctx, conv) (<-chan domain.StreamEvent, error)` — channel-based. `internal/llm` does not import `internal/ui`; both talk through `internal/domain`.
2. **StreamEvent**:
   ```go
   type StreamEvent struct {
       Delta string  // incremental text only (not accumulated)
       Usage *Usage  // set only on the final event before close
       Err   error   // set on error event before close
   }
   ```
   Channel close signals end of stream.
3. **UI consumption**: recursive `tea.Cmd`. Each `Cmd` reads one event from the channel and returns a `tea.Msg`; on receive, `Update` schedules the next read. On `!ok` from channel, emits `streamDoneMsg` and stops rescheduling.
4. **Channel buffer**: unbuffered. Follows Uber Go Style Guide ("Channel Size is One or None"). TCP provides natural backpressure; recursive Cmd reads immediately.

## Cancellation / Ctrl+C

5. **Mechanism**: store `context.CancelFunc` in the Bubble Tea Model. During streaming, Ctrl+C calls `cancel()`. Parser goroutine detects `ctx.Done()`, closes the channel.
6. **Double Ctrl+C**: no timer, no flag. During stream → cancel; outside stream → quit. "Second Ctrl+C exits" emerges naturally because cancel is synchronous at the UI level.
7. **Partial text on cancel**: preserved in both `m.messages` and `conversation.Messages` (keeps LLM context consistent for next turn). If zero tokens received before cancel, the assistant turn is dropped entirely (user turn stays).

## Rendering

8. **Streaming buffer**: dedicated field `m.streaming string` on the Model. Each delta appends to it. On done or cancel-with-text, promoted to `m.messages` + `conversation.Messages`, then cleared. Keeps `m.messages` as "finalized" only.
9. **Typing indicator**: existing spinner, repositioned — rendered after the partial text (`● <partial text> <spinner>`). Before first token arrives, only the spinner shows (as today).
10. **Input blocking**: only Enter is guarded while streaming (already the case). Textarea stays focused so user can pre-type the next message. Matches Phase 10.2 polish direction, avoids redoing work.
11. **Auto-scroll**: follow mode. Before updating viewport content on delta, check `viewport.AtBottom()`. If true, `GotoBottom()` after. If false, leave scroll position alone (user is reading history). Same rule at stream end.

## OpenRouter protocol

12. **Request**: `"stream": true` + `"stream_options": {"include_usage": true}` so final chunk carries usage.
13. **SSE edge cases**:
    - Lines starting with `:` (comments / heartbeats): ignore silently.
    - `data: [DONE]`: stop reading, close channel, return.
    - Chunks with `choices: []` and `usage` populated: emit `StreamEvent{Usage: &...}` (no delta).
    - Mid-stream `error` field: emit `StreamEvent{Err: ...}`, close. UI preserves partial text as assistant message **plus** adds a separate error line below.
    - Non-200 HTTP status on initial response: parse as `apiErrorBody`, return error from `Stream()` before returning channel. UI treats it like today.

## HTTP transport

14. **Timeouts**: remove `http.Client.Timeout` (total timeout kills long streams). Use custom `http.Transport` with:
    - `DialContext` timeout: 10s
    - `ResponseHeaderTimeout`: 30s
    - No total timeout; rely on `ctx` for cancellation.
15. **Path consolidation**: remove `Chat()` entirely. `Stream()` is the only method. UI uses only streaming message types.

## Usage capture

16. Behavior parity with Phase 3.5: usage always captured when available. If user cancels before the usage chunk arrives, `Usage` is nil on that turn — UI tolerates silently (important for Phase 6.4 `/cost`, which must handle missing data).

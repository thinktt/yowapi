# Engine Flow Follow-ups

The engine request path now publishes synchronously through `RequestMove`.
`PublishGameUpdates` returns an error when the request cannot be published, but
existing callers may continue to ignore that error for now.

## Deferred reliability work

- Record a durable pending engine request with the game state, preferably using
  a transactional outbox or a pending-request field on the game document.
- Give each request a deterministic key such as `gameID:moveIndex`.
- Retry pending requests after a yowapi restart instead of guessing from the
  NATS queue alone.
- Make worker responses idempotent so a response published before an ACK does
  not apply a move twice.
- Decide how normal move, draw, resign, and game-creation callers should expose
  request-publication failures.

These are follow-ups only. The current change intentionally does not add a
startup sweep, outbox, request IDs, or new retry behavior.

# ADR 007: Notification Delivery Guarantee — At-Least-Once

**Status:** Accepted
**Date:** 2025-04-12


## Context

The scheduler sends a release notification email and then updates `last_seen_tag` in the database. These are two separate side effects with no shared transaction boundary. A decision had to be made about the order of operations and the acceptable failure behavior.

## Options Considered

**At-least-once delivery (email first, DB update second)**

Pros:
- A missed release notification defeats the purpose of the service — this order prevents that
- Simple implementation with no additional infrastructure
- If the DB write fails, the next tick retries automatically

Cons:
- If the email succeeds but the DB write fails, the same notification is sent again on the next tick
- Duplicate emails are possible, though rare

**At-most-once delivery (DB update first, email second)**

Pros:
- No duplicate emails — once `last_seen_tag` is updated the notification is never re-sent

Cons:
- If the email fails after the DB update, the release is silently skipped and the user never gets notified
- A missed notification is a worse failure than a duplicate for this use case

**Exactly-once delivery (Transactional Outbox pattern)**

Pros:
- Guarantees each notification is delivered exactly once regardless of failure

Cons:
- Requires storing outbox records in the DB within the same transaction as the `last_seen_tag` update
- Requires a separate process to drain the outbox and send emails
- Significant added complexity not warranted for a single-instance service at this scope

## Decision

At-least-once delivery was chosen. The email is sent before `last_seen_tag` is updated.

## Consequences

- Duplicate notifications are possible if the DB write fails after a successful email send. This is intentional, not a bug.
- The tradeoff is explicit: a duplicate email is a lesser failure than a missed release notification, especially for security-related releases.
- If exactly-once delivery is ever required, the Transactional Outbox pattern is the correct upgrade path.

# ADR 006: Unsubscribe Action — Hard Delete

**Status:** Accepted
**Date:** 2025-04-12


## Context

The subscriptions table stores email addresses, which are personally identifiable information (PII). When a user unsubscribes, a decision had to be made about whether to permanently delete the record or retain it in a deactivated state.

## Options Considered

**Hard delete (`DELETE FROM subscriptions`)**

Pros:
- Complies with PII minimization principles — no personal data retained after the user opts out
- Simple implementation with no additional schema complexity
- Eliminates any risk of accidentally re-activating or exposing a deleted subscription

Cons:
- Irreversible — the record cannot be restored if deleted by mistake
- No audit trail of past subscriptions

**Soft delete (add `is_active` or `deleted_at` column)**

Pros:
- Reversible — subscription can be restored
- Enables admin dashboards and historical reporting

Cons:
- Retains email addresses after the user has explicitly opted out — violates PII minimization
- Requires every query to filter on `is_active = true` to avoid operating on deleted records
- Risk of future developers accidentally exposing or reactivating deleted subscriptions

## Decision

Hard delete was chosen.

## Consequences

- Unsubscribing is permanent. The user must re-subscribe and re-confirm if they change their mind.
- No audit trail of past subscriptions is kept. This is acceptable — there is no business requirement for it.
- Any future admin dashboard or reporting feature must not introduce soft deletes. If subscription history is ever required, it must be stored as a separate audit log that does not retain PII beyond its purpose.

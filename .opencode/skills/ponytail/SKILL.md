---
name: ponytail
description: Use when marking a deliberate simplification with a `ponytail:` comment that names the ceiling and upgrade path.
---

# ponytail

A `ponytail:` comment documents a deliberate YAGNI simplification: why this minimal code works now, what ceiling it hits, and what triggers an upgrade.

## When to Use

- You chose the simplest code that works and intentionally left out an abstraction, config, or dependency.
- You want future readers to know this was a choice, not an oversight.
- You need a clear signal for when to revisit (more users, more data, more complexity).

## Format

```ts
// ponytail: comment mode (10k comments/day). Upgrade: stream to file / batch writes when > 1M comments/day.
```

Structure:
- `ponytail:` label
- What was simplified and the current ceiling
- Upgrade path / trigger to revisit

## Rule

One `ponytail:` comment per deliberate simplification. No abstractions added "for later". If the ceiling is never reached, the comment stays as documentation of the choice.

## Example

```ts
// ponytail: in-memory comment cache keyed by postId. Ceiling: ~10k active posts, ~50MB RAM.
// Upgrade: switch to Redis when active posts exceed 10k or cache hit rate drops below 80%.
const cache = new Map<string, Comment[]>();
```

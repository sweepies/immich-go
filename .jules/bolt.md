# Bolt's Journal

## 2025-05-23 - [Metadata First: Deferring Expensive Operations]
**Learning:** In file synchronization tools, defer expensive operations (like file hashing) until after cheap heuristic checks (like filename/size/date metadata matching) have failed to provide a definitive answer.
**Action:** When designing "should I process this?" logic, always order checks from cheapest to most expensive. If a cheap check can confirm "already processed" or "skip", return early to avoid I/O.

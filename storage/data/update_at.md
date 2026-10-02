# User and item write metadata

`User.UpdateAt` and `Item.UpdateAt` record the latest database write, independently of the item's business `Timestamp`. Normal batch upserts overwrite any caller-supplied `UpdateAt` with the current UTC time. Nonempty partial modifications (including category changes) also refresh it. Empty patches remain no-ops.

Feedback insertion assigns metadata only when it creates a missing user or item; it must not change existing entity fields or metadata. MongoDB stores the field as `updateat`, following its existing lowercase BSON naming convention. SQL stores it as `update_at`. SQL writes use microsecond precision; MongoDB persists milliseconds. Readers should compare instants with the backend's precision rather than assuming exact equality with an application clock.

SQL initialization migrates existing tables idempotently. Existing rows receive the Unix epoch (`1970-01-01T00:00:00Z`) as an unknown-history sentinel, not an invented last-write time. Existing MongoDB documents and protobuf messages without metadata decode to the zero time until the next entity write.

Protobuf/proxy responses and binary dumps include `UpdateAt`. Restore preserves business timestamps but intentionally refreshes write metadata: restoring is a new database write. It does not replay the original database's write history. Older dumps remain readable without the added field.

ClickHouse implicit entity creation excludes already-present IDs because that backend has no `ON CONFLICT DO NOTHING`. Concurrent first creation retains the backend's existing nontransactional semantics. Full entity reads use `FINAL`, and latest-item reads use current base rows because ClickHouse materialized views do not reflect subsequent mutations. Initialization refreshes the latest-item view projection to include the new metadata field.

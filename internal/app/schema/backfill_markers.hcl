# The domains whose rows, written before the domain stored events, have been imported as events: one
# row per domain, written by eventstore.Backfill in the transaction that imports it. Part of the
# v0.2.0 backfill, and retired with it.
table "backfill_markers" {
  schema = schema.public
  column "domain" {
    null = false
    type = text
  }
  column "imported_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.domain]
  }
}

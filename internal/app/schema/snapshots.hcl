// The latest snapshot of a stream: the aggregate's whole state at stream_version, so a load
// replays only the events after it. A snapshot whose schema_version is not the aggregate's
// current one is ignored.
table "snapshots" {
  schema = schema.public
  column "stream_id" {
    null = false
    type = text
  }
  column "stream_version" {
    null = false
    type = integer
  }
  column "schema_version" {
    null = false
    type = integer
  }
  column "payload" {
    null = false
    type = jsonb
  }
  column "taken_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.stream_id]
  }
}

// The event store: every event every aggregate recorded, append-only. position orders the whole
// store; (stream_id, stream_version) orders one aggregate's stream and stops two writers from
// appending the same version.
table "events" {
  schema = schema.public
  column "position" {
    null = false
    type = bigserial
  }
  column "stream_id" {
    null = false
    type = text
  }
  column "stream_version" {
    null = false
    type = integer
  }
  column "type" {
    null = false
    type = text
  }
  column "schema_version" {
    null = false
    type = integer
  }
  column "payload" {
    null = false
    type = jsonb
  }
  column "metadata" {
    null    = false
    type    = jsonb
    default = sql("'{}'::jsonb")
  }
  column "recorded_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.position]
  }
  index "events_stream_id_stream_version_key" {
    unique  = true
    columns = [column.stream_id, column.stream_version]
  }
}

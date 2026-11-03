# How far each projector and event handler has read the event store, by name. Rows whose name
# starts with an underscore are markers, not checkpoints: _backfill/<domain> records that a
# domain's rows have been imported as events.
table "projection_checkpoints" {
  schema = schema.public
  column "name" {
    null = false
    type = text
  }
  column "position" {
    null    = false
    type    = bigint
    default = 0
  }
  column "updated_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.name]
  }
}

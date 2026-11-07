# How far each projector and event handler has read the event store, by name.
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

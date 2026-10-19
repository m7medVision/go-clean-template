table "queue_messages" {
  schema = schema.public
  column "id" {
    null = false
    type = bigserial
  }
  column "queue" {
    null = false
    type = varchar(255)
  }
  column "body" {
    null = false
    type = jsonb
  }
  column "read_count" {
    null    = false
    type    = integer
    default = 0
  }
  column "enqueued_at" {
    null    = false
    type    = timestamp
    default = sql("now()")
  }
  column "visible_at" {
    null    = false
    type    = timestamp
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_queue_messages_queue_visible_at" {
    on {
      column = column.queue
    }
    on {
      column = column.visible_at
    }
  }
}

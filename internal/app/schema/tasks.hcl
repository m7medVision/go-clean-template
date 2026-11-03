# The task read model: written only by the task projector, from the task events. It has no foreign
# key to users, because each projector advances on its own and the task one can be ahead.
table "tasks" {
  schema = schema.public
  column "id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "title" {
    null = false
    type = varchar(255)
  }
  column "description" {
    null    = true
    type    = text
    default = ""
  }
  column "status" {
    null    = false
    type    = varchar(20)
    default = "todo"
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("now()")
  }
  column "updated_at" {
    null    = false
    type    = timestamp
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_tasks_user_id" {
    on {
      column = column.user_id
    }
  }
  index "idx_tasks_user_status" {
    on {
      column = column.user_id
    }
    on {
      column = column.status
    }
  }
}

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
  foreign_key "tasks_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [column.id]
    ref_table   = table.users
    on_delete   = CASCADE
  }
}

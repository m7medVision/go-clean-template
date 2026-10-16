// Desired database schema, applied by Atlas (declarative workflow).
// Edit this file to change the schema; Atlas computes and applies the diff.
schema "public" {
}

table "users" {
  schema = schema.public
  column "id" {
    null = false
    type = uuid
  }
  column "username" {
    null = false
    type = varchar(255)
  }
  column "email" {
    null = false
    type = varchar(255)
  }
  column "password_hash" {
    null = false
    type = varchar(255)
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
  index "users_username_key" {
    unique  = true
    columns = [column.username]
  }
  index "users_email_key" {
    unique  = true
    columns = [column.email]
  }
}

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

table "history" {
  schema = schema.public
  column "id" {
    null = false
    type = serial
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "source" {
    null = true
    type = varchar(255)
  }
  column "destination" {
    null = true
    type = varchar(255)
  }
  column "original" {
    null = true
    type = varchar(255)
  }
  column "translation" {
    null = true
    type = varchar(255)
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_history_user_id" {
    on {
      column = column.user_id
    }
  }
  foreign_key "history_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [column.id]
    ref_table   = table.users
    on_delete   = CASCADE
  }
}

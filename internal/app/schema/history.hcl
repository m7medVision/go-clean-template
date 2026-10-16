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

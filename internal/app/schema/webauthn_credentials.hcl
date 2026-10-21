table "webauthn_credentials" {
  schema = schema.public
  column "id" {
    null = false
    type = bytea
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "name" {
    null = false
    type = text
  }
  column "credential" {
    null = false
    type = jsonb
  }
  column "created_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  column "last_used_at" {
    null = true
    type = timestamptz
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_webauthn_credentials_user_id" {
    on {
      column = column.user_id
    }
  }
  foreign_key "webauthn_credentials_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }
}

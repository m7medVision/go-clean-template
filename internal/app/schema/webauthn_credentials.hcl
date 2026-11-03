# The passkey read model: written only by the passkey projector, so it has no foreign key to users.
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
}

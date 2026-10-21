# An identity at an OpenID Connect provider, linked to the local user it signs in as. A user is
# matched by (issuer, subject) only, never by email: the provider's email is not proof of the account.
table "user_identities" {
  schema = schema.public
  column "issuer" {
    null = false
    type = text
  }
  column "subject" {
    null = false
    type = text
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "created_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.issuer, column.subject]
  }
  index "idx_user_identities_user_id" {
    on {
      column = column.user_id
    }
  }
  foreign_key "user_identities_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }
}

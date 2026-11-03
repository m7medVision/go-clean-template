# An identity at an OpenID Connect provider, linked to the local user it signs in as. A user is
# matched by (issuer, subject) only, never by email: the provider's email is not proof of the account.
# Part of the user read model: written only by the user projector, so it has no foreign key to users.
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
}

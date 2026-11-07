# The user read model: written only by the user projector. Usernames and emails are unique by their
# claims, which commit with the events; the table repeats no rule, so it can store whatever was
# committed.
table "users" {
  schema = schema.public
  column "id" {
    null = false
    type = uuid
  }
  column "username" {
    null = false
    type = text
  }
  column "email" {
    null = false
    type = text
  }
  column "password_hash" {
    null = false
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  column "updated_at" {
    null    = false
    type    = timestamptz
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_users_email" {
    columns = [column.email]
  }
}

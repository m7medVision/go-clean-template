// Values that must be unique across aggregates, such as a user's email. A command reserves its
// claims in the same transaction that appends its events, so two registrations of one email
// cannot both succeed.
table "claims" {
  schema = schema.public
  column "scope" {
    null = false
    type = text
  }
  column "value" {
    null = false
    type = text
  }
  column "owner_id" {
    null = false
    type = text
  }
  primary_key {
    columns = [column.scope, column.value]
  }
}

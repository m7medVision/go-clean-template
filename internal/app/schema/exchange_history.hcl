table "exchange_history" {
  schema = schema.public
  column "id" {
    null = false
    type = bigserial
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "from_currency" {
    null = false
    type = char(3)
  }
  column "to_currency" {
    null = false
    type = char(3)
  }
  column "amount" {
    null = false
    type = numeric
  }
  column "rate" {
    null = false
    type = numeric
  }
  column "result" {
    null = false
    type = numeric
  }
  column "rate_date" {
    null = false
    type = date
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_exchange_history_user_id" {
    on {
      column = column.user_id
    }
  }
  foreign_key "exchange_history_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_delete   = CASCADE
  }
}

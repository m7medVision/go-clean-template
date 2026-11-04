# The exchange read model: written only by the exchange projector, so it has no foreign key to
# users. conversion_id is the conversion's stream; rows stored before conversions stored events get
# theirs from the backfill.
table "exchange_history" {
  schema = schema.public
  column "id" {
    null = false
    type = bigserial
  }
  column "conversion_id" {
    null = true
    type = uuid
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
  index "exchange_history_conversion_id_key" {
    unique  = true
    columns = [column.conversion_id]
  }
  index "idx_exchange_history_user_id" {
    on {
      column = column.user_id
    }
  }
}

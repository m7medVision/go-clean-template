table "webauthn_used_challenges" {
  schema = schema.public
  column "challenge_hash" {
    null = false
    type = bytea
  }
  column "expires_at" {
    null = false
    type = timestamptz
  }
  primary_key {
    columns = [column.challenge_hash]
  }
  index "idx_webauthn_used_challenges_expires_at" {
    on {
      column = column.expires_at
    }
  }
}

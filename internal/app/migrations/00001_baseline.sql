-- +goose Up
CREATE TABLE "backfill_markers" (
  "domain" text NOT NULL,
  "imported_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("domain")
);
CREATE TABLE "claims" (
  "scope" text NOT NULL,
  "value" text NOT NULL,
  "owner_id" text NOT NULL,
  PRIMARY KEY ("scope", "value")
);
CREATE TABLE "events" (
  "position" bigserial NOT NULL,
  "stream_id" text NOT NULL,
  "stream_version" integer NOT NULL,
  "type" text NOT NULL,
  "schema_version" integer NOT NULL,
  "payload" jsonb NOT NULL,
  "metadata" jsonb NOT NULL DEFAULT '{}',
  "recorded_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("position")
);
CREATE UNIQUE INDEX "events_stream_id_stream_version_key" ON "events" ("stream_id", "stream_version");
CREATE TABLE "exchange_history" (
  "id" bigserial NOT NULL,
  "conversion_id" uuid NULL,
  "user_id" uuid NOT NULL,
  "from_currency" character(3) NOT NULL,
  "to_currency" character(3) NOT NULL,
  "amount" numeric NOT NULL,
  "rate" numeric NOT NULL,
  "result" numeric NOT NULL,
  "rate_date" date NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "exchange_history_conversion_id_key" ON "exchange_history" ("conversion_id");
CREATE INDEX "idx_exchange_history_user_id" ON "exchange_history" ("user_id", "created_at");
CREATE TABLE "projection_checkpoints" (
  "name" text NOT NULL,
  "position" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("name")
);
CREATE TABLE "queue_messages" (
  "id" bigserial NOT NULL,
  "queue" text NOT NULL,
  "body" jsonb NOT NULL,
  "read_count" integer NOT NULL DEFAULT 0,
  "enqueued_at" timestamptz NOT NULL DEFAULT now(),
  "visible_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_queue_messages_queue_visible_at" ON "queue_messages" ("queue", "visible_at");
CREATE TABLE "snapshots" (
  "stream_id" text NOT NULL,
  "stream_version" integer NOT NULL,
  "schema_version" integer NOT NULL,
  "payload" jsonb NOT NULL,
  "taken_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("stream_id")
);
CREATE TABLE "tasks" (
  "id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  "title" text NOT NULL,
  "description" text NULL DEFAULT '',
  "status" character varying(20) NOT NULL DEFAULT 'todo',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_tasks_user_id" ON "tasks" ("user_id");
CREATE INDEX "idx_tasks_user_status" ON "tasks" ("user_id", "status");
CREATE TABLE "user_identities" (
  "issuer" text NOT NULL,
  "subject" text NOT NULL,
  "user_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("issuer", "subject")
);
CREATE INDEX "idx_user_identities_user_id" ON "user_identities" ("user_id");
CREATE TABLE "users" (
  "id" uuid NOT NULL,
  "username" text NOT NULL,
  "email" text NOT NULL,
  "password_hash" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_users_email" ON "users" ("email");
CREATE TABLE "webauthn_credentials" (
  "id" bytea NOT NULL,
  "user_id" uuid NOT NULL,
  "name" text NOT NULL,
  "credential" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "last_used_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
CREATE INDEX "idx_webauthn_credentials_user_id" ON "webauthn_credentials" ("user_id");
CREATE TABLE "webauthn_used_challenges" (
  "challenge_hash" bytea NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("challenge_hash")
);
CREATE INDEX "idx_webauthn_used_challenges_expires_at" ON "webauthn_used_challenges" ("expires_at");

-- +goose Down
DROP TABLE "webauthn_used_challenges";
DROP TABLE "webauthn_credentials";
DROP TABLE "users";
DROP TABLE "user_identities";
DROP TABLE "tasks";
DROP TABLE "snapshots";
DROP TABLE "queue_messages";
DROP TABLE "projection_checkpoints";
DROP TABLE "exchange_history";
DROP TABLE "events";
DROP TABLE "claims";
DROP TABLE "backfill_markers";

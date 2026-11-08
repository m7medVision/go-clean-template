-- +goose Up
DROP TABLE "queue_messages";

-- +goose Down
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

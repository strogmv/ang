-- Create "idempotency_keys" table
CREATE TABLE "public"."idempotency_keys" (
  "key" text NOT NULL,
  "response_payload" jsonb NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NULL,
  PRIMARY KEY ("key")
);
-- Create "outbox_events" table
CREATE TABLE "public"."outbox_events" (
  "id" uuid NOT NULL,
  "event_type" text NOT NULL,
  "payload" jsonb NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "processed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_outbox_unprocessed" to table: "outbox_events"
CREATE INDEX "idx_outbox_unprocessed" ON "public"."outbox_events" ("created_at") WHERE (processed_at IS NULL);

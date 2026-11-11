-- 0001_init.sql — Groupify initial schema
--
-- Design notes
--   * `topics.current_members` is a maintained counter, not a derived count.
--     It is only ever mutated inside a transaction that also mutates the
--     matching `registrations` row, so the two cannot drift. The CHECK
--     constraint is the final line of defence against overbooking.
--   * `registrations.index_number` stores the *normalised* value (trimmed,
--     internal whitespace collapsed, upper-cased). The UNIQUE constraint on it
--     is what makes "one registration per student" true under concurrency.
--   * `app_settings` is a singleton row. Registration transactions take a
--     FOR SHARE lock on it; open/close takes FOR UPDATE. That pair defines the
--     boundary: a registration that committed before closure stands, and no
--     registration can commit after a closure has taken effect.
--
-- The migration runner wraps each file in its own transaction; do not add
-- BEGIN/COMMIT here.

CREATE TABLE topics (
    id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            text        NOT NULL,
    description     text        NOT NULL DEFAULT '',
    capacity        integer     NOT NULL,
    current_members integer     NOT NULL DEFAULT 0,
    is_active       boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT topics_name_not_blank        CHECK (btrim(name) <> ''),
    CONSTRAINT topics_capacity_positive     CHECK (capacity > 0),
    CONSTRAINT topics_members_non_negative  CHECK (current_members >= 0),
    CONSTRAINT topics_members_within_capacity CHECK (current_members <= capacity)
);

-- Topic names are unique case-insensitively; duplicate topics are an admin
-- mistake, not a feature.
CREATE UNIQUE INDEX topics_name_unique_idx ON topics (lower(btrim(name)));

CREATE TABLE registrations (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    full_name    text        NOT NULL,
    index_number text        NOT NULL,
    topic_id     bigint      NOT NULL REFERENCES topics (id) ON DELETE RESTRICT,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT registrations_full_name_not_blank    CHECK (btrim(full_name) <> ''),
    CONSTRAINT registrations_index_number_not_blank CHECK (btrim(index_number) <> ''),
    CONSTRAINT registrations_index_number_unique    UNIQUE (index_number)
);

CREATE INDEX registrations_topic_id_idx   ON registrations (topic_id);
CREATE INDEX registrations_created_at_idx ON registrations (created_at DESC, id DESC);

CREATE TABLE admins (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT admins_email_not_blank CHECK (btrim(email) <> '')
);

CREATE UNIQUE INDEX admins_email_unique_idx ON admins (lower(email));

-- Opaque session tokens. Only the SHA-256 of the token is stored, so a
-- database leak does not hand over live sessions.
CREATE TABLE admin_sessions (
    token_hash   bytea       PRIMARY KEY,
    admin_id     bigint      NOT NULL REFERENCES admins (id) ON DELETE CASCADE,
    csrf_token   text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);

CREATE INDEX admin_sessions_admin_id_idx    ON admin_sessions (admin_id);
CREATE INDEX admin_sessions_expires_at_idx  ON admin_sessions (expires_at);

CREATE TABLE app_settings (
    id                   smallint    PRIMARY KEY DEFAULT 1,
    registration_open    boolean     NOT NULL DEFAULT false,
    updated_at           timestamptz NOT NULL DEFAULT now(),
    updated_by_admin_id  bigint      REFERENCES admins (id) ON DELETE SET NULL,

    CONSTRAINT app_settings_singleton CHECK (id = 1)
);

-- Safe default: a fresh install accepts no registrations until an admin opens
-- the period deliberately.
INSERT INTO app_settings (id, registration_open) VALUES (1, false);

CREATE TABLE audit_logs (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_admin_id bigint      REFERENCES admins (id) ON DELETE SET NULL,
    -- Free-form actor label for events without an admin (e.g. 'system',
    -- 'student'). Kept separate from the FK so history survives admin deletion.
    actor_label    text        NOT NULL DEFAULT '',
    action         text        NOT NULL,
    entity_type    text        NOT NULL,
    entity_id      text        NOT NULL DEFAULT '',
    before_data    jsonb,
    after_data     jsonb,
    request_id     text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at DESC, id DESC);
CREATE INDEX audit_logs_action_idx     ON audit_logs (action);
CREATE INDEX audit_logs_entity_idx     ON audit_logs (entity_type, entity_id);

-- Idempotency claims for POST /api/registrations. The row is inserted (claimed)
-- before the registration work and completed before COMMIT, so a concurrent
-- duplicate blocks on the uncommitted row instead of double-registering.
CREATE TABLE idempotency_keys (
    key             text        PRIMARY KEY,
    request_hash    text        NOT NULL,
    response_status integer,
    response_body   jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);

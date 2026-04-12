-- +goose Up

CREATE TABLE users_new (
    id         BIGSERIAL PRIMARY KEY,
    units      TEXT NOT NULL DEFAULT 'mmol',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO users_new (id, units, created_at)
SELECT id, units, created_at FROM users;

CREATE TABLE external_accounts (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    provider     TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    display_name TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, external_id)
);

INSERT INTO external_accounts (user_id, provider, external_id, display_name)
SELECT id, 'telegram', id::TEXT, first_name FROM users;

ALTER TABLE glucose_readings DROP CONSTRAINT glucose_readings_user_id_fkey;

DROP TABLE users;

ALTER TABLE users_new RENAME TO users;

ALTER TABLE glucose_readings
    ADD CONSTRAINT glucose_readings_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE external_accounts
    ADD CONSTRAINT external_accounts_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

SELECT setval(pg_get_serial_sequence('users', 'id'), GREATEST(COALESCE((SELECT MAX(id) FROM users), 0), 1));

-- +goose Down

CREATE TABLE users_old (
    id         BIGINT PRIMARY KEY,
    username   TEXT,
    first_name TEXT,
    units      TEXT NOT NULL DEFAULT 'mmol',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO users_old (id, username, first_name, units, created_at)
SELECT u.id, '', ea.display_name, u.units, u.created_at
FROM users u
LEFT JOIN external_accounts ea ON ea.user_id = u.id AND ea.provider = 'telegram';

ALTER TABLE glucose_readings DROP CONSTRAINT glucose_readings_user_id_fkey;

ALTER TABLE external_accounts DROP CONSTRAINT external_accounts_user_id_fkey;

DROP TABLE external_accounts;

DROP TABLE users;

ALTER TABLE users_old RENAME TO users;

ALTER TABLE glucose_readings
    ADD CONSTRAINT glucose_readings_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

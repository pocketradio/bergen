CREATE TABLE bergen.users (
    id text PRIMARY KEY CHECK (id <> ''),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bergen.client_credentials (
    id text PRIMARY KEY CHECK (id <> ''),
    user_id text NOT NULL REFERENCES bergen.users(id),
    credential_digest bytea NOT NULL UNIQUE CHECK (octet_length(credential_digest) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE INDEX client_credentials_user_id_idx ON bergen.client_credentials(user_id);

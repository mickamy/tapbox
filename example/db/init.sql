CREATE TABLE IF NOT EXISTS notes
(
    id         BIGSERIAL PRIMARY KEY,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO notes (title, body)
VALUES ('Welcome to tapbox', 'This is a sample note created by the seed script.'),
       ('gRPC tracing', 'tapbox captures gRPC calls transparently.'),
       ('SQL tracing', 'PostgreSQL queries appear as child spans.');

CREATE TABLE collection
(
    name text NOT NULL,
    path text NOT NULL
);

CREATE TABLE download
(
    url       text NOT NULL,
    source_id text,
    state     text
)

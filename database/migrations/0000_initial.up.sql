CREATE TABLE collection
(
    name text NOT NULL,
    path text NOT NULL,
    UNIQUE(name)
);

CREATE TABLE download
(
    collection_id int NOT NULL,
    url       text NOT NULL,
    state     text NOT NULL DEFAULT 'new',
    added     timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (collection_id) REFERENCES collection(rowid)
)

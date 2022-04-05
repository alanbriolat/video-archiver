CREATE TABLE collection
(
    id integer PRIMARY KEY,
    name text NOT NULL,
    path text NOT NULL,
    UNIQUE(name)
);

CREATE TABLE download
(
    id integer PRIMARY KEY,
    collection_id int NOT NULL,
    url       text NOT NULL,
    added     timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    state     text NOT NULL DEFAULT 'new',
    FOREIGN KEY (collection_id) REFERENCES collection(id)
);

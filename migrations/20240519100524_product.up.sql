CREATE TABLE IF NOT EXISTS ventrata.products (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    capacity integer
);

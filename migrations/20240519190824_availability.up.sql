CREATE TABLE IF NOT EXISTS ventrata.availability (
    id uuid PRIMARY KEY,
    product_id uuid REFERENCES products(id),
    date timestamptz DEFAULT now()
);

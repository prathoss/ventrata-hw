CREATE TABLE IF NOT EXISTS ventrata.pricing (
    product_id uuid REFERENCES products(id),
    currency char(3) NOT NULL,
    price integer,
    CONSTRAINT pricing_pk PRIMARY KEY (product_id, currency)
)

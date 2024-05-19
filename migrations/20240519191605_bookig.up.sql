CREATE TABLE IF NOT EXISTS ventrata.bookings (
    id uuid PRIMARY KEY,
    availability_id uuid REFERENCES availability(id),
    confirmed boolean
);

CREATE TABLE IF NOT EXISTS ventrata.tickets (
    id uuid PRIMARY KEY,
    booking_id uuid REFERENCES bookings(id),
    content text
);

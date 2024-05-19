CREATE USER ventrata_usr WITH PASSWORD 'ventrata123';

CREATE DATABASE ventrata OWNER ventrata_usr;
\c ventrata;

CREATE SCHEMA ventrata;
GRANT ALL ON SCHEMA ventrata TO ventrata_usr;

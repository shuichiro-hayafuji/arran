#!/bin/sh
set -eu
# Only runs against a new PostgreSQL volume. Passwords are read from files,
# never command-line arguments, logs, compose interpolation or user-data.
migration_password=$(cat /run/secrets/migration_password)
runtime_password=$(cat /run/secrets/runtime_password)
export migration_password runtime_password
psql -v ON_ERROR_STOP=1 --username postgres --dbname postgres <<'SQL'
\getenv migration_password migration_password
\getenv runtime_password runtime_password
CREATE ROLE arran_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'migration_password';
CREATE ROLE arran_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'runtime_password';
CREATE DATABASE arran OWNER arran_migrator;
REVOKE ALL ON DATABASE arran FROM PUBLIC;
GRANT CONNECT ON DATABASE arran TO arran_runtime;
\connect arran
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO arran_runtime;
SQL

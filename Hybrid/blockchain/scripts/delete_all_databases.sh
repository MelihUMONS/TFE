#!/bin/bash

export PGPASSWORD="postgres"

DATABASES=$(psql -h 127.0.0.1 -p 5112 -U postgres -t -c "SELECT datname FROM pg_database WHERE datistemplate = false AND datname != 'postgres';" AND datname != 'test')

for DB in $DATABASES; do
    echo "Dropping database: $DB"
    psql -h 172.17.0.1 -p 5112 -U postgres -c "DROP DATABASE \"$DB\";"
done

unset PGPASSWORD
echo "All non-system databases have been dropped."

 psql "host=127.0.0.1 port=5112 user=postgres password=postgres sslmode=disable" -c "create database hybrid;"


#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"


for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"

  # Convertir la table en hypertable
  $PSQL "SELECT create_hypertable('$TABLE_NAME', 'time');"
done
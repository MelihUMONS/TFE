#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"

# Boucle pour supprimer les tables
for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"
  
  # Supprimer la table
  $PSQL "DROP TABLE IF EXISTS $TABLE_NAME CASCADE;"
  $PSQL "DROP TABLE IF EXISTS hypertable_$TABLE_NAME CASCADE;"
done

echo "Suppression de $TABLE_COUNT tables terminée."

#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -c"

#init 
$PSQL "CREATE DATABASE $DB_NAME;"
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"

# Boucle pour créer les tables
for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"
  
  # Créer la table
  $PSQL "CREATE TABLE $TABLE_NAME (
    time TIMESTAMPTZ NOT NULL,
    data DOUBLE PRECISION NOT NULL
  );"
done

echo "Création de $TABLE_COUNT tables terminée."

#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"


# Créer les hypertables et ajouter des index
for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"
  HYPERTABLE_NAME="hypertable_${TABLE_NAME}"
  
  # Créer une copie de la table de capteur
  $PSQL "CREATE TABLE $HYPERTABLE_NAME (LIKE $TABLE_NAME INCLUDING ALL);"
  
  # Convertir la copie en hypertable
  $PSQL "SELECT create_hypertable('$HYPERTABLE_NAME', 'time');"
  
  # Insérer les données de la table de capteur dans l'hypertable
  $PSQL "INSERT INTO $HYPERTABLE_NAME SELECT * FROM $TABLE_NAME;"
  
  # Ajouter un index sur la colonne time
  $PSQL "CREATE INDEX ON $HYPERTABLE_NAME (time);"
done
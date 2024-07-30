#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5
SQL_FILE="./brouillon/insertData.sql"

# Commande pour accéder à PostgreSQL
PSQL="docker exec -i timescale-pg-11 psql -U $DB_USER -d $DB_NAME"

# Boucle pour chaque table de capteur
for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"
  
  # Remplacer [id] par le numéro du capteur et exécuter la commande SQL
  sed "s/\[id\]/${i}/g" $SQL_FILE | $PSQL
  echo "Données insérées dans la table $TABLE_NAME."
done

echo "Insertion des données terminée."

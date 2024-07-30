#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5
START_DATE="2023-01-01 00:00:00"
END_DATE="2023-01-31 23:59:59"
CURRENT_DATE=$START_DATE
SECONDS_ELAPSED=0
TOTAL_SECONDS=$(( $(date -d "$END_DATE" +%s) - $(date -d "$START_DATE" +%s) + 1 ))

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"

# Fonction pour générer une valeur aléatoire
generate_random_value() {
  echo "scale=2; $RANDOM/32767" | bc
}

# Boucle pour insérer les données
while [ "$CURRENT_DATE" != "$END_DATE" ]; do
  for i in $(seq 1 $TABLE_COUNT)
  do
    TABLE_NAME="${TABLE_PREFIX}${i}"
    DATA=$(generate_random_value)
    
    # Insérer la donnée
    $PSQL "INSERT INTO $TABLE_NAME (time, data) VALUES ('$CURRENT_DATE', $DATA);"
  done

  # Incrementer la date d'une seconde
  CURRENT_DATE=$(date -d "$CURRENT_DATE + 1 second" +"%Y-%m-%d %H:%M:%S")

   # Incrementer le compteur
  SECONDS_ELAPSED=$((SECONDS_ELAPSED + 1))

  # Afficher la progression toutes les 1000 secondes
  if (( SECONDS_ELAPSED % 1000 == 0 )); then
    echo "Progression : $SECONDS_ELAPSED / $TOTAL_SECONDS secondes traitées."
  fi

done

echo "Insertion des données terminée."

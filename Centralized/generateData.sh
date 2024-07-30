#!/bin/bash

# Variables
DB_NAME="test"
TABLE_PREFIX="sensor"
START_DATE="2023-01-01 00:00:00"
END_DATE="2023-01-31 23:59:59"
CURRENT_DATE=$START_DATE
OUTPUT_FILE="insertData.sql"

# Fonction pour générer une valeur aléatoire
generate_random_value() {
  echo "scale=2; $RANDOM/32767" | bc
}

# Initialiser le fichier de sortie
echo "-- Script pour insérer des données dans les tables de capteurs" > $OUTPUT_FILE
echo "-- Base de données : $DB_NAME" >> $OUTPUT_FILE
echo "-- Début : $START_DATE" >> $OUTPUT_FILE
echo "-- Fin : $END_DATE" >> $OUTPUT_FILE
echo "" >> $OUTPUT_FILE
echo "INSERT INTO ${TABLE_PREFIX}[id] (time, data) VALUES" >> $OUTPUT_FILE

# Calculer le nombre total de secondes
TOTAL_SECONDS=$(( $(date -d "$END_DATE" +%s) - $(date -d "$START_DATE" +%s) + 1 ))
SECONDS_ELAPSED=0

# Boucle pour générer les données
while [ "$CURRENT_DATE" != "$END_DATE" ]; do
  DATA=$(generate_random_value)
  
  # Ajouter l'insertion au fichier
  echo "('$CURRENT_DATE', $DATA)," >> $OUTPUT_FILE

  # Incrementer la date d'une seconde
  CURRENT_DATE=$(date -d "$CURRENT_DATE + 1 second" +"%Y-%m-%d %H:%M:%S")

  # Incrementer le compteur
  SECONDS_ELAPSED=$((SECONDS_ELAPSED + 1))

  # Afficher la progression toutes les 1000 secondes
  if (( SECONDS_ELAPSED % 1000 == 0 )); then
    echo "Progression : $SECONDS_ELAPSED / $TOTAL_SECONDS secondes traitées."
  fi
done

# Supprimer la dernière virgule et ajouter un point-virgule
sed -i '$ s/,$/;/' $OUTPUT_FILE

# Afficher la progression finale
echo "Progression : $SECONDS_ELAPSED / $TOTAL_SECONDS secondes traitées."
echo "Génération du fichier $OUTPUT_FILE terminée."

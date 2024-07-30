#!/bin/bash

# Variables
DB_NAME="test"
DB_USER="postgres"
TABLE_PREFIX="sensor"
TABLE_COUNT=5
QUERY_OUTPUT_FILE="query_times.txt"

# Commande pour accéder à PostgreSQL
PSQL="docker exec -it timescale-pg-11 psql -U $DB_USER -d $DB_NAME -c"

# Fonction pour mesurer le temps d'exécution d'une requête
measure_query_time() {
  local table_name=$1
  local query="SELECT time_bucket('1 day', time) AS day, avg(data) FROM $table_name WHERE time >= '2023-01-01' AND time < '2023-02-01' GROUP BY day;"
  local start_time=$(date +%s%N)
  $PSQL "$query" > /dev/null
  local end_time=$(date +%s%N)
  local elapsed_time=$(( ($end_time - $start_time) / 1000000 )) # Convert to milliseconds
  echo "$elapsed_time"
}

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

# Initialiser le fichier de sortie
echo "Table,Time (ms)" > $QUERY_OUTPUT_FILE

# Mesurer le temps des requêtes sur les tables normales
for i in $(seq 1 $TABLE_COUNT)
do
  TABLE_NAME="${TABLE_PREFIX}${i}"
  $PSQL "CREATE INDEX ON $TABLE_NAME (time);"
  query_time=$(measure_query_time $TABLE_NAME)
  echo "$TABLE_NAME,$query_time" >> $QUERY_OUTPUT_FILE
  echo "Temps pour la table $TABLE_NAME (normale) : $query_time ms"
done

# Mesurer le temps des requêtes sur les hypertables
for i in $(seq 1 $TABLE_COUNT)
do
  HYPERTABLE_NAME="hypertable_${TABLE_PREFIX}${i}"
  query_time=$(measure_query_time $HYPERTABLE_NAME)
  echo "$HYPERTABLE_NAME,$query_time" >> $QUERY_OUTPUT_FILE
  echo "Temps pour la table $HYPERTABLE_NAME (hypertable) : $query_time ms"
done

echo "Les temps de requête ont été enregistrés dans $QUERY_OUTPUT_FILE."

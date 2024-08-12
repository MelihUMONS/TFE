#!/bin/bash

# Définir les dates de début et de fin
start_date="2023-01-01 00:00:00"
end_date="2023-01-08 23:59:59"

# Convertir les dates en secondes depuis Epoch
start_ts=$(date -d "$start_date" +%s)
end_ts=$(date -d "$end_date" +%s)

# Initialiser la liste pour stocker les données
data_list="["

# Nombre total de secondes pour le calcul de la progression
total_seconds=$((end_ts - start_ts + 1))
progress_interval=$((total_seconds / 100))  # Intervalle de progression (1% du total)

count=0
for current_ts in $(seq $start_ts $end_ts); do
    # Générer une donnée aléatoire
    data_value=$(awk -v min=0 -v max=1000 'BEGIN{srand(); printf "%.2f", min+rand()*(max-min+1)}')
    
    # Convertir le timestamp en date
    current_date=$(date -d @$current_ts +"%Y-%m-%d %H:%M:%S")
    
    # Ajouter l'entrée à la liste de données avec le format souhaité
    data_list+="{\\\"date\\\": \\\"$current_date\\\", \\\"data\\\": \\\"$data_value\\\"}, "
    

    # Afficher la progression chaque minute
    if [ $((count % 3600)) -eq 0 ]; then
        echo "Progression : $((count / 3600)) minutes traitées"
    fi

    # Afficher un message de progression tous les 1% du total
    if [ $((count % progress_interval)) -eq 0 ]; then
        echo "Progression : $((count * 100 / total_seconds))%"
    fi

    count=$((count + 1))
done

# Retirer la dernière virgule et espace, et fermer la liste
data_list="${data_list%, }]"
data_list=$(echo $data_list | sed 's/, \]/]/g')

# Écrire les données dans un fichier
echo $data_list > data.json

echo "Le fichier data.json a été généré avec succès."

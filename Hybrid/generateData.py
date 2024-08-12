import json
from datetime import datetime, timedelta
import random

# Définir les dates de début et de fin
start_date = datetime(2023, 1, 1)
end_date = datetime(2023, 1, 8, 23, 59, 59)  # Inclure la fin de la journée du 8 janvier

# Initialiser la liste pour stocker les données
data_list = []

# Boucle pour générer les données à une fréquence d'une seconde
current_date = start_date
while current_date <= end_date:
    data_entry = {
        "date": current_date.strftime("%Y-%m-%d %H:%M:%S"),
        "data": f"{random.uniform(0, 1000):.2f}"  # Générer une donnée aléatoire
    }
    data_list.append(data_entry)
    current_date += timedelta(seconds=1)

# Convertir la liste en format JSON
data_json = json.dumps(data_list)

# Écrire les données dans un fichier
with open('data.json', 'w') as file:
    file.write(data_json)

print("Le fichier data.json a été généré avec succès.")


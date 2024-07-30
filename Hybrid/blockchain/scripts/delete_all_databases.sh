#!/bin/bash

export MYSQL_PWD="rootpassword0"

DATABASES=$(mysql -h 172.17.0.1 -P 3306 -u root -e "show databases"  | grep -vE "Database|information_schema|mysql|performance_schema|sys")


for DB in $DATABASES; do
    echo "Dropping database: $DB"
    mysql  -h 172.17.0.1 -P 3306 -u root -e "DROP DATABASE \`$DB\`;"
done

unset MYSQL_PWD
echo "All non-system databases have been dropped."

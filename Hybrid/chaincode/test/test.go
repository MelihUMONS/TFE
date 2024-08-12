package main

import (
	"crypto/aes"
	"crypto/cipher"
	//"crypto/sha256"
	"database/sql"
	"encoding/hex"
	//"encoding/json"
	"fmt"
	"log"
	//"math/big"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type Sensor struct {
	SensorID  string `json:"sensorId"`
	SharedKey string `json:"sharedKey"`
}

type Data struct {
	Date string `json:"date"`
	Data string `json:"data"`
}

func connectToPostgres(dbName string) (*sql.DB, error) {
	var connStr string
	if dbName == "" {
		connStr = "host=127.0.0.1 port=5112 user=postgres password=postgres sslmode=disable"
	} else {
		connStr = fmt.Sprintf("host=127.0.0.1 port=5112 user=postgres password=postgres dbname=%s sslmode=disable", dbName)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
	}

	fmt.Println("Successfully connected to TimescaleDB!")

	return db, nil
}

func runQuery(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	return db.Query(query, args...)
}

func createFolderIfNotExists(folderPath string) error {
	return os.MkdirAll(folderPath, os.ModePerm)
}

func appendToFile(filePath string, dataList []Data) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, data := range dataList {
		content := fmt.Sprintf("%s;%s\n", data.Date, data.Data)
		if _, err := f.WriteString(content); err != nil {
			return err
		}
	}
	return nil
}

func sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func InitLedger( ) error {
	log.Println("============= START : Initialize Ledger ===========")

	db, err :=  connectToPostgres("")
	if err != nil {
		return err
	}
	defer db.Close()
	fmt.Println("dbName")
	dbName := "HOSTNAME"
	fmt.Println(dbName)
	_, err =  runQuery(db, fmt.Sprintf("CREATE DATABASE %s;", dbName))
	if err != nil {
		fmt.Println(err)
		return err
	}
	

	db, err =  connectToPostgres(dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err =  runQuery(db, "CREATE TABLE IF NOT EXISTS _keys (sensorId TEXT, _key TEXT);")
	if err != nil {
		return err
	}

	log.Println("============= END : Initialize Ledger ===========")
	return nil
}
/*
func AddSensor(  sensorStr, aStr, bStr string) error {
	log.Println("============= START : AddSensor ===========")
	
	db, err :=  connectToPostgres("hybrid")
	if err != nil {
		return err
	}
	defer db.Close()

	var sensor Sensor
	err = json.Unmarshal([]byte(sensorStr), &sensor)
	if err != nil {
		return err
	}

	a := new(big.Int)
	a.SetString(aStr, 10)

	b := new(big.Int)
	b.SetString(bStr, 10)

	randomizer := big.NewInt(7265483)

	skX := new(big.Int)
	skX.SetString(fmt.Sprintf("%x", sha256.Sum256([]byte(sensor.SharedKey))), 16)

	skY := new(big.Int)
	skY.Mul(a, skX)
	skY.Add(skY, b)
	skY.Mul(skY, randomizer)

	sensor.SharedKey = fmt.Sprintf("%s|%s", skX.String(), skY.String())

	mkX := new(big.Int)
	mkX.SetString(fmt.Sprintf("%x", sha256.Sum256([]byte(sensorStr))), 16)

	mkY := new(big.Int)
	mkY.Mul(a, mkX)
	mkY.Add(mkY, b)
	mkY.Mul(mkY, randomizer)

	myKey := fmt.Sprintf("%s|%s", mkX.String(), mkY.String())

	_, err =  runQuery(db, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			date TIMESTAMPTZ NOT NULL,
			data TEXT
		);`, sensor.SensorID))
	if err != nil {
		return err
	}

	// Convertir la table en hypertable
	_, err =  runQuery(db, fmt.Sprintf(`
		SELECT create_hypertable('%s', 'date', if_not_exists => TRUE);`, sensor.SensorID))
	if err != nil {
		return err
	}

	// Ajouter des index sur la colonne temporelle
	_, err =  runQuery(db, fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS ON %s (time);`, sensor.SensorID))
	if err != nil {
		return err
	}


	dbName := os.Getenv("HOSTNAME")
	db, err =  connectToPostgres(dbName)
	if err != nil {
		return err
	}
	defer db.Close()


	_, err =  runQuery(db, fmt.Sprintf("INSERT INTO _keys (sensorId, _key) VALUES ('%s', '%s')", sensor.SensorID, myKey))
	if err != nil {
		return err
	}

	sensorJSON, err := json.Marshal(sensor)
	if err != nil {
		return err
	}

	

	log.Println("============= END : AddSensor ===========")
	return nil
}

func InsertData(  sensorID, dataStr string) error {
	log.Println("============= START : InsertData ===========")
	var data []Data
	err := json.Unmarshal([]byte(dataStr), &data)
	if err != nil {
		return err
	}

	dbName := os.Getenv("HOSTNAME")
	db, err :=  connectToPostgres(dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	sensorAsBytes, err := ctx.GetStub().GetState(sensorID)
	if err != nil {
		return err
	}
	if sensorAsBytes == nil {
		return fmt.Errorf("%s does not exist", sensorID)
	}

	var sensor Sensor
	err = json.Unmarshal(sensorAsBytes, &sensor)
	if err != nil {
		return err
	}

	rows, err :=  runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
	if err != nil {
		return err
	}
	defer rows.Close()
	
	var myKey string
	if rows.Next() {
		err := rows.Scan(&myKey)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no results found for the given sensorId")
	}

	skXStr, skYStr := splitKey(myKey)
	mkXStr, mkYStr := splitKey(sensor.SharedKey)

	skX := new(big.Int)
	skX.SetString(skXStr, 10)

	skY := new(big.Int)
	skY.SetString(skYStr, 10)

	mkX := new(big.Int)
	mkX.SetString(mkXStr, 10)

	mkY := new(big.Int)
	mkY.SetString(mkYStr, 10)

	a := new(big.Int).Sub(skY, mkY)
	a.Div(a, new(big.Int).Sub(skX, mkX))

	b := new(big.Int).Sub(mkY, new(big.Int).Mul(a, mkX))

	secret := new(big.Int).Mul(a, big.NewInt(658741143))
	secret.Add(secret, b)

	secretStr := secret.String()
	if len(secretStr) < 32 {
		secretStr = fmt.Sprintf("%032s", secretStr)
	}
	secretBuffer := sha256.Sum256([]byte(secretStr[len(secretStr)-32:]))


	db, err =  connectToPostgres("hybrid")
	if err != nil {
		return err
	}
	defer db.Close()

	

	for i, d := range data {
		ciphertext, err := encrypt(secretBuffer[:], []byte(d.Data))
		if err != nil {
			return err
		}
		data[i].Data = hex.EncodeToString(ciphertext) // Update the data to be the encrypted value
	}

	for _, d := range data {
		rows, err =  runQuery(db,fmt.Sprintf("INSERT INTO %s (date, data) VALUES ('%s', '%s')", sensorID, d.Date, d.Data))
		if err != nil {
			log.Printf("Failed to add data in MySQL: %s", err)
			log.Printf("Data will be stored to a temporary file!")

			folderPath := "/tmp/sql_queue"
			filePath := fmt.Sprintf("%s/%s.txt", folderPath, sensorID)
			if err := createFolderIfNotExists(folderPath); err != nil {
				return err
			}
			if err := appendToFile(filePath, data); err != nil {
				return err
			}
		}
	}

	log.Println("============= END : InsertData ===========")
	return nil
}




func ReadData(  sensorID string) (string, error) {
	log.Println("============= START : ReadData ===========")
	dbName := os.Getenv("HOSTNAME")
	db, err :=  connectToPostgres(dbName)
	if err != nil {
		return "",err
	}
	defer db.Close()


	sensorAsBytes, err := ctx.GetStub().GetState(sensorID)
	if err != nil {
		return "", err
	}
	if sensorAsBytes == nil {
		return "", fmt.Errorf("%s does not exist", sensorID)
	}

	var sensor Sensor
	err = json.Unmarshal(sensorAsBytes, &sensor)
	if err != nil {
		return "", err
	}

	rows, err :=  runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var myKey string
	if rows.Next() {
		err := rows.Scan(&myKey)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("no results found for the given sensorId")
	}

	skXStr, skYStr := splitKey(myKey)
	mkXStr, mkYStr := splitKey(sensor.SharedKey)

	skX := new(big.Int)
	skX.SetString(skXStr, 10)

	skY := new(big.Int)
	skY.SetString(skYStr, 10)

	mkX := new(big.Int)
	mkX.SetString(mkXStr, 10)

	mkY := new(big.Int)
	mkY.SetString(mkYStr, 10)

	a := new(big.Int).Sub(skY, mkY)
	a.Div(a, new(big.Int).Sub(skX, mkX))

	b := new(big.Int).Sub(mkY, new(big.Int).Mul(a, mkX))

	secret := new(big.Int).Mul(a, big.NewInt(658741143))
	secret.Add(secret, b)

	secretStr := secret.String()
	if len(secretStr) < 32 {
		secretStr = fmt.Sprintf("%032s", secretStr)
	}
	secretBuffer := sha256.Sum256([]byte(secretStr[len(secretStr)-32:]))


	db, err =  connectToPostgres("hybrid")
	if err != nil {
		return "",err
	}
	defer db.Close()


	rows, err =  runQuery(db, fmt.Sprintf("SELECT * FROM %s", sensorID))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var results []Data
	for rows.Next() {
		var d Data
		err := rows.Scan(&d.Date, &d.Data)
		if err != nil {
			return "", err
		}

		plaintext, err := decrypt(secretBuffer[:], d.Data)
		if err != nil {
			return "", err
		}
		d.Data = string(plaintext)

		results = append(results, d)
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	log.Println("============= END : ReadData ===========")
	return string(resultJSON), nil
}

func speedTest(  sensorID string) (string, error) {
	log.Println("============= START : speedTest ===========")
	dbName := os.Getenv("HOSTNAME")
	db, err :=  connectToPostgres(dbName)
	if err != nil {
		return "",err
	}
	defer db.Close()


	sensorAsBytes, err := ctx.GetStub().GetState(sensorID)
	if err != nil {
		return "", err
	}
	if sensorAsBytes == nil {
		return "", fmt.Errorf("%s does not exist", sensorID)
	}

	var sensor Sensor
	err = json.Unmarshal(sensorAsBytes, &sensor)
	if err != nil {
		return "", err
	}

	rows, err :=  runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var myKey string
	if rows.Next() {
		err := rows.Scan(&myKey)
		if err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("no results found for the given sensorId")
	}

	skXStr, skYStr := splitKey(myKey)
	mkXStr, mkYStr := splitKey(sensor.SharedKey)

	skX := new(big.Int)
	skX.SetString(skXStr, 10)

	skY := new(big.Int)
	skY.SetString(skYStr, 10)

	mkX := new(big.Int)
	mkX.SetString(mkXStr, 10)

	mkY := new(big.Int)
	mkY.SetString(mkYStr, 10)

	a := new(big.Int).Sub(skY, mkY)
	a.Div(a, new(big.Int).Sub(skX, mkX))

	b := new(big.Int).Sub(mkY, new(big.Int).Mul(a, mkX))

	secret := new(big.Int).Mul(a, big.NewInt(658741143))
	secret.Add(secret, b)

	secretStr := secret.String()
	if len(secretStr) < 32 {
		secretStr = fmt.Sprintf("%032s", secretStr)
	}
	secretBuffer := sha256.Sum256([]byte(secretStr[len(secretStr)-32:]))


	db, err =  connectToPostgres("hybrid")
	if err != nil {
		return "",err
	}
	defer db.Close()


	query := fmt.Sprintf(`
		SELECT time_bucket('1 day', time) AS day, avg(data) 
		FROM %s 
		WHERE time >= '2023-01-01' AND time < '2023-02-01' 
		GROUP BY day;`, sensorID)

	rows, err =  runQuery(db, query)
	if err != nil {
		return "",err
	}
	defer rows.Close()


	var results []Data
	for rows.Next() {
		var d Data
		err := rows.Scan(&d.Date, &d.Data)
		if err != nil {
			return "", err
		}

		plaintext, err := decrypt(secretBuffer[:], d.Data)
		if err != nil {
			return "", err
		}
		d.Data = string(plaintext)

		results = append(results, d)
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", err
	}

	log.Println("============= END : speedTest ===========")
	return string(resultJSON), nil
}
*/

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "|")
	return parts[0], parts[1]
}

func encrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := []byte("1234567890abcdef")
	cfb := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	cfb.XORKeyStream(ciphertext, plaintext)
	return ciphertext, nil
}

func decrypt(key []byte, ciphertext string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := []byte("1234567890abcdef")
	ciphertextBytes, _ := hex.DecodeString(ciphertext)
	cfb := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertextBytes))
	cfb.XORKeyStream(plaintext, ciphertextBytes)
	return plaintext, nil
}


func main() {

	// Initialisation du registre
	InitLedger()
	/*
	if err != nil {
		log.Fatalf("Failed to initialize ledger: %v", err)
	}

	// Ajout d'un capteur
	sensorStr := `{"sensorId":"sensor1234", "sharedKey": "test"}`
	aStr := "1238412"
	bStr := "4785623"
	err = AddSensor(nil, sensorStr, aStr, bStr)
	if err != nil {
		log.Fatalf("Failed to add sensor: %v", err)
	}

	// Insertion de données
	dataStr := `[{"date": "2024-05-25 12:34:56", "data": "456.65"}, {"date": "2024-05-26 13:45:00", "data": "460.01"}, {"date": "2024-05-27 14:56:23", "data": "785.1"}]`
	err = InsertData(nil, "sensor1234", dataStr)
	if err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	// Lecture des données
	readData, err := ReadData(nil, "sensor1234")
	if err != nil {
		log.Fatalf("Failed to read data: %v", err)
	}

	fmt.Println("Read Data:", readData)
	*/
}

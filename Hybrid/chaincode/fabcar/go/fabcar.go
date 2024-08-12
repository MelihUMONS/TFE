package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type FabCar struct {
	contractapi.Contract
}

type Sensor struct {
	SensorID  string `json:"sensorId"`
	SharedKey string `json:"sharedKey"`
}

type Data struct {
	Date string `json:"date"`
	Data string `json:"data"`
}

func (s *FabCar) connectToPostgres(dbName string) (*sql.DB, error) {
	var connStr string
	if dbName == "" {
		connStr = "host=172.17.0.1 port=5112 user=postgres password=postgres sslmode=disable"
	} else {
		connStr = fmt.Sprintf("host=172.17.0.1 port=5112 user=postgres password=postgres dbname=%s sslmode=disable", dbName)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %v", err)
	}

	// Ping the database to check if the connection is actually established
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	fmt.Println("Successfully connected to TimescaleDB!")
	return db, nil
}

func (s *FabCar) runQuery(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {

	return db.Query(query, args...)
}

func (s *FabCar) createFolderIfNotExists(folderPath string) error {
	return os.MkdirAll(folderPath, os.ModePerm)
}

func (s *FabCar) appendToFile(filePath string, dataList []Data) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err);return err
	}
	defer f.Close()

	for _, data := range dataList {
		content := fmt.Sprintf("%s;%s\n", data.Date, data.Data)
		if _, err := f.WriteString(content); err != nil {
			fmt.Println(err);return err
		}
	}
	return nil
}

func (s *FabCar) sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (s *FabCar) InitLedger(ctx contractapi.TransactionContextInterface) error {
	log.Println("============= START : Initialize Ledger ===========")

	db, err := s.connectToPostgres("")
	if err != nil {
		fmt.Println(err);return err
	}
	defer db.Close()

	dbName := os.Getenv("HOSTNAME")
	fmt.Println(dbName)
	_, err = s.runQuery(db, fmt.Sprintf(`CREATE DATABASE "%s" ;`, dbName))
	if err != nil {
		fmt.Println(err);return err
	}

	db, err = s.connectToPostgres(dbName)
	if err != nil {
		fmt.Println(err);return err
	}
	defer db.Close()

	_, err = s.runQuery(db, "CREATE TABLE IF NOT EXISTS _keys (sensorId TEXT UNIQUE, _key TEXT);")
	if err != nil {
		fmt.Println(err);return err
	}

	log.Println("============= END : Initialize Ledger ===========")
	return nil
}

func (s *FabCar) AddSensorKeys(ctx contractapi.TransactionContextInterface, sensorStr, aStr, bStr string) error {
	log.Println("============= START : AddSensorKeys ===========")

	var sensor Sensor
	err := json.Unmarshal([]byte(sensorStr), &sensor)
	if err != nil {
		fmt.Println(err)
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

	n := 256

	// Générer un grand nombre aléatoire
	mkX, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), uint(n)))
	if err != nil {
		fmt.Println("Erreur lors de la génération du nombre aléatoire :", err)
		return err
	}


	mkY := new(big.Int)
	mkY.Mul(a, mkX)
	mkY.Add(mkY, b)
	mkY.Mul(mkY, randomizer)

	myKey := fmt.Sprintf("%s|%s", mkX.String(), mkY.String())

	dbName := os.Getenv("HOSTNAME")
	db, err := s.connectToPostgres(dbName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer db.Close()

	_, err = s.runQuery(db, fmt.Sprintf("INSERT INTO _keys (sensorId, _key) VALUES ('%s', '%s')", sensor.SensorID, myKey))
	if err != nil {
		fmt.Println(err)
		return err
	}

	sensorJSON, err := json.Marshal(sensor)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = ctx.GetStub().PutState(sensor.SensorID, sensorJSON)
	if err != nil {
		fmt.Println(err)
		return err
	}

	log.Println("============= END : AddSensorKeys ===========")
	return nil
}

func (s *FabCar) AddSensor(ctx contractapi.TransactionContextInterface, sensorStr, aStr, bStr string) error {
	log.Println("============= START : AddSensor ===========")

	var sensor Sensor
	err := json.Unmarshal([]byte(sensorStr), &sensor)
	if err != nil {
		fmt.Println(err)
		return err
	}

	db, err := s.connectToPostgres("hybrid")
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer db.Close()

	_, err = s.runQuery(db, fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		time TIMESTAMPTZ NOT NULL,
		data TEXT
	);`, sensor.SensorID))
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Convertir la table en hypertable
	_, err = s.runQuery(db, fmt.Sprintf(`
	SELECT create_hypertable('%s', 'time');`, sensor.SensorID))
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Ajouter des index sur la colonne temporelle
	_, err = s.runQuery(db, fmt.Sprintf(`
	CREATE INDEX ON %s (time);`, sensor.SensorID))
	if err != nil {
		fmt.Println(err)
		return err
	}

	log.Println("============= END : AddSensor ===========")
	return nil
}


func (s *FabCar) InsertData(ctx contractapi.TransactionContextInterface, sensorID, dataStr string) error {
	log.Println("============= START : InsertData ===========")
	var data []Data
	err := json.Unmarshal([]byte(dataStr), &data)
	if err != nil {
		fmt.Println(err);return err
	}

	dbName := os.Getenv("HOSTNAME")
	db, err := s.connectToPostgres(dbName)
	if err != nil {
		fmt.Println(err);return err
	}
	defer db.Close()

	sensorAsBytes, err := ctx.GetStub().GetState(sensorID)
	if err != nil {
		fmt.Println(err);return err
	}
	if sensorAsBytes == nil {
		return fmt.Errorf("%s does not exist", sensorID)
	}

	var sensor Sensor
	err = json.Unmarshal(sensorAsBytes, &sensor)
	if err != nil {
		fmt.Println(err);return err
	}

	rows, err := s.runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
	if err != nil {
		fmt.Println(err);return err
	}
	defer rows.Close()
	
	var myKey string
	if rows.Next() {
		err := rows.Scan(&myKey)
		if err != nil {
			fmt.Println(err);return err
		}
	} else {
		return fmt.Errorf("no results found for the given sensorId")
	}

	skXStr, skYStr := splitKey(sensor.SharedKey)
	mkXStr, mkYStr := splitKey(myKey)

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
	fmt.Println("Le secret est ")
	fmt.Println(secretStr)

	if len(secretStr) < 32 {
		secretStr = fmt.Sprintf("%032s", secretStr)
	}
	secretBuffer := sha256.Sum256([]byte(secretStr[len(secretStr)-32:]))


	db, err = s.connectToPostgres("hybrid")
	if err != nil {
		fmt.Println(err);return err
	}
	defer db.Close()

	// Construire la requête d'insertion
	query := fmt.Sprintf("INSERT INTO %s (time, data) VALUES ", sensorID)

	var values []string
	for _, d := range data {
		ciphertext, err := encrypt(secretBuffer[:], []byte(d.Data))
		if err != nil {
			fmt.Println(err)
			return err
		}
		encryptedData := hex.EncodeToString(ciphertext)
		value := fmt.Sprintf("('%s', '%s')", d.Date, encryptedData)
		values = append(values, value)
	}

	// Ajouter toutes les valeurs à la requête
	query += strings.Join(values, ", ")

	fmt.Println("Executing INSERT query ... ")
	// Exécuter la requête d'insertion
	_, err = s.runQuery(db, query)
	if err != nil {
		fmt.Println(err)
		return err
	}


	log.Println("============= END : InsertData ===========")
	return nil
}




func (s *FabCar) ReadData(ctx contractapi.TransactionContextInterface, sensorID string) (string, error) {
	log.Println("============= START : ReadData ===========")
	dbName := os.Getenv("HOSTNAME")
	db, err := s.connectToPostgres(dbName)
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

	rows, err := s.runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
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


	db, err = s.connectToPostgres("hybrid")
	if err != nil {
		return "",err
	}
	defer db.Close()


	rows, err = s.runQuery(db, fmt.Sprintf("SELECT * FROM %s", sensorID))
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

func (s *FabCar) SpeedTestInsert(ctx contractapi.TransactionContextInterface, sensorID string) error {
	log.Println("============= START : InsertEncryptedData ===========")
	dbName := os.Getenv("HOSTNAME")
	db, err := s.connectToPostgres(dbName)
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

	rows, err := s.runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
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

	db, err = s.connectToPostgres("hybrid")
	if err != nil {
		return err
	}
	defer db.Close()

	startTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2023, 1, 9, 0, 0, 0, 0, time.UTC)
	totalData := int(endTime.Sub(startTime).Seconds())
	data := make([]Data, totalData)

	for i := 0; i < totalData; i++ {
		data[i] = Data{
			Date: startTime.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Data: fmt.Sprintf("%d",i),
		}
	}

	batchSize := 50000
	for i := 0; i < totalData; i += batchSize {
		end := i + batchSize
		if end > totalData {
			end = totalData
		}

		query := fmt.Sprintf("INSERT INTO %s (time, data) VALUES ", sensorID)
		var values []string

		for _, d := range data[i:end] {
			ciphertext, err := encrypt(secretBuffer[:], []byte(d.Data))
			if err != nil {
				fmt.Println(err)
				return err
			}
			encryptedData := hex.EncodeToString(ciphertext)
			value := fmt.Sprintf("('%s', '%s')", d.Date, encryptedData)
			values = append(values, value)
		}

		query += strings.Join(values, ", ")

		fmt.Println("Executing INSERT query ... ")
		_, err = s.runQuery(db, query)
		if err != nil {
			fmt.Println(err)
			return err
		}

		log.Printf("Inserted %d/%d data points\n", end, totalData)
		s.sleep(1000)
	}

	log.Println("============= END : InsertEncryptedData ===========")
	return nil
}



func (s *FabCar) SpeedTestQuery(ctx contractapi.TransactionContextInterface, sensorID string) (string, error) {
	log.Println("============= START : speedTest ===========")
	dbName := os.Getenv("HOSTNAME")
	db, err := s.connectToPostgres(dbName)
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

	rows, err := s.runQuery(db, fmt.Sprintf("SELECT _key FROM _keys WHERE sensorId = '%s'", sensor.SensorID))
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


	db, err = s.connectToPostgres("hybrid")
	if err != nil {
		return "",err
	}
	defer db.Close()


	query := fmt.Sprintf(`
		SELECT time_bucket('1 day', time) AS day, data 
		FROM %s 
		WHERE time >= '2023-01-01' AND time < '2023-01-02';`, sensorID)

	rows, err = s.runQuery(db, query)
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
	chaincode, err := contractapi.NewChaincode(new(FabCar))
	if err != nil {
		log.Fatalf("Error creating FabCar chaincode: %s", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Fatalf("Error starting FabCar chaincode: %s", err)
	}
}

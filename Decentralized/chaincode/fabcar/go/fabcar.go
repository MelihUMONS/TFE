package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

func (s *FabCar) connectToDatabase(mysqlConnStr string) (*sql.DB, error) {
	db, err := sql.Open("mysql", mysqlConnStr)
	if err != nil {
		log.Printf("Error connecting to MySQL: %s", err)
		return nil, err
	}
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

func (s *FabCar) sleep(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (s *FabCar) InitLedger(ctx contractapi.TransactionContextInterface) error {
	log.Println("============= START : Initialize Ledger ===========")
	mysqlConnStr := "root:rootpassword0@tcp(172.17.0.1:3306)/"
	db, err := s.connectToDatabase(mysqlConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	dbName := os.Getenv("HOSTNAME")
	s.sleep(1000)

	_, err = s.runQuery(db, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName))
	if err != nil {
		return err
	}

	dbConnStr := fmt.Sprintf("root:rootpassword0@tcp(172.17.0.1:3306)/%s", dbName)
	db, err = s.connectToDatabase(dbConnStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = s.runQuery(db, "CREATE TABLE IF NOT EXISTS _keys (sensorId TEXT, _key TEXT);")
	if err != nil {
		return err
	}

	log.Println("============= END : Initialize Ledger ===========")
	return nil
}

func (s *FabCar) AddSensor(ctx contractapi.TransactionContextInterface, sensorStr, aStr, bStr string) error {
	log.Println("============= START : AddSensor ===========")
	mysqlConnStr := fmt.Sprintf("root:rootpassword0@tcp(172.17.0.1:3306)/%s", os.Getenv("HOSTNAME"))
	db, err := s.connectToDatabase(mysqlConnStr)
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

	_, err = s.runQuery(db, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (date DATETIME DEFAULT CURRENT_TIMESTAMP, data TEXT);", sensor.SensorID))
	if err != nil {
		return err
	}

	_, err = s.runQuery(db, "INSERT INTO _keys (sensorId, _key) VALUES (?, ?)", sensor.SensorID, myKey)
	if err != nil {
		return err
	}

	sensorJSON, err := json.Marshal(sensor)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(sensor.SensorID, sensorJSON)
	if err != nil {
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
		return err
	}

	mysqlConnStr := fmt.Sprintf("root:rootpassword0@tcp(172.17.0.1:3306)/%s", os.Getenv("HOSTNAME"))
	db, err := s.connectToDatabase(mysqlConnStr)
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

	rows, err := s.runQuery(db, "SELECT _key FROM _keys WHERE sensorId = ?", sensor.SensorID)
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

	for _, d := range data {
		iv := []byte("1234567890abcdef")
		block, err := aes.NewCipher(secretBuffer[:])
		if err != nil {
			return err
		}
		ciphertext := encrypt(block, iv, []byte(d.Data))
		d.Data = hex.EncodeToString(ciphertext)
	}

	for _, d := range data {
		_, err = s.runQuery(db, fmt.Sprintf("INSERT INTO %s (date, data) VALUES (?, ?)", sensorID), d.Date, d.Data)
		if err != nil {
			log.Printf("Failed to add data in MySQL: %s", err)
			log.Printf("Data will be stored to a temporary file!")

			folderPath := "/tmp/sql_queue"
			filePath := fmt.Sprintf("%s/%s.txt", folderPath, sensorID)
			if err := s.createFolderIfNotExists(folderPath); err != nil {
				return err
			}
			if err := s.appendToFile(filePath, data); err != nil {
				return err
			}
		}
	}

	log.Println("============= END : InsertData ===========")
	return nil
}




func (s *FabCar) ReadData(ctx contractapi.TransactionContextInterface, sensorID string) (string, error) {
	log.Println("============= START : ReadData ===========")
	mysqlConnStr := fmt.Sprintf("root:rootpassword0@tcp(172.17.0.1:3306)/%s", os.Getenv("HOSTNAME"))
	db, err := s.connectToDatabase(mysqlConnStr)
	if err != nil {
		return "", err
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

	rows, err := s.runQuery(db, "SELECT _key FROM _keys WHERE sensorId = ?", sensor.SensorID)
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

		iv := []byte("1234567890abcdef")
		block, err := aes.NewCipher(secretBuffer[:])
		if err != nil {
			return "", err
		}
		plaintext := decrypt(block, iv, d.Data)
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




func (s *FabCar) SendFingerprint(ctx contractapi.TransactionContextInterface) error {
	fingerprint := "ERROR"
	cmd := fmt.Sprintf(`MYSQL_PWD=rootpassword0 mysqldump -h 172.17.0.1 -P 3306 -u root --no-create-info --skip-comments --compact --ignore-table=%s._keys %s | md5sum`, os.Getenv("HOSTNAME"), os.Getenv("HOSTNAME"))

	output, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return err
	}

	fingerprint = strings.TrimSpace(strings.Split(string(output), " ")[0])

	resp, err := http.Get(fmt.Sprintf("http://localhost:5000/?peerId=%s&fingerprint=\"%s\"", os.Getenv("HOSTNAME"), fingerprint))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (s *FabCar) EndorseCeremony(ctx contractapi.TransactionContextInterface, goodFingerprint string) error {
	log.Println("============= START : endorseCeremony ===========")

	fingerprintKey := fmt.Sprintf("fingerprint_%s", time.Now().Format(time.RFC3339))

	fingerprintRecord := map[string]string{
		"fingerprint": goodFingerprint,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	fingerprintJSON, err := json.Marshal(fingerprintRecord)
	if err != nil {
		return err
	}

	err = ctx.GetStub().PutState(fingerprintKey, fingerprintJSON)
	if err != nil {
		return err
	}

	log.Println("============= END : endorseCeremony ===========")
	return nil
}

func (s *FabCar) TestUpdate(ctx contractapi.TransactionContextInterface, goodFingerprint string) (string, error) {
	log.Println("============= START : testUpdate ===========")

	log.Println("============= END : testUpdate ===========")
	return "Updated!", nil
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, "|")
	return parts[0], parts[1]
}

func encrypt(block cipher.Block, iv, plaintext []byte) []byte {
	cfb := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	cfb.XORKeyStream(ciphertext, plaintext)
	return ciphertext
}


func decrypt(block cipher.Block, iv []byte, ciphertext string) []byte {
	ciphertextBytes, _ := hex.DecodeString(ciphertext)
	cfb := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertextBytes))
	cfb.XORKeyStream(plaintext, ciphertextBytes)
	return plaintext
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

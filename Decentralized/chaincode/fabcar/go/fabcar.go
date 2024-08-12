package main

import (
    "encoding/json"
    "fmt"
    "log"
    "strconv"

    "github.com/hyperledger/fabric-contract-api-go/contractapi"
    "github.com/hyperledger/fabric/common/flogging"
)

type SmartContract struct {
    contractapi.Contract
}

type SensorData struct {
    SensorId  string `json:"sensorId"`
    Data      string `json:"data"`
    Timestamp string `json:"timestamp"`
}

type PrivateSensorData struct {
    SensorId  string `json:"sensorId"`
    Data      string `json:"data"`
    Timestamp string `json:"timestamp"`
}

var logger = flogging.MustGetLogger("sensor_cc")

// InitLedger initializes the ledger with some sample data
func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
    logger.Info("Initializing Ledger")

    sensorData := []SensorData{
        {SensorId: "S1", Data: "26.5", Timestamp: "2024-07-30T10:00:00Z"},
        {SensorId: "S2", Data: "27.1", Timestamp: "2024-07-30T10:01:00Z"},
    }

    for i, data := range sensorData {
        dataAsBytes, _ := json.Marshal(data)
        err := ctx.GetStub().PutState("SENSOR"+strconv.Itoa(i), dataAsBytes)

        if err != nil {
            return fmt.Errorf("failed to put sensor data: %v", err)
        }
    }

    return nil
}

// AddSensorData adds new sensor data to the ledger
func (s *SmartContract) AddSensorData(ctx contractapi.TransactionContextInterface, sensorId, data, timestamp string) error {
    sensorData := SensorData{
        SensorId:  sensorId,
        Data:      data,
        Timestamp: timestamp,
    }

    dataAsBytes, _ := json.Marshal(sensorData)
    return ctx.GetStub().PutState(sensorId, dataAsBytes)
}

// QuerySensorData returns the sensor data stored in the ledger
func (s *SmartContract) QuerySensorData(ctx contractapi.TransactionContextInterface, sensorId string) (*SensorData, error) {
    dataAsBytes, err := ctx.GetStub().GetState(sensorId)
    if err != nil {
        return nil, fmt.Errorf("failed to read from world state: %v", err)
    }
    if dataAsBytes == nil {
        return nil, fmt.Errorf("%s does not exist", sensorId)
    }

    sensorData := new(SensorData)
    _ = json.Unmarshal(dataAsBytes, sensorData)

    return sensorData, nil
}

// QuerySensorDataBySensorId queries sensor data using the SensorId field with CouchDB index
func (s *SmartContract) QuerySensorDataBySensorId(ctx contractapi.TransactionContextInterface, sensorId string) ([]*SensorData, error) {
    queryString := fmt.Sprintf(`{"selector":{"SensorId":"%s"},"use_index":"indexSensorDoc"}`, sensorId)

    resultsIterator, err := ctx.GetStub().GetQueryResult(queryString)
    if err != nil {
        return nil, fmt.Errorf("failed to query sensor data: %v", err)
    }
    defer resultsIterator.Close()

    var results []*SensorData
    for resultsIterator.HasNext() {
        queryResponse, err := resultsIterator.Next()
        if err != nil {
            return nil, err
        }

        var sensorData SensorData
        err = json.Unmarshal(queryResponse.Value, &sensorData)
        if err != nil {
            return nil, err
        }
        results = append(results, &sensorData)
    }

    return results, nil
}

// AddPrivateSensorData adds new private sensor data to the ledger
func (s *SmartContract) AddPrivateSensorData(ctx contractapi.TransactionContextInterface, collection, sensorId, data, timestamp string) error {
    privateSensorData := PrivateSensorData{
        SensorId:  sensorId,
        Data:      data,
        Timestamp: timestamp,
    }

    dataAsBytes, _ := json.Marshal(privateSensorData)
    return ctx.GetStub().PutPrivateData(collection, sensorId, dataAsBytes)
}

// QueryPrivateSensorData returns the private sensor data stored in the ledger
func (s *SmartContract) QueryPrivateSensorData(ctx contractapi.TransactionContextInterface, collection, sensorId string) (*PrivateSensorData, error) {
    dataAsBytes, err := ctx.GetStub().GetPrivateData(collection, sensorId)
    if err != nil {
        return nil, fmt.Errorf("failed to read private data from world state: %v", err)
    }
    if dataAsBytes == nil {
        return nil, fmt.Errorf("%s does not exist in collection %s", sensorId, collection)
    }

    privateSensorData := new(PrivateSensorData)
    _ = json.Unmarshal(dataAsBytes, privateSensorData)

    return privateSensorData, nil
}

func main() {
    chaincode, err := contractapi.NewChaincode(new(SmartContract))
    if err != nil {
        log.Panicf("Error creating sensor chaincode: %v", err)
    }

    if err := chaincode.Start(); err != nil {
        log.Panicf("Error starting sensor chaincode: %v", err)
    }
}


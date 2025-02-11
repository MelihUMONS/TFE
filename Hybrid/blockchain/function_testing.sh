#!/bin/bash

export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=$PWD/../config/
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_LOCALMSPID=Org1MSP
export CORE_PEER_TLS_ROOTCERT_FILE=${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt
export CORE_PEER_MSPCONFIGPATH=${PWD}/organizations/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp
export CORE_PEER_ADDRESS=localhost:7051

sleep 3

peer chaincode query -C mychannel -n fabcar -c '{"Args":["AddSensor","{\"sensorId\":\"sensortest\", \"PatiendId\": 4567, \"sharedKey\": \"test\"}","1238412","4785623"]}'

sleep 3

peer chaincode invoke -o localhost:7050 --ordererTLSHostnameOverride orderer.example.com --tls --cafile "${PWD}/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem" -C mychannel -n fabcar --peerAddresses localhost:7051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt" --peerAddresses localhost:9051 --tlsRootCertFiles "${PWD}/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt" -c '{"function":"AddSensorKeys","Args":["{\"sensorId\":\"sensortest\", \"PatiendId\": 4567, \"sharedKey\": \"test\"}","1238412","4785623"]}'

sleep 3

peer chaincode query -C mychannel -n fabcar -c '{"Args":["SpeedTestInsert","sensortest"]}'

sleep 3

peer chaincode query -C mychannel -n fabcar -c '{"Args":["ReadData","sensortest"]}'
sleep 3
peer chaincode query -C mychannel -n fabcar -c '{"Args":["SpeedTestQuery","sensortest"]}'

peer chaincode query -C mychannel -n fabcar -c '{"Args":["InsertData","sensortest1",'$(cat data.json)']}'

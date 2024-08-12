'use strict';

const { Contract } = require('fabric-contract-api');
const pgp = require('pg-promise')();
const crypto = require('crypto');
const fs = require('fs');
const { exec } = require('child_process');
const path = require('path');
const { env } = require('process');

class FabCar extends Contract {

    // ############################################################################
    // ############################################################################
    //
    //                              UTILS
    //
    // ############################################################################
    // ############################################################################

    async connectToDatabase(timescaleConnStr) {
        return new Promise((resolve, reject) => {
            try {
                const db = pgp(timescaleConnStr);
                console.info('TimescaleDB connection established');
                resolve(db);
            } catch (error) {
                console.error('Error connecting to TimescaleDB:', error);
                reject(error);
            }
        });
    }

    async runQuery(db, query, params = []) {
        return new Promise((resolve, reject) => {
            db.any(query, params)
                .then(results => {
                    resolve(results);
                })
                .catch(error => {
                    console.error('Error executing query:', error);
                    reject(error);
                });
        });
    }

    async createFolderIfNotExists(folderPath) {
        if (!fs.existsSync(folderPath)) {
            fs.mkdirSync(folderPath, { recursive: true });
            console.info(`Created folder: ${folderPath}`);
        }
    }

    async appendToFile(filePath, dataList) {
        if (!fs.existsSync(filePath)) {
            fs.writeFileSync(filePath, '');
            console.info(`Created file: ${filePath}`);
        }

        dataList.forEach(data => {
            const content = `${data.date};${data.data}`;
            fs.appendFileSync(filePath, `\n${content}`);
        });

        console.info(`Appended content to file: ${filePath}`);
    }

    async sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    // ############################################################################
    // ############################################################################
    //          
    //                              CHAINCODE
    //
    // ############################################################################
    // ############################################################################

    async initLedger(ctx) {
        console.info('============= START : Initialize Ledger ===========');

        //create database
        const timescaleConnStr = 'postgres://myuser:rootpassword@172.17.0.1:5432/postgres';
        const db = await this.connectToDatabase(timescaleConnStr);

        const dbName = process.env.HOSTNAME;

        // Create the database if it doesn't exist
        await this.runQuery(db, `CREATE DATABASE ${dbName}`);

        // Connect to the new database
        const dbConnStr = `postgres://myuser:rootpassword@172.17.0.1:5432/${dbName}`;
        const dbConnection = await this.connectToDatabase(dbConnStr);

        // Create the _keys table
        await this.runQuery(dbConnection, 'CREATE TABLE IF NOT EXISTS _keys (sensorId TEXT, _key TEXT);');

        console.info('============= END : Initialize Ledger ===========');
    }


    async AddSensor(ctx, sensorstr, a_str, b_str) {
        console.info('============= START : AddSensor ===========');
        const mysqlConnStr = {host: "172.17.0.1", port: 3306, user: "root", password: "rootpassword0", database: process.env.HOSTNAME};
        const connection = await this.connectToDatabase(mysqlConnStr);
        const sensor = JSON.parse(sensorstr);
        const a = BigInt(a_str);
        const b = BigInt(b_str);
        const randomizer = BigInt(7265483); //hardcoded to prevent "leaked a and b during transmission"

        //compute shared key (stored in the bloc and visible for everyone)
        let sk_x = BigInt(BigInt('0x' + Buffer.from(sensor.sharedKey, 'utf-8').toString('hex')));
        let sk_y = randomizer * (a * sk_x + b);
        sensor.sharedKey = sk_x.toString() + '|' + sk_y.toString();

        //compute my key
        let mk_x = BigInt('0x' + crypto.randomBytes(32).toString('hex'));
        let mk_y = randomizer * (a * mk_x + b);
        let mykey = mk_x.toString() + '|' + mk_y.toString();

        //create table for sensor
        await this.runQuery(connection,`CREATE TABLE IF NOT EXISTS ${sensor.sensorId} (date DATETIME DEFAULT CURRENT_TIMESTAMP, data TEXT);`);
        console.info('Created sensor table in MySQL !');

        //store my key into database (other key storing systems can be possible)
        await this.runQuery(connection,'INSERT INTO _keys (sensorId, _key) VALUES (?, ?)',[sensor.sensorId, mykey]);
        console.info('Inserted key into MySQL !');

        //insert bloc
        await ctx.stub.putState(sensor.sensorId, Buffer.from(JSON.stringify(sensor)));
        console.info('============= END : AddSensor ===========');
    }

    async InsertData(ctx, sensorId, datastr) {
        console.info('============= START : InsertData ===========');
        const data = JSON.parse(datastr);
        let results = undefined;

        try {
            const mysqlConnStr = {host: "172.17.0.1", port: 3306, user: "root", password: "rootpassword0", database: process.env.HOSTNAME};
            const connection = await this.connectToDatabase(mysqlConnStr);

            // Get shared Key from blockchain
            const sensorAsBytes = await ctx.stub.getState(sensorId);
            if (!sensorAsBytes || sensorAsBytes.length === 0) {
                throw new Error(`${sensorId} does not exist`);
            }
            let sensorFound = JSON.parse(sensorAsBytes.toString());

            //get my key
            results = await this.runQuery(connection,'SELECT _key FROM _keys WHERE sensorId = ? ',[sensorFound.sensorId]);
            if (results.length == 0) 
                return console.info('No results found for the given sensorId.');
            const mykey = results[0]._key;
            const [sk_x_str, sk_y_str] = mykey.split('|');
            const [mk_x_str, mk_y_str] = sensorFound.sharedKey.split('|');
            const sk_x = BigInt(sk_x_str);
            const sk_y = BigInt(sk_y_str);
            const mk_x = BigInt(mk_x_str);
            const mk_y = BigInt(mk_y_str);

            // Compute secret with Shamir's Secret Sharing
            const a = (sk_y - mk_y) / (sk_x - mk_x);
            const b = mk_y - a * mk_x;
            const secret = a * BigInt(658741143) + b; 

            console.info('Computed secret:', secret.toString());
            // Encrypt data using AES-256 encryption
            const secretBuffer = Buffer.from(secret.toString().slice(-32), 'utf-8'); // Use last 256 bits
            for (const d of data) {
                const iv = Buffer.from('1234567890abcdef1234567890abcdef', 'hex');
                const cipher = crypto.createCipheriv('aes-256-cbc', crypto.createHash('sha256').update(secretBuffer).digest(), iv);
                let encrypted = cipher.update(Buffer.from(d['data']), undefined,'hex');
                encrypted += cipher.final('hex');
                d['data'] = encrypted;
            }

            // Insert data into MySQL
            for (const d of data) 
                await this.runQuery(connection,`INSERT INTO ${mysql.escapeId(sensorId)} (date, data) VALUES (?, ?)`,[d['date'], d['data']]);
        } catch (err) {
            console.info("Failed to add data in MySQL: " + err);
            console.info("Data will be stored to a temporary file!");

            const folderPath = '/tmp/sql_queue';
            const filePath = path.join(folderPath, `${sensorId}.txt`);
            this.createFolderIfNotExists(folderPath);
            this.appendToFile(filePath, data);

        }
        console.info('============= END : InsertData ===========');
    }



    async ReadData(ctx, sensorId) {
        console.info('============= START : ReadData ===========');

        const mysqlConnStr = {host: "172.17.0.1", port: 3306, user: "root", password: "rootpassword0", database: process.env.HOSTNAME};
        const connection = await this.connectToDatabase(mysqlConnStr);
        let results = undefined;
        // Get shared Key from blockchain
        const sensorAsBytes = await ctx.stub.getState(sensorId);
        if (!sensorAsBytes || sensorAsBytes.length === 0) {
            throw new Error(`${sensorId} does not exist`);
        }
        let sensorFound = JSON.parse(sensorAsBytes.toString());


        try{
            results = await this.runQuery(connection,'SELECT _key FROM _keys WHERE sensorId = ? ',[sensorFound.sensorId]);
            console.info('Inserted key into MySQL !');
        }catch{console.error('Error selecting key from MySQL:', error);}

        if (results.length == 0) 
            return console.info('No results found for the given sensorId.');

        const mykey = results[0]._key;
        console.info('Got the shared key from MySQL:', mykey);

        // Compute secret with SSS (Shamir's Secret Sharing)
        const [sk_x_str, sk_y_str] = mykey.split('|');
        const [mk_x_str, mk_y_str] = sensorFound.sharedKey.split('|');

        const sk_x = BigInt(sk_x_str);
        const sk_y = BigInt(sk_y_str);
        const mk_x = BigInt(mk_x_str);
        const mk_y = BigInt(mk_y_str);

        const a = (sk_y - mk_y) / (sk_x - mk_x);
        const b = mk_y - a * mk_x;
        const secret = a * BigInt(658741143) + b; // 658741.145 * 10^6 to handle it as BigInt

        try{
            results = await this.runQuery(connection,`SELECT * FROM ${mysql.escapeId(sensorId)}`);
            console.info('Selected data from MySQL !');
        }catch{console.error('Error selecting data from MySQL:', error);}
        

        const secretBuffer = Buffer.from(secret.toString().slice(-32), 'utf-8'); // Use last 256 bits
        

        for (const d of results) {
            const iv = Buffer.from('1234567890abcdef1234567890abcdef', 'hex');
            const decipher = crypto.createDecipheriv('aes-256-cbc', crypto.createHash('sha256').update(secretBuffer).digest(), iv);
            const encryptedText = d['data'];
            let decrypted = decipher.update(encryptedText, 'hex', 'utf8');
            decrypted += decipher.final('utf8');
            d['data'] = decrypted; // Store the decrypted data
        }
        console.log(results);
        console.info('============= END : ReadData ===========');

        return JSON.stringify(results);
    }
}

module.exports = FabCar;

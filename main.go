package main

import (
	"flag"
	"fmt"

	"github.com/joho/godotenv"
)

func main() {
	var envFile, inputDir, outputDir string
	flag.StringVar(&envFile, "config", ".env", ".env config file")
	flag.StringVar(&inputDir, "input", "list_addresses.txt", "BTC addresses")
	flag.StringVar(&outputDir, "output", "txs_report.csv", "The report in CSV format")
	flag.Parse()

	err := godotenv.Load(envFile)
	if err != nil {
		panic(fmt.Sprintf("Error loading %v file", envFile))
	}

	// Init BTC client
	btcClient := newBTCClient()

	addresses := readAddresses(inputDir)
	fmt.Printf("Len of addresses: %v\n", len(addresses))

	records := getOutTxsFromAddresses(btcClient, addresses)

	writeRecordsToCSV(records, outputDir)
}

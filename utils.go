package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/btcsuite/btcd/rpcclient"
)

func newBTCClient() *rpcclient.Client {
	connCfg := &rpcclient.ConnConfig{
		Host:         os.Getenv("BTC_NODE_HOST"),
		User:         os.Getenv("BTC_NODE_USERNAME"),
		Pass:         os.Getenv("BTC_NODE_PASSWORD"),
		HTTPPostMode: true,                                     // Bitcoin core only supports HTTP POST mode
		DisableTLS:   !(os.Getenv("BTC_NODE_HTTPS") == "true"), // Bitcoin core does not provide TLS by default
	}
	btcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		panic(err)
	}
	return btcClient
}

func readAddresses(fileDir string) []string {
	file, err := os.Open(fileDir)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	addresses := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		addresses = append(addresses, scanner.Text())
	}
	return addresses
}

func importAddresses(btcClient *rpcclient.Client, addresses []string) {
	for idx, address := range addresses {
		err := btcClient.ImportAddressRescan(address, "", false)
		if err != nil {
			panic(err)
		}
		if idx%50 == 0 {
			fmt.Printf("Scanned %v addresses\n", idx+1)
		}
	}
}

type VOutInfo struct {
	Address string `json:"scriptpubkey_address"`
	Value   uint64 `json:"value"`
}

type VInInfo struct {
	PrevOut VOutInfo `json:"prevout"`
}

type TxInfo struct {
	TxID string     `json:"txid"`
	Vin  []VInInfo  `json:"vin"`
	Vout []VOutInfo `json:"vout"`
	Fee  uint64     `json:"fee"`
}

func getOutTxsFromAddresses(btcClient *rpcclient.Client, addresses []string) [][]string {
	records := [][]string{}

	for _, address := range addresses {
		lastSeenTxID := ""

		for {
			requestDir := fmt.Sprintf("https://blockstream.info/api/address/%v/txs/chain/%v", address, lastSeenTxID)

			response, err := http.Get(requestDir)
			if err != nil {
				panic(err)
			}
			responseData, err := ioutil.ReadAll(response.Body)
			if err != nil {
				panic(err)
			}
			var res []TxInfo
			err = json.Unmarshal(responseData, &res)
			if err != nil {
				panic(err)
			}

			for _, tx := range res {
				isSend := true
				for idx := 0; idx < len(tx.Vin); idx++ {
					if tx.Vin[idx].PrevOut.Address != address {
						isSend = false
						break
					}
				}
				if !isSend {
					continue
				}

				receiveAddresses := []string{}
				receiveAmount := uint64(0)
				for _, vout := range tx.Vout {
					if tx.Vin[0].PrevOut.Address != vout.Address {
						receiveAmount += vout.Value
						receiveAddresses = append(receiveAddresses, vout.Address)
					}
				}
				if len(receiveAddresses) == 0 {
					for _, vout := range tx.Vout {
						receiveAmount += vout.Value
						receiveAddresses = append(receiveAddresses, vout.Address)
					}
				}
				if len(receiveAddresses) != 1 {
					isEqual := true
					for idx := 1; idx < len(receiveAddresses); idx++ {
						if receiveAddresses[idx] != receiveAddresses[0] {
							isEqual = false
							break
						}
					}
					if !isEqual {
						fmt.Printf("More than one receiver txID: %v\n", tx.TxID)
						continue
					}
				}
				records = append(records, []string{
					tx.TxID,
					tx.Vin[0].PrevOut.Address,
					receiveAddresses[0],
					strconv.FormatUint(receiveAmount, 10),
					strconv.FormatUint(tx.Fee, 10),
				})
			}

			if len(res) < 25 {
				break
			}
			lastSeenTxID = res[24].TxID
		}

		fmt.Printf("Scanned address %v\n", address)

	}
	return records
}

func writeFeeRecordsToCSV(records [][]string, fileDir string) {
	file, err := os.Create(fileDir)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"txID", "from", "to", "amount", "fee"})
	for _, record := range records {
		writer.Write(record)
	}
	writer.Flush()
}

type BalanceInfo struct {
	TotalSum uint64 `json:"funded_txo_sum"`
	SpentSum uint64 `json:"spent_txo_sum"`
}

type AccountInfo struct {
	Balance BalanceInfo `json:"chain_stats"`
}

func balanceAddresses(btcClient *rpcclient.Client, addresses []string) [][]string {
	records := [][]string{}

	for _, address := range addresses {
		requestDir := fmt.Sprintf("https://blockstream.info/api/address/%v", address)

		response, err := http.Get(requestDir)
		if err != nil {
			panic(err)
		}
		responseData, err := ioutil.ReadAll(response.Body)
		if err != nil {
			panic(err)
		}
		var res AccountInfo
		err = json.Unmarshal(responseData, &res)
		if err != nil {
			panic(err)
		}

		balance := res.Balance.TotalSum - res.Balance.SpentSum

		records = append(records, []string{
			address,
			strconv.FormatUint(balance, 10),
		})

		fmt.Printf("Scanned address %v, balance %v\n", address, float64(balance)/1e8)

	}
	return records
}

func writeBalanceRecordsToCSV(records [][]string, fileDir string) {
	file, err := os.Create(fileDir)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"address", "balance"})
	for _, record := range records {
		writer.Write(record)
	}
	writer.Flush()
}

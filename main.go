package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gocarina/gocsv"
	"golang.org/x/time/rate"

	"example.com/challenge/cipher"
	"example.com/challenge/models"
	tomise "example.com/challenge/omisethrottled"
	"example.com/challenge/workerpool"
)

const (
	TotalWorkers = 4

	ThrottleDuration = 10 * time.Second
	OmisePublicKey   = "pkey_test_60122sx1h21f2f193zi"
	OmiseSecretKey   = "skey_test_60122rti2pfx7hwtuph"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: program <encrypted-file>")
		os.Exit(1)
	}
	records, err := readFile(os.Args[1])
	if err != nil {
		fmt.Println("Error reading file: ", err)
		os.Exit(1)
	}

	recordsLen := len(records)
	if recordsLen == 0 {
		fmt.Println("No records found in the file.")
		os.Exit(1)
	}

	fmt.Println("performing donations...")

	client, err := tomise.NewClient(OmisePublicKey, OmiseSecretKey, ThrottleDuration)
	if err != nil {
		fmt.Println("Error creating omise client: ", err)
		os.Exit(1)
	}

	pool := workerpool.NewWorkerPool(
		context.Background(),
		client,
		recordsLen,
		TotalWorkers,
		rate.NewLimiter(rate.Every(time.Second/20), 1), // https://docs.opn.ooo/api-rate-limiting
	)
	defer pool.Close()

	// process records
	pool.ProcessRecords(records)
	results := pool.GetResults(recordsLen)

	// print summary
	printSummary(results, records, recordsLen)
}

func readFile(path string) ([]*models.Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rotReader, err := cipher.NewRot128Reader(file)
	if err != nil {
		return nil, err
	}

	records := []*models.Record{}
	if err := gocsv.Unmarshal(rotReader, &records); err != nil {
		return nil, err
	}

	return records, nil
}

func printSummary(results []*models.Record, records []*models.Record, recordsLen int) {
	var totalRecieved, successfullyDonated, faultyDonation, averageDonation int64 = 0, 0, 0, 0
	var topDonors = [3]int{-1, -1, -1}
	for _, r := range results {
		if r.Error != nil {
			fmt.Printf("Error: %d : %v\n", r.Idx, r.Error)
			faultyDonation += r.AmountSubunits
			continue
		}

		successfullyDonated += r.AmountSubunits
		tempIdx := -1
		for d := 0; d < len(topDonors); d++ {
			if topDonors[d] == -1 {
				topDonors[d] = r.Idx
				break
			} else {
				if records[topDonors[d]].AmountSubunits < r.AmountSubunits {
					tempIdx = r.Idx
					r = records[topDonors[d]]
					topDonors[d] = tempIdx
				}
			}
		}
	}

	totalRecieved = successfullyDonated + faultyDonation
	averageDonation = totalRecieved / int64(recordsLen)

	fmt.Println("done.")
	fmt.Println()
	fmt.Printf("%25s: THB %25s\n", "total received", strconv.FormatInt(totalRecieved, 10))
	fmt.Printf("%25s: THB %25s\n", "successfully donated", strconv.FormatInt(successfullyDonated, 10))
	fmt.Printf("%25s: THB %25s\n", "faulty donation", strconv.FormatInt(faultyDonation, 10))
	fmt.Println()
	fmt.Printf("%25s: THB %25s\n", "average per person", strconv.FormatInt(averageDonation, 10))
	fmt.Printf("%25s: THB %s\n", "top donors", records[topDonors[0]].Name)
	fmt.Printf("%25s  THB %s\n", "", records[topDonors[1]].Name)
	fmt.Printf("%25s  THB %s\n", "", records[topDonors[2]].Name)
}

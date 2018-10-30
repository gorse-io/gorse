package main

import (
	"bufio"
	"fmt"
	"gonum.org/v1/gonum/floats"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	file, err := os.Open("D:\\data.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	count := []float64{0, 0}
	mlen := []float64{0, 0}
	plen := []float64{0, 0}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return r == ','
		})
		prefix := strings.FieldsFunc(fields[4], func(r rune) bool {
			return r == '/'
		})
		label := fields[6]
		mask, _ := strconv.Atoi(prefix[1])
		if label == "0" {
			count[0]++
			mlen[0] += float64(mask)
			plen[0] += float64(len(strings.FieldsFunc(fields[5], func(r rune) bool {
				return r == '-'
			})))
		} else {
			count[1]++
			mlen[1] += float64(mask)
			plen[1] += float64(len(strings.FieldsFunc(fields[5], func(r rune) bool {
				return r == '-'
			})))
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println(count)
	floats.Div(mlen, count)
	fmt.Println(mlen)
	floats.Div(plen, count)
	fmt.Println(plen)
}

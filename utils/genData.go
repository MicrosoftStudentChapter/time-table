package utils

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

func GenerateJson() {
	f, err := excelize.OpenFile("timetable1.xlsx")
	defer func() {
		if err = f.Close(); err != nil {
			panic(err)
		}
	}()
	HandleError(err)
	sheets := f.GetSheetList()

	// Define categories to exclude
	excludedCategories := map[string]bool{
		"2ND ECE":          true,
		"2ND YEAR ECE ENC": true,
		"3RD ECE":          true,
		"4TH ECE":          true,
		"4TH YEAR B":       true,
		"DLIT":             true,
		"PG TIME TABLE":    true,
		"PG TIME TABLE 1":  true,
		"PG TIME TABLE1":   true,
	}

	classes := make(map[string]map[int]string)
	for _, sheet := range sheets {
		// Skip excluded categories
		if excludedCategories[sheet] {
			continue
		}

		temp := make(map[int]string)
		rows, err := f.GetRows(sheet)
		for i, d := range rows {
			if i == 4 {
				for j, k := range d {
					if k != "" && k != "DAY" && k != "HOURS" && k != "SR NO" && k != "SR.NO" && k != "TUTORIAL" {
						// Add bounds checking for column number (Excel supports 1-16384)
						colNum := j + 1
						if colNum >= 1 && colNum <= 16384 {
							temp[colNum] = k
						}
					}
				}
			}
		}
		classes[sheet] = temp
		HandleError(err)
	}
	ExcelToJson(classes, f)
}

func ExcelToJson(classes map[string]map[int]string, f *excelize.File) {
	file, err := os.OpenFile("./data.json", os.O_TRUNC|os.O_WRONLY, os.ModeAppend)
	HandleError(err)
	defer file.Close()
	data := make(map[string]map[string][][]Data)
	for i, d := range classes {
		temp := make(map[string][][]Data)
		for j, k := range d {
			tc := GetTableData(i, j, f)
			temp[strings.Trim(k, " ")] = tc
		}
		data[strings.Trim(i, " ")] = temp
	}
	dj, _ := json.MarshalIndent(data, "", "	")
	_, err = file.Write(dj)
	HandleError(err)
}

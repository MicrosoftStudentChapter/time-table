package utils

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

var excludedSheets = map[string]bool{
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


func GenerateJson() {
	f, err := excelize.OpenFile("timetable1.xlsx")
	if err != nil {
		log.Printf("[ERROR] GenerateJson: cannot open timetable1.xlsx: %v", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("[ERROR] GenerateJson: error closing workbook: %v", err)
		}
	}()

	sheets := f.GetSheetList()
	data := make(map[string]map[string][][]Data)

	for _, sheet := range sheets {
		if excludedSheets[sheet] {
			log.Printf("[INFO] Skipping excluded sheet %q", sheet)
			continue
		}


		grid, err := FlattenSheet(f, sheet)
		if err != nil {
			log.Printf("[WARN] Skipping sheet %q: %v", sheet, err)
			continue
		}

		bounds, err := DetectGridBounds(grid, sheet)
		if err != nil {
			log.Printf("[WARN] Skipping sheet %q: %v", sheet, err)
			continue
		}

		log.Printf("[INFO] Sheet %q: headerRow=%d, dataRows=%d–%d, dayCol=%d, timeCol=%d, %d class(es)",
			sheet, bounds.HeaderRow, bounds.DataStartRow, bounds.DataEndRow, bounds.DayCol, bounds.TimeCol, len(bounds.ClassColumns))

		classData := make(map[string][][]Data)
		for col, className := range bounds.ClassColumns {
			className = strings.TrimSpace(className)
			if className == "" {
				continue
			}
			table := GetTableData(grid, bounds, col, sheet)
			classData[className] = table
		}

		if len(classData) > 0 {
			data[strings.TrimSpace(sheet)] = classData
		}
	}

	ExcelToJson(data)
}

func ExcelToJson(data map[string]map[string][][]Data) {
	file, err := os.OpenFile("./data.json", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[ERROR] ExcelToJson: cannot open data.json for writing: %v", err)
		return
	}
	defer file.Close()

	dj, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		log.Printf("[ERROR] ExcelToJson: JSON marshal failed: %v", err)
		return
	}

	_, err = file.Write(dj)
	if err != nil {
		log.Printf("[ERROR] ExcelToJson: write failed: %v", err)
	}
}

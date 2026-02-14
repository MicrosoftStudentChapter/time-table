package utils

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Data struct {
	Course string `json:"course"`
	Color  string `json:"color"`
}

var (
	// Matches standard course codes (UCS675, UMA023) and non-standard ones (UCSXX1).
	// The 5-7 letter variant avoids false-matching room names (LAB1, APC5, RSF1).
	reCourseCode = regexp.MustCompile(`[A-Z]{2,4}\d{2,4}|[A-Z]{5,7}\d{1,3}`)
	// Suffix L/T/P must be followed by whitespace or end-of-string, so room codes
	// like "TA3" aren't mistaken for a suffix "T" (from "LAB1 TA3").
	reTypeSuffix = regexp.MustCompile(`(?:[A-Z]{2,4}\d{2,4}|[A-Z]{5,7}\d{1,3})\s?([LTP])(?:\s|$)`)
	reElective   = regexp.MustCompile(`[A-Z]{2,4}\d{2,4}(?:/[A-Z]{2,4}\d{2,4})+`)

	// Matches professor initials in second sub-rows: "ABJ", "PK", "HJS/SCB", "DKA-RA"
	reProfPattern = regexp.MustCompile(`^[A-Z]{2,4}(-[A-Z]+)?(/[A-Z]{2,4}(-[A-Z]+)?)*$`)

	// Matches standalone room/location codes (with optional hyphen): "APC-5", "RF4", "LC-1", "G312", "AP-C3"
	reRoomStandalone = regexp.MustCompile(`^(LP|LT|LC|TA|BC|CC|CD|APC|AP|PL|VL|GC|RSF|RF|RA|G|C|L)-?[A-Z]?\d{1,4}$`)

	headerAnchors = map[string]bool{
		"day": true, "hours": true, "hour": true,
	}

	reTime = regexp.MustCompile(`(?i)\d{1,2}:\d{2}\s*(AM|PM|am|pm)?`)
)

var dayofweek = []string{
	"Timings",
	"Monday",
	"Tuesday",
	"Wednesday",
	"Thursday",
	"Friday",
}

func FlattenSheet(f *excelize.File, sheet string) ([][]string, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("FlattenSheet GetRows: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("FlattenSheet: sheet %q is empty", sheet)
	}

	maxCols := 0
	for _, r := range rows {
		if len(r) > maxCols {
			maxCols = len(r)
		}
	}

	grid := make([][]string, len(rows))
	for i, r := range rows {
		grid[i] = make([]string, maxCols)
		copy(grid[i], r)
	}

	mergedCells, err := f.GetMergeCells(sheet)
	if err != nil {
		log.Printf("[WARN] FlattenSheet: could not get merge cells for sheet %q: %v", sheet, err)
		return grid, nil
	}

	for _, mc := range mergedCells {
		startCell := mc.GetStartAxis()
		endCell := mc.GetEndAxis()
		value := mc.GetCellValue()

		startCol, startRow, err1 := excelize.CellNameToCoordinates(startCell)
		endCol, endRow, err2 := excelize.CellNameToCoordinates(endCell)
		if err1 != nil || err2 != nil {
			log.Printf("[WARN] FlattenSheet: bad merge range %s:%s — %v / %v", startCell, endCell, err1, err2)
			continue
		}

		for r := startRow - 1; r <= endRow-1 && r < len(grid); r++ {
			for c := startCol - 1; c <= endCol-1 && c < maxCols; c++ {
				if c < len(grid[r]) {
					grid[r][c] = value
				}
			}
		}
	}

	return grid, nil
}

type GridBounds struct {
	HeaderRow    int
	DataStartRow int
	DataEndRow   int
	DayCol       int
	TimeCol      int
	ClassColumns map[int]string
}

func DetectGridBounds(grid [][]string, sheet string) (*GridBounds, error) {
	gb := &GridBounds{
		HeaderRow:    -1,
		DataStartRow: -1,
		DayCol:       -1,
		TimeCol:      -1,
		ClassColumns: make(map[int]string),
	}

	for r := 0; r < len(grid); r++ {
		for c := 0; c < len(grid[r]); c++ {
			val := strings.TrimSpace(strings.ToLower(grid[r][c]))
			if val == "day" || val == "days" {
				gb.DayCol = c
				gb.HeaderRow = r
			}
			if val == "hours" || val == "hour" || val == "timings" || val == "timing" || val == "time" {
				gb.TimeCol = c
				if gb.HeaderRow == -1 {
					gb.HeaderRow = r
				}
			}
		}
		if gb.HeaderRow != -1 {
			break
		}
	}

	if gb.HeaderRow == -1 {
		return nil, fmt.Errorf("DetectGridBounds: no header row with 'DAY'/'HOURS' found in sheet %q", sheet)
	}

	if gb.TimeCol == -1 {
		for c := 0; c < len(grid[gb.HeaderRow]); c++ {
			for r := gb.HeaderRow + 1; r < len(grid) && r <= gb.HeaderRow+5; r++ {
				if c < len(grid[r]) && reTime.MatchString(grid[r][c]) {
					gb.TimeCol = c
					break
				}
			}
			if gb.TimeCol != -1 {
				break
			}
		}
	}
	if gb.TimeCol == -1 {
		log.Printf("[WARN] DetectGridBounds: could not find time column in sheet %q, falling back to DayCol+1", sheet)
		gb.TimeCol = gb.DayCol + 1
	}
	if gb.DayCol == -1 {
		gb.DayCol = 0
	}

	for r := gb.HeaderRow + 1; r < len(grid); r++ {
		if gb.TimeCol < len(grid[r]) && reTime.MatchString(grid[r][gb.TimeCol]) {
			gb.DataStartRow = r
			break
		}
	}
	if gb.DataStartRow == -1 {
		gb.DataStartRow = gb.HeaderRow + 1
		log.Printf("[WARN] DetectGridBounds: no time value found below header in sheet %q, using row %d", sheet, gb.DataStartRow)
	}

	gb.DataEndRow = gb.DataStartRow
	for r := gb.DataStartRow; r < len(grid); r++ {
		hasContent := false
		if gb.TimeCol < len(grid[r]) && strings.TrimSpace(grid[r][gb.TimeCol]) != "" {
			hasContent = true
		}
		if !hasContent {
			for c := gb.TimeCol + 1; c < len(grid[r]) && c <= gb.TimeCol+20; c++ {
				if strings.TrimSpace(grid[r][c]) != "" {
					hasContent = true
					break
				}
			}
		}
		if hasContent {
			gb.DataEndRow = r
		} else {
			if r > gb.DataEndRow+3 {
				break
			}
		}
	}

	skipLabels := map[string]bool{
		"day": true, "days": true, "hours": true, "hour": true,
		"sr no": true, "sr.no": true, "sr.no.": true, "s.no": true, "s. no": true,
		"tutorial": true, "lecture": true, "practical": true,
		"timings": true, "timing": true, "time": true,
		"": true,
	}

	for c := 0; c < len(grid[gb.HeaderRow]); c++ {
		val := strings.TrimSpace(grid[gb.HeaderRow][c])
		low := strings.ToLower(val)
		if !skipLabels[low] && c != gb.DayCol && c != gb.TimeCol {
			gb.ClassColumns[c] = val
		}
	}

	for delta := 1; delta <= 2; delta++ {
		r := gb.HeaderRow - delta
		if r < 0 {
			break
		}
		for c := 0; c < len(grid[r]); c++ {
			val := strings.TrimSpace(grid[r][c])
			low := strings.ToLower(val)
			if !skipLabels[low] && c != gb.DayCol && c != gb.TimeCol {
				existing, hasExisting := gb.ClassColumns[c]
				if !hasExisting || len(val) > len(existing) {
					gb.ClassColumns[c] = val
				}
			}
		}
	}

	if len(gb.ClassColumns) == 0 {
		return nil, fmt.Errorf("DetectGridBounds: no class columns found near header row %d in sheet %q", gb.HeaderRow, sheet)
	}

	return gb, nil
}

func ExtractCellData(raw, sheet, cellRef string) Data {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Data{Course: "", Color: "success"} // free period
	}

	color := ""
	isElective := reElective.MatchString(trimmed)
	suffixMatch := reTypeSuffix.FindStringSubmatch(trimmed)

	if isElective {
		color = "info"
	} else if len(suffixMatch) >= 2 {
		switch suffixMatch[1] {
		case "L":
			color = "danger"
		case "T":
			color = "primary"
		case "P":
			color = "warning"
		}
	}

	codes := reCourseCode.FindAllString(trimmed, -1)
	if len(codes) == 0 {
		// No course code found — this is second-sub-row supplementary data
		// (room codes, professor initials, lab names, descriptive text like
		// "BIO PROCESS/MICRO PROCESS/MOL BIO LAB", "CAPSTONE LECTURE F105/F106").
		// Ensure LAB-containing entries get the practical color if not already set.
		if strings.Contains(strings.ToUpper(trimmed), "LAB") && color == "" {
			color = "warning"
		}
		return Data{Course: trimmed, Color: color}
	}

	parts := []string{}

	beforeCode := trimmed
	firstCodeIdx := strings.Index(trimmed, codes[0])
	if firstCodeIdx > 0 {
		beforeCode = strings.TrimSpace(trimmed[:firstCodeIdx])
		if beforeCode != "" {
			parts = append(parts, beforeCode)
		}
	}

	for _, code := range codes {
		clean := strings.ReplaceAll(code, " ", "")
		resolved := GetSubjectName(clean)
		if resolved != "" {
			parts = append(parts, resolved)
		} else {
			parts = append(parts, clean)
		}
	}

	if len(suffixMatch) >= 2 {
		parts = append(parts, suffixMatch[1])
	}

	// Capture any text after all recognized patterns (course codes + type suffix).
	// The room/location code from the second Excel sub-row often ends up here.
	// e.g. "UPH013P G312" → after code UPH013 and suffix P, "G312" remains.
	codeLocations := reCourseCode.FindAllStringIndex(trimmed, -1)
	if len(codeLocations) > 0 {
		lastCodeEnd := codeLocations[len(codeLocations)-1][1]
		rest := trimmed[lastCodeEnd:]

		// Skip past the type suffix character (L/T/P) that may follow the last code.
		// It can be adjacent ("UPH013P") or with a space ("UPH013 P").
		// We only skip if the suffix char is at a word boundary (followed by space or end).
		skip := 0
		if len(rest) > 0 && rest[0] == ' ' {
			skip = 1
		}
		if skip < len(rest) && (rest[skip] == 'L' || rest[skip] == 'T' || rest[skip] == 'P') {
			if skip+1 >= len(rest) || rest[skip+1] == ' ' {
				skip++
			}
		}

		afterText := strings.TrimSpace(rest[skip:])
		if afterText != "" {
			parts = append(parts, afterText)
		}
	}

	course := strings.Join(parts, " ")
	return Data{Course: course, Color: color}
}

var timeValues = []string{
	"8:00am", "8:50am", "9:40am", "10:30am", "11:20am", "12:10pm",
	"1:00pm", "1:50pm", "2:40pm", "3:30pm", "4:20pm", "5:10pm",
	"6:00pm", "6:50pm",
}

func GetTableData(grid [][]string, bounds *GridBounds, classCol int, sheet string) [][]Data {

	header := make([]Data, len(dayofweek))
	for i, d := range dayofweek {
		header[i] = Data{Course: d, Color: "dark"}
	}

	numSlots := len(timeValues)
	type timeSlot struct {
		row  int
		time string
	}
	var slots []timeSlot
	for r := bounds.DataStartRow; r <= bounds.DataEndRow && r < len(grid); r++ {
		if bounds.TimeCol < len(grid[r]) {
			val := strings.TrimSpace(grid[r][bounds.TimeCol])
			if reTime.MatchString(val) {
				slots = append(slots, timeSlot{row: r, time: val})
			}
		}
	}

	type dayBlock struct {
		name     string
		startRow int
		endRow   int
	}

	dayNameMap := map[string]int{
		"monday": 0, "tuesday": 1, "wednesday": 2,
		"thursday": 3, "friday": 4, "saturday": 5, "sunday": 6,
	}

	var blocks []dayBlock
	currentDay := ""
	blockStart := bounds.DataStartRow

	for r := bounds.DataStartRow; r <= bounds.DataEndRow && r < len(grid); r++ {
		dayVal := ""
		if bounds.DayCol < len(grid[r]) {
			dayVal = strings.TrimSpace(strings.ToLower(grid[r][bounds.DayCol]))
		}

		if dayVal != "" && dayVal != currentDay {
			if currentDay != "" {
				blocks = append(blocks, dayBlock{name: currentDay, startRow: blockStart, endRow: r - 1})
			}
			currentDay = dayVal
			blockStart = r
		}
	}
	if currentDay != "" {
		blocks = append(blocks, dayBlock{name: currentDay, startRow: blockStart, endRow: bounds.DataEndRow})
	}

	type slotInDay struct {
		slotIdx int
		row     int
	}

	daySlots := make(map[int][]slotInDay)

	globalSlotIdx := 0
	for _, blk := range blocks {
		dayIdx, ok := dayNameMap[blk.name]
		if !ok || dayIdx >= 5 {
			continue
		}
		for _, s := range slots {
			if s.row >= blk.startRow && s.row <= blk.endRow && globalSlotIdx < numSlots {
				daySlots[dayIdx] = append(daySlots[dayIdx], slotInDay{slotIdx: globalSlotIdx, row: s.row})
			}
			if s.row >= blk.startRow && s.row <= blk.endRow {
				globalSlotIdx++
			}
		}
	}

	dayData := make([][]Data, 5)
	for di := 0; di < 5; di++ {
		dayData[di] = make([]Data, numSlots)
		for s := 0; s < numSlots; s++ {
			dayData[di][s] = Data{Course: "", Color: "success"}
		}
	}

	slotIdx := 0
	dayIdx := 0
	for r := bounds.DataStartRow; r <= bounds.DataEndRow && r < len(grid) && dayIdx < 5; r += 2 {
		cellContent := ""
		for sub := 0; sub < 2; sub++ {
			row := r + sub
			if row < len(grid) && classCol < len(grid[row]) {
				val := strings.TrimSpace(grid[row][classCol])
				if val != "" {
					if cellContent != "" && val != cellContent {
						cellContent += " " + val
					} else if cellContent == "" {
						cellContent = val
					}
				}
			}
		}

		cellRef := fmt.Sprintf("%s%d", colName(classCol), r+1)
		dayData[dayIdx][slotIdx] = ExtractCellData(cellContent, sheet, cellRef)
		slotIdx++

		if slotIdx >= numSlots {
			slotIdx = 0
			dayIdx++
		}
	}

	result := make([][]Data, numSlots+1)
	result[0] = header
	for s := 0; s < numSlots; s++ {
		row := make([]Data, len(dayofweek))
		row[0] = Data{Course: timeValues[s], Color: "dark"}
		for di := 0; di < 5 && di+1 < len(row); di++ {
			row[di+1] = dayData[di][s]
		}
		result[s+1] = row
	}

	return result
}

func colName(col int) string {
	name, err := excelize.ColumnNumberToName(col + 1)
	if err != nil {
		return fmt.Sprintf("COL%d", col)
	}
	return name
}

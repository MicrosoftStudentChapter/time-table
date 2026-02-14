package utils

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
)


var subjectCodeToName map[string]string

var reRoomCode = regexp.MustCompile(`^(LP|LT|TA|BC|CC|CD|APC|PL|VL|GC)\d{1,4}$`)

var reIsCourseCode = regexp.MustCompile(`^[A-Z]{2,4}\d{2,4}$`)

func init() {
	subjectCodeToName = make(map[string]string)
	file, err := os.Open("./subjects.json")
	if err != nil {
		return
	}
	defer file.Close()

	type subjectEntry struct {
		Name string `json:"name"`
	}
	raw := make(map[string]subjectEntry)
	bytes, err := io.ReadAll(file)
	if err != nil {
		return
	}
	json.Unmarshal(bytes, &raw)
	for code, entry := range raw {
		c := strings.TrimSpace(code)
		if entry.Name != "" {
			subjectCodeToName[c] = entry.Name
		}
	}
}

type ClassEntry struct {
	Time      string `json:"time"`
	Period    string `json:"period"`
	Title     string `json:"title"`
	Location  string `json:"location,omitempty"`
	Professor string `json:"professor,omitempty"`
	Type      string `json:"type,omitempty"`
	TypeColor string `json:"typeColor,omitempty"`
	IsBreak   bool   `json:"isBreak,omitempty"`
}

type MobileTimeTable map[string][]ClassEntry

var timeSlotRanges = []struct {
	Start  string
	End    string
	Period string
}{
	{"8:00", "8:50", "AM"},
	{"8:50", "9:40", "AM"},
	{"9:40", "10:30", "AM"},
	{"10:30", "11:20", "AM"},
	{"11:20", "12:10", "PM"},
	{"12:10", "1:00", "PM"},
	{"1:00", "1:50", "PM"},
	{"1:50", "2:40", "PM"},
	{"2:40", "3:30", "PM"},
	{"3:30", "4:20", "PM"},
	{"4:20", "5:10", "PM"},
	{"5:10", "6:00", "PM"},
	{"6:00", "6:50", "PM"},
	{"6:50", "7:40", "PM"},
}

var colorToType = map[string]struct {
	typeName  string
	typeColor string
}{
	"danger":  {"lecture", "#FF9800"},
	"primary": {"tutorial", "#E91E63"},
	"warning": {"practical", "#4CAF50"},
	"info":    {"elective", "#2196F3"},
}

func TransformToMobileFormat(table [][]Data) MobileTimeTable {
	result := MobileTimeTable{
		"monday":    {},
		"tuesday":   {},
		"wednesday": {},
		"thursday":  {},
		"friday":    {},
	}

	dayKeys := []string{"monday", "tuesday", "wednesday", "thursday", "friday"}

	for slotIdx := 0; slotIdx < len(timeSlotRanges) && slotIdx+1 < len(table); slotIdx++ {
		row := table[slotIdx+1]
		slot := timeSlotRanges[slotIdx]
		timeRange := slot.Start + "-" + slot.End

		for dayIdx, dayKey := range dayKeys {
			colIdx := dayIdx + 1
			if colIdx >= len(row) {
				continue
			}

			cell := row[colIdx]
			entry := buildClassEntry(cell, timeRange, slot.Period)
			if entry != nil {
				result[dayKey] = append(result[dayKey], *entry)
			}
		}
	}

	return result
}

func buildClassEntry(cell Data, timeRange, period string) *ClassEntry {
	course := strings.TrimSpace(cell.Course)

	if course == "" && cell.Color == "success" {
		return &ClassEntry{
			Time:    timeRange,
			Period:  period,
			Title:   "Break / Free Slot",
			IsBreak: true,
		}
	}

	if course == "" {
		return nil
	}

	location, title := parseCourseString(course)
	typeName := ""
	typeColor := ""
	if info, ok := colorToType[cell.Color]; ok {
		typeName = info.typeName
		typeColor = info.typeColor
	}

	return &ClassEntry{
		Time:      timeRange,
		Period:    period,
		Title:     title,
		Location:  location,
		Professor: "",
		Type:      typeName,
		TypeColor: typeColor,
	}
}

func parseCourseString(course string) (location, title string) {
	course = strings.TrimSpace(course)
	if course == "" {
		return "", ""
	}

	parts := strings.Fields(course)


	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if last == "L" || last == "T" || last == "P" {
			parts = parts[:len(parts)-1]
		}
	}

	if len(parts) == 0 {
		return "", course
	}

	
	if len(parts) >= 2 {
		candidate := parts[len(parts)-1]
		if reRoomCode.MatchString(candidate) {
			location = candidate
			parts = parts[:len(parts)-1]
		}
	}

	for i, token := range parts {
		if reIsCourseCode.MatchString(token) {
			if name, ok := subjectCodeToName[token]; ok {
				parts[i] = name
			}
		}
	}

	title = strings.Join(parts, " ")
	return location, title
}

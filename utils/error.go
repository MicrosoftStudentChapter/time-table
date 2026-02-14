package utils

import "log"

func HandleError(err error) bool {
	if err != nil {
		log.Printf("[ERROR] %v", err)
		return true
	}
	return false
}

func LogCellWarning(sheet, cell, rawText string) {
	log.Printf("[WARN] Sheet %q, Cell %s: unrecognized text %q — skipping regex extraction", sheet, cell, rawText)
}

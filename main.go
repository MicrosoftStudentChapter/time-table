package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"log"

	"github.com/MicrosoftStudentChapter/time-table/utils"
)

func init() {
	fmt.Println("Initializing server...")
	// UNCOMMENT THIS TO RE-GENERATE THE TIMETABLE
	// BE CAUTIOUS WHEN USING THIS
	// utils.GetSubjectMapping()
	// utils.GenerateJson()
	fmt.Println("Server initialized")
}

func main() {

	dataFile, err := os.Open("./data.json")
	if err != nil {
		log.Fatalf("Failed to open data.json: %v", err)
	}
	defer dataFile.Close()
	
	data := make(map[string]map[string][][]utils.Data)
	byteRes, err := io.ReadAll(dataFile)
	if err != nil {
		log.Fatalf("Failed to read data.json: %v", err)
	}
	
	err = json.Unmarshal([]byte(byteRes), &data)
	if err != nil {
		log.Fatalf("Failed to parse data.json: %v", err)
	}
	
	table, err := template.ParseFiles("./templates/table.html")
	if err != nil {
		log.Fatalf("Failed to parse table template: %v", err)
	}
	
	home, err := template.ParseFiles("./templates/home.html")
	if err != nil {
		log.Fatalf("Failed to parse home template: %v", err)
	}
	
	courseNameCode, err := template.ParseFiles("./templates/course-name-code.html")
	if err != nil {
		log.Fatalf("Failed to parse course-name-code template: %v", err)
	}
	
	errorPage, err := template.ParseFiles("./templates/error.html")
	if err != nil {
		log.Fatalf("Failed to parse error template: %v", err)
	}

	type HomeData struct {
		Sheets  []string
		Classes map[string][]string
	}

	var sheets []string
	for i := range data {
		sheets = append(sheets, strings.Trim(i, " "))
	}
	sort.StringSlice(sheets).Sort()
	classes := make(map[string][]string)
	for i, d := range data {
		temp := make([]string, 0)
		for j := range d {
			temp = append(temp, strings.Trim(j, " "))
		}
		sort.StringSlice(temp).Sort()
		classes[i] = temp
	}
	h := HomeData{
		Sheets:  sheets,
		Classes: classes,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			if err := errorPage.Execute(w, "This page is under construction !!(404)"); err != nil {
				log.Printf("Error while executing error template: %v", err)
			}
			return
		}
		if err := home.Execute(w, h); err != nil {
			log.Printf("Error while executing home template: %v", err)
		}
	})

	type TimeTableData struct {
		Data      [][]utils.Data
		ClassName string
	}

	http.HandleFunc("/timetable", func(w http.ResponseWriter, r *http.Request) {
		class := r.URL.Query().Get("classname")
		sheet := r.URL.Query().Get("sheet")

		flag := true
		for _, d := range classes[sheet] {
			if class == d {
				flag = false
			}
		}
		if flag {
			if err := errorPage.Execute(w, "Invalid category/class combination"); err != nil {
				log.Printf("Error while executing error template: %v", err)
			}
			return
		}
		data := TimeTableData{
			Data:      data[sheet][class],
			ClassName: class,
		}
		if err := table.Execute(w, data); err != nil {
			log.Printf("Error while executing table template: %v", err)
		}
	})

	// handler to serve add course page
	http.HandleFunc("/course", func(w http.ResponseWriter, r *http.Request) {
		if err := courseNameCode.Execute(w, h); err != nil {
			log.Printf("Error while executing course template: %v", err)
		}
	})

	fs := http.FileServer(http.Dir("assets/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	fmt.Println("Server Running at http://localhost:5000")
	utils.HandleError(http.ListenAndServe(":5000", nil))
}

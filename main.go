package main

import (
  "fmt"
  "net/http"
  "html/template"
  _ "github.com/mattn/go-sqlite3"
  "database/sql"
  "encoding/json"
)

type Page struct {
  Name string
  DBStatus bool
}

type SearchResult struct {
  Title string
  Author string
  Year string
  ID string
}

func main() {
  templates := template.Must(template.ParseFiles("templates/index.html"))

  db, _ := sql.Open("sqlite3", "dev.db")

  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    p := Page{Name: "Gopher", DBStatus: false}
    if name := r.FormValue("name"); name != "" {
      p.Name = name
    }

    p.DBStatus = db.Ping() == nil

    if err := templates.ExecuteTemplate(w, "index.html", p); err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
    }
  })

  http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
    results := []SearchResult{
      SearchResult{"Moby-Dick", "Herman Melville", "1851", "222222"},
      SearchResult{"The Adventures of Huckleberry Finn", "Mark Twain", "1884", "444444"},
      SearchResult{"The Catcher in the Rye", "JD Salinger", "1951", "333333"},
    }

    encoder := json.NewEncoder(w)
    if err := encoder.Encode(results); err != nil {
      http.Error(w, err.Error(), http.StatusInternalServerError)
    }
  })

  fmt.Println(http.ListenAndServe(":8080", nil))
}

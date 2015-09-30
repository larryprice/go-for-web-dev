package main

import (
  "fmt"
  "net/http"
)

func main() {
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
  	fmt.Fprintf(w, "Hello, Go Web Development\n")
  })

  fmt.Println(http.ListenAndServe(":8080", nil))
}

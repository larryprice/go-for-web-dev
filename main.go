package main

import (
	"net/http"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/gorp.v1"

	"encoding/json"
	"encoding/xml"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"net/url"
	"strconv"

	"github.com/codegangsta/negroni"
	"github.com/goincremental/negroni-sessions"
	"github.com/goincremental/negroni-sessions/cookiestore"
	gmux "github.com/gorilla/mux"
	"github.com/yosssi/ace"
)

type Book struct {
	PK             int64  `db:"pk"`
	Title          string `db:"title"`
	Author         string `db:"author"`
	Classification string `db:"classification"`
	ID             string `db:"id"`
	User           string `db:"user"`
}

type User struct {
	Username string `db:"username"`
	Secret   []byte `db:"secret"`
}

type Page struct {
	Books  []Book
	Filter string
	User   string
}

type SearchResult struct {
	Title  string `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year   string `xml:"hyr,attr"`
	ID     string `xml:"owi,attr"`
}

var db *sql.DB
var dbmap *gorp.DbMap

func initDb() {
	db, _ = sql.Open("sqlite3", "dev.db")

	dbmap = &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	dbmap.AddTableWithName(Book{}, "books").SetKeys(true, "pk")
	dbmap.AddTableWithName(User{}, "users").SetKeys(false, "username")
	dbmap.CreateTablesIfNotExists()
}

func verifyDatabase(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if err := db.Ping(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	next(w, r)
}

func getBookCollection(books *[]Book, sortCol, filterByClass, username string, w http.ResponseWriter) bool {
	if sortCol == "" {
		sortCol = "pk"
	}
	where := " where user=?"
	if filterByClass == "fiction" {
		where += " and classification between '800' and '900'"
	} else if filterByClass == "nonfiction" {
		where += " and classification not between '800' and '900'"
	}
	if _, err := dbmap.Select(books, "select * from books"+where+" order by "+sortCol, username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	return true
}

func getStringFromSession(r *http.Request, key string) string {
	var strVal string
	if val := sessions.GetSession(r).Get(key); val != nil {
		strVal = val.(string)
	}
	return strVal
}

func verifyUser(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.URL.Path == "/login" {
		next(w, r)
		return
	}

	if username := getStringFromSession(r, "User"); username != "" {
		if user, _ := dbmap.Get(User{}, username); user != nil {
			next(w, r)
			return
		}
	}
	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

type LoginPage struct {
	Error string
}

func main() {
	initDb()

	mux := gmux.NewRouter()

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		var p LoginPage
		if r.FormValue("register") != "" {
			secret, _ := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), bcrypt.DefaultCost)
			user := User{r.FormValue("username"), secret}
			if err := dbmap.Insert(&user); err != nil {
				p.Error = err.Error()
			} else {
				sessions.GetSession(r).Set("User", user.Username)
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		} else if r.FormValue("login") != "" {
			user, err := dbmap.Get(User{}, r.FormValue("username"))
			if err != nil {
				p.Error = err.Error()
			} else if user == nil {
				p.Error = "No such user found with Username: " + r.FormValue("username")
			} else {
				u := user.(*User)
				if err = bcrypt.CompareHashAndPassword(u.Secret, []byte(r.FormValue("password"))); err != nil {
					p.Error = err.Error()
				} else {
					sessions.GetSession(r).Set("User", u.Username)
					http.Redirect(w, r, "/", http.StatusFound)
					return
				}
			}
		}

		template, err := ace.Load("templates/login", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = template.Execute(w, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		sessions.GetSession(r).Set("User", nil)
		sessions.GetSession(r).Set("Filter", nil)

		http.Redirect(w, r, "/login", http.StatusFound)
	})

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var b []Book
		if !getBookCollection(&b, getStringFromSession(r, "SortBy"), r.FormValue("filter"),
			getStringFromSession(r, "User"), w) {
			return
		}

		sessions.GetSession(r).Set("Filter", r.FormValue("filter"))

		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("GET").Queries("filter", "{filter:all|fiction|nonfiction}")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var b []Book
		if !getBookCollection(&b, r.FormValue("sortBy"), getStringFromSession(r, "Filter"),
			getStringFromSession(r, "User"), w) {
			return
		}

		sessions.GetSession(r).Set("SortBy", r.FormValue("sortBy"))

		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}).Methods("GET").Queries("sortBy", "{sortBy:title|author|classification}")

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		template, err := ace.Load("templates/index", "", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		p := Page{Books: []Book{}, Filter: getStringFromSession(r, "Filter"), User: getStringFromSession(r, "User")}
		if !getBookCollection(&p.Books, getStringFromSession(r, "SortBy"),
			getStringFromSession(r, "Filter"), p.User, w) {
			return
		}

		if err = template.Execute(w, p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []SearchResult
		var err error

		if results, err = search(r.FormValue("search")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("POST")

	mux.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		var book ClassifyBookResponse
		var err error

		if book, err = find(r.FormValue("id")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		b := Book{
			PK:             -1,
			Title:          book.BookData.Title,
			Author:         book.BookData.Author,
			Classification: book.Classification.MostPopular,
			ID:             r.FormValue("id"),
			User:           getStringFromSession(r, "User"),
		}
		if err = dbmap.Insert(&b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("PUT")

	mux.HandleFunc("/books/{pk}", func(w http.ResponseWriter, r *http.Request) {
		pk, _ := strconv.ParseInt(gmux.Vars(r)["pk"], 10, 64)
		var b Book
		if err := dbmap.SelectOne(&b, "select * from books where pk=? and user=?",
			pk, getStringFromSession(r, "User")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		if _, err := dbmap.Delete(&b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}).Methods("DELETE")

	n := negroni.Classic()
	n.Use(sessions.Sessions("go-for-web-dev", cookiestore.New([]byte("my-secret-123"))))
	n.Use(negroni.HandlerFunc(verifyDatabase))
	n.Use(negroni.HandlerFunc(verifyUser))
	n.UseHandler(mux)
	n.Run(":8080")
}

type ClassifySearchResponse struct {
	Results []SearchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title  string `xml:"title,attr"`
		Author string `xml:"author,attr"`
		ID     string `xml:"owi,attr"`
	} `xml:"work"`
	Classification struct {
		MostPopular string `xml:"sfa,attr"`
	} `xml:"recommendations>ddc>mostPopular"`
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&owi=" + url.QueryEscape(id))

	if err != nil {
		return ClassifyBookResponse{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c, err
}

func search(query string) ([]SearchResult, error) {
	var c ClassifySearchResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&title=" + url.QueryEscape(query))

	if err != nil {
		return []SearchResult{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c.Results, err
}

func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

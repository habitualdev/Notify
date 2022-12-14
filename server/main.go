package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type application struct {
	auth struct {
		username string
		password string
	}
}

type Endpoints map[string]Endpoint
type NotificationStore struct {
	Endpoints
	Mux sync.Mutex
}
type Endpoint struct {
	Notifications []Notification
}

type Config struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	TimeOff  string `yaml:"timeoff"`
}

func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

var MainStore = NotificationStore{}

func main() {
	app := new(application)
	MainStore.Endpoints = make(map[string]Endpoint)
	configPath := flag.String("config", "notify-config.yaml", "Path to config file")
	flag.Parse()

	config, err := NewConfig(*configPath)

	fmt.Println(config)

	if err != nil {
		log.Fatal(err)
	}

	app.auth.username = config.Username
	app.auth.password = config.Password

	if app.auth.username == "" {
		log.Fatal("basic auth username must be provided")
	}

	if app.auth.password == "" {
		log.Fatal("basic auth password must be provided")
	}

	coolOffDuration, err := time.ParseDuration(config.TimeOff)
	fmt.Println(coolOffDuration)
	if err != nil {
		log.Fatal("timeoff must be provided in the correct format (e.g. 1h30m)")
	}

	go NotificationCleanup(coolOffDuration)

	mux := http.NewServeMux()

	mux.HandleFunc("/", app.basicAuth(app.UriHandler))

	srv := &http.Server{
		Addr:         ":4000",
		Handler:      mux,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("starting server on %s", srv.Addr)
	err = srv.ListenAndServe()
	log.Fatal(err)
}

func (app *application) UriHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		notifications := GetHandler(r.RequestURI)
		jsonData, _ := json.Marshal(notifications)
		fmt.Fprintf(w, "%s", string(jsonData))

	} else if r.Method == "POST" {
		fmt.Fprintf(w, "%v", PostHandler(r.RequestURI, *r))
	} else {
		fmt.Fprintf(w, "Unsupported method : %s", r.Method)
	}
}

func (app *application) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))
			expectedUsernameHash := sha256.Sum256([]byte(app.auth.username))
			expectedPasswordHash := sha256.Sum256([]byte(app.auth.password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

func GetHandler(uri string) Notifications {
	notifications := Notifications{}
	MainStore.Mux.Lock()
	for k, n := range MainStore.Endpoints {
		if strings.Split(k, "?")[0] == strings.Split(uri, "?")[0] {
			notifications = n.Notifications
			MainStore.Mux.Unlock()
			return notifications
		}
	}
	MainStore.Mux.Unlock()
	return notifications
}

func PostHandler(uri string, request http.Request) string {
	notifications := Notifications{}
	if request.FormValue("title") == "" {
		return "Title is required"
	}
	if request.FormValue("content") == "" {
		return "Content is required"
	}
	MainStore.Mux.Lock()
	println("Locking")
	for y, n := range MainStore.Endpoints {
		if y == strings.Split(uri, "?")[0] {
			notifications = n.Notifications
			checksum := sha256.New()
			checksum.Write([]byte(request.FormValue("title") + request.FormValue("content") + time.Now().Format(time.RFC3339)))
			checksumHex := fmt.Sprintf("%x", checksum.Sum(nil))
			notifications = append(notifications, Notification{Title: request.FormValue("title"),
				Content: request.FormValue("content"), Checksum: checksumHex, PostTime: time.Now().Format(time.RFC3339)})
			MainStore.Endpoints[y] = Endpoint{notifications}
			MainStore.Mux.Unlock()
			return "Success"
		}
	}

	MainStore.Endpoints[strings.Split(uri, "?")[0]] = Endpoint{Notifications{Notification{
		Title:    request.FormValue("title"),
		Content:  request.FormValue("content"),
		PostTime: time.Now().Format(time.RFC3339),
	}}}
	MainStore.Mux.Unlock()
	println("unlocked")
	return "Success"

}

func NotificationCleanup(coolOff time.Duration) {
	for {
		time.Sleep(10 * time.Second)
		MainStore.Mux.Lock()
		println("Locking - cleanup")
		for k, n := range MainStore.Endpoints {
			tempNotifications := Endpoint{}
			for i, v := range n.Notifications {
				parsedTime, err := time.Parse(time.RFC3339, v.PostTime)
				if err != nil {
					log.Fatal(err)
				}
				if time.Since(parsedTime) > coolOff {
					println(time.Now().Format(time.RFC3339))
					println(parsedTime.String())
					println(v.PostTime)
					fmt.Println(time.Since(parsedTime))
					continue
				} else {
					tempNotifications.Notifications = append(tempNotifications.Notifications, n.Notifications[i])

				}
			}
			MainStore.Endpoints[k] = tempNotifications
		}
		println("Unlocking - cleanup")
		MainStore.Mux.Unlock()
	}
}

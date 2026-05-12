package main

import (
	"crypto/subtle"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"
)

var tpl *template.Template
var store *sessions.CookieStore

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables.")
	}
	tpl = template.Must(template.ParseGlob("templates/*.gohtml"))

	sessionKey := os.Getenv("SESSION_SECRET")
	if sessionKey == "" {
		log.Fatal("SESSION_SECRET environment variable is required")
	}
	store = sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   true,
	}
}

func main() {
	// Public routes
	http.HandleFunc("/", index)
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)

	// Protected routes - require authentication
	http.Handle("/dashboard", checkAuth(http.HandlerFunc(dashboard)))

	// API routes - all protected
	http.Handle("/api/", checkAuth(http.StripPrefix("/api", apiHandler())))

	log.Println("app started...")
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// apiHandler returns a handler for all API routes
func apiHandler() http.Handler {
	mux := http.NewServeMux()
	// Register API endpoints here
	mux.HandleFunc("/doorbell", doorbell)

	return mux
}

func checkAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "door-opener-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		// Check if user is authenticated
		auth, ok := session.Values["authenticated"].(bool)
		if !ok || !auth {
			// For API endpoints, return 401 Unauthorized
			if r.Header.Get("Content-Type") == "application/json" || r.Header.Get("Accept") == "application/json" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// For web pages, redirect to login page
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func index(w http.ResponseWriter, r *http.Request) {
	if err := tpl.ExecuteTemplate(w, "login.gohtml", nil); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	password := os.Getenv("APP_PASSWORD")
	if password == "" {
		http.Error(w, "Server configuration error: APP_PASSWORD not set", http.StatusInternalServerError)
		return
	}

	input := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(input), []byte(password)) == 1 {
		session, err := store.Get(r, "door-opener-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		session.Values["authenticated"] = true
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if err := tpl.ExecuteTemplate(w, "login.gohtml", map[string]string{"Error": "Invalid password"}); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	if err := tpl.ExecuteTemplate(w, "dashboard.gohtml", nil); err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "door-opener-session")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	session.Values["authenticated"] = false
	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func doorbell(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Doorbell pressed! Making GET request to Shelly...")

	shellyIP := os.Getenv("SHELLY_IP")
	if shellyIP == "" {
		log.Println("SHELLY_IP environment variable not set. Cannot make request.")
		http.Error(w, "Server configuration error: SHELLY_IP not set", http.StatusInternalServerError)
		return
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://" + shellyIP + "/relay/0?turn=on")
	if err != nil {
		log.Printf("Error making request to Shelly: %v\n", err)
		http.Error(w, "Error triggering doorbell", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Shelly returned non-OK status: %d\n", resp.StatusCode)
		http.Error(w, "Error triggering doorbell: Shelly returned non-OK status", http.StatusInternalServerError)
		return
	}

	log.Println("Doorbell triggered successfully!")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Printf("Error writing response: %v\n", err)
	}
}

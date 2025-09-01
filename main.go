package main

import (
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

	// Initialize the session store with a secret key
	// In production, this should be a strong random key from environment variables
	sessionKey := os.Getenv("SESSION_SECRET")
	if sessionKey == "" {
		sessionKey = "default-session-secret-change-in-production"
		log.Println("Warning: Using default session secret. Set SESSION_SECRET env variable in production.")
	}
	store = sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
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
	tpl.ExecuteTemplate(w, "login.gohtml", nil)
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

	// In a real application, you'd hash and compare passwords securely.
	// For this example, we'll do a direct comparison.
	if r.FormValue("password") == password {
		// Set user as authenticated in the session
		session, err := store.Get(r, "door-opener-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		session.Values["authenticated"] = true
		err = session.Save(r, w)
		if err != nil {
			http.Error(w, "Failed to save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	http.Error(w, "Invalid credentials", http.StatusUnauthorized)
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	// In a real application, you'd check for a valid session here.
	tpl.ExecuteTemplate(w, "dashboard.gohtml", nil)
}

func logout(w http.ResponseWriter, r *http.Request) {
	session, err := store.Get(r, "door-opener-session")
	if err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Clear the session
	session.Values["authenticated"] = false
	session.Options.MaxAge = -1 // Delete the cookie

	err = session.Save(r, w)
	if err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func doorbell(w http.ResponseWriter, r *http.Request) {
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
	w.Write([]byte("OK"))
}

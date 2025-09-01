package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var tpl *template.Template

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables.")
	}
	tpl = template.Must(template.ParseGlob("templates/*.gohtml"))
}

func main() {
	http.HandleFunc("/", index)
	http.HandleFunc("/login", login)
	http.HandleFunc("/dashboard", dashboard)
	http.HandleFunc("/doorbell", doorbell)
	log.Println("app started...")
	log.Fatal(http.ListenAndServe(":8080", nil))
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
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	http.Error(w, "Invalid credentials", http.StatusUnauthorized)
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	// In a real application, you'd check for a valid session here.
	tpl.ExecuteTemplate(w, "dashboard.gohtml", nil)
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

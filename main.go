// Package main implements a simple web-based tower defense game server.
//
// Features:
// - user registration/login with bcrypt password hashing
// - save/load game state into SQLite
// - basic game lobby + routes for menu, tutorial, and gameplay
// - game over scoreboard page
package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// Sql Lite Database Connection
var db *sql.DB

// initDB sets up the SQLite database connection and required tables.
// This function is called once at application startup.
func initDB() {
	// Make Connection to Database
	var err error
	db, err = sql.Open("sqlite3", "accounts.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	// Create Users Table
	stmt := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);`
	if _, err := db.Exec(stmt); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	// Create Saves Table
	stmt2 := `CREATE TABLE IF NOT EXISTS saves (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		wave INTEGER NOT NULL,
		lives INTEGER NOT NULL,
		gold INTEGER NOT NULL,
		seed INTEGER NOT NULL,
		towers TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);`
	if _, err := db.Exec(stmt2); err != nil {
		log.Fatalf("failed to initialize save table: %v", err)
	}
}

// getCurrentUser returns the username from session, or empty when unauthenticated.
func getCurrentUser(c *gin.Context) string {
	// Get session
	s := sessions.Default(c)
	// Get username
	user, ok := s.Get("user").(string)
	if !ok {
		return ""
	}
	return user
}

// isAllowedUsernameChar returns true if rune is allowed for usernames.
func isAllowedUsernameChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '_' || r == '-' || r == '.' {
		return true
	}
	return false
}

// validateUsername checks username constraints and returns an error message when invalid.
func validateUsername(username string) string {
	// Username can't be empty
	if username == "" {
		return "Username cannot be empty"
	}
	// Username can't be longer than 15 characters
	if len(username) > 15 {
		return "Username must be 15 characters or fewer"
	}

	// Userncame can't have whitespace
	for _, r := range username {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return "Username cannot contain whitespace"
		}
		if !isAllowedUsernameChar(r) {
			return "Username can only contain letters, numbers, underscore, dash, and dot"
		}
	}
	return ""
}

// validatePassword checks password constraints and returns an error message when invalid.
func validatePassword(password string) string {
	// Password can't be empty
	if password == "" {
		return "Password cannot be empty"
	}
	//Password can't be longer than 15 characters
	if len(password) > 15 {
		return "Password must be 15 characters or fewer"
	}
	// Password can't contain whitespace
	for _, r := range password {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return "Password cannot contain whitespace"
		}
	}
	return ""
}

// requireAuth check if user is logged in, if not, redirect to login page
func requireAuth() gin.HandlerFunc {
	// Get sesssion and check if there is a username stored
	return func(c *gin.Context) {
		if getCurrentUser(c) == "" {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// main Creates routes for all pages and functions of the website, also opens or creates the database
func main() {
	// Create database
	initDB()
	defer db.Close()

	// Create web-app
	r := gin.Default()
	r.Use(sessions.Sessions("castle_session", cookie.NewStore([]byte("super-secret-key"))))
	r.LoadHTMLGlob("templates/*") // HTML templates
	r.Static("/static", "./static") // Static resources, JS and CSS

	// Main menu
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "menu.html", gin.H{"User": getCurrentUser(c)})
	})

	// Registration endpoint: show form and handle new user creation.
	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{"Error": ""})
	})

	// Check and verify password and usernames before inserting into database
	r.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		// Ensure info is given
		if username == "" || password == "" {
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": "Username and password cannot be empty"})
			return
		}
		// validate username and password
		if errMsg := validateUsername(username); errMsg != "" {
			// If username validation fails, show specific validation message.
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": errMsg})
			return
		}

		if errMsg := validatePassword(password); errMsg != "" {
			// If password validation fails, show specific validation message.
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": errMsg})
			return
		}

		// Encrypt password
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "register.html", gin.H{"Error": "Failed to hash password"})
			return
		}

		// Insert into database
		_, err = db.Exec("INSERT INTO users(username, password_hash) VALUES(?, ?)", username, string(hash))
		if err != nil {
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": "Username already exists"})
			return
		}

		c.Redirect(http.StatusSeeOther, "/login")
	})

	// Login page
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"Error": ""})
	})

	// Check if given login info is valid
	r.POST("/login", func(c *gin.Context) {
		// Extract form values from POST request.
		username := c.PostForm("username")
		password := c.PostForm("password")

		// Fail early if required fields are missing.
		if username == "" || password == "" {
			c.HTML(http.StatusBadRequest, "login.html", gin.H{"Error": "Username and password cannot be empty"})
			return
		}

		// Get password from user
		var storedHash string
		err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&storedHash)
		if err != nil {
			if err == sql.ErrNoRows {
				c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid credentials"})
				return
			}
			c.HTML(http.StatusInternalServerError, "login.html", gin.H{"Error": "Database error"})
			return
		}

		// Check if password equals the stored hashed password
		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
			// Incorrect password provided (hash mismatch) - generic message for security.
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid credentials"})
			return
		}

		// Set the session equal to the user
		s := sessions.Default(c)
		s.Set("user", username)
		s.Save()
		c.Redirect(http.StatusSeeOther, "/")
	})

	// Logout of session
	r.GET("/logout", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Clear()
		s.Save()
		c.Redirect(http.StatusSeeOther, "/")
	})

	// Save users progress
	r.POST("/save", requireAuth(), func(c *gin.Context) {
		// The stats of the game when quit
		var payload struct {
			Wave   int    `json:"wave"`
			Lives  int    `json:"lives"`
			Gold   int    `json:"gold"`
			Seed   int    `json:"seed"`
			Towers string `json:"towers"`
		}
		if err := c.BindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		// Get user and save their stats
		user := getCurrentUser(c)
		stmt := `INSERT INTO saves(username, wave, lives, gold, seed, towers, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(username) DO UPDATE SET wave=excluded.wave, lives=excluded.lives, gold=excluded.gold, seed=excluded.seed, towers=excluded.towers, updated_at=excluded.updated_at`
		if _, err := db.Exec(stmt, user, payload.Wave, payload.Lives, payload.Gold, payload.Seed, payload.Towers, time.Now().UTC()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "saved"})
	})

	// Load current save
	r.GET("/load", requireAuth(), func(c *gin.Context) {
		// Get the user
		user := getCurrentUser(c)
		var wave, lives, gold, seed int
		var towers string
		// Get stats from user
		err := db.QueryRow("SELECT wave,lives,gold,seed,towers FROM saves WHERE username = ?", user).Scan(&wave, &lives, &gold, &seed, &towers)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "no save"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load save"})
			return
		}
		// Send stats to game
		c.JSON(http.StatusOK, gin.H{"wave": wave, "lives": lives, "gold": gold, "seed": seed, "towers": towers})
	})

	// Delete user save on new game
	r.POST("/delete-save", requireAuth(), func(c *gin.Context) {
		user := getCurrentUser(c)
		if _, err := db.Exec("DELETE FROM saves WHERE username = ?", user); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete save"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "deleted"})
	})

	r.GET("/tutorial", func(c *gin.Context) {
		c.HTML(http.StatusOK, "tutorial.html", gin.H{"User": getCurrentUser(c)})
	})

	r.GET("/game", requireAuth(), func(c *gin.Context) {
		c.HTML(http.StatusOK, "game.html", gin.H{"User": getCurrentUser(c)})
	})

	r.GET("/game-over", requireAuth(), func(c *gin.Context) {
		c.HTML(http.StatusOK, "game-over.html", gin.H{"User": getCurrentUser(c)})
	})

	r.GET("/quit", func(c *gin.Context) {
		c.String(http.StatusOK, "Goodbye!")
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

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

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "accounts.db")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	stmt := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);`
	if _, err := db.Exec(stmt); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

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

func getCurrentUser(c *gin.Context) string {
	s := sessions.Default(c)
	user, ok := s.Get("user").(string)
	if !ok {
		return ""
	}
	return user
}

func requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if getCurrentUser(c) == "" {
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

func main() {
	initDB()
	defer db.Close()

	r := gin.Default()
	r.Use(sessions.Sessions("castle_session", cookie.NewStore([]byte("super-secret-key"))))
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "menu.html", gin.H{"User": getCurrentUser(c)})
	})

	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", gin.H{"Error": ""})
	})

	r.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		if username == "" || password == "" {
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": "Username and password cannot be empty"})
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "register.html", gin.H{"Error": "Failed to hash password"})
			return
		}

		_, err = db.Exec("INSERT INTO users(username, password_hash) VALUES(?, ?)", username, string(hash))
		if err != nil {
			c.HTML(http.StatusBadRequest, "register.html", gin.H{"Error": "Username already exists"})
			return
		}

		c.Redirect(http.StatusSeeOther, "/login")
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"Error": ""})
	})

	r.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		if username == "" || password == "" {
			c.HTML(http.StatusBadRequest, "login.html", gin.H{"Error": "Username and password cannot be empty"})
			return
		}

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

		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "Invalid credentials"})
			return
		}

		s := sessions.Default(c)
		s.Set("user", username)
		s.Save()
		c.Redirect(http.StatusSeeOther, "/")
	})

	r.GET("/logout", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Clear()
		s.Save()
		c.Redirect(http.StatusSeeOther, "/")
	})

	r.POST("/save", requireAuth(), func(c *gin.Context) {
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

	r.GET("/load", requireAuth(), func(c *gin.Context) {
		user := getCurrentUser(c)
		var wave, lives, gold, seed int
		var towers string
		err := db.QueryRow("SELECT wave,lives,gold,seed,towers FROM saves WHERE username = ?", user).Scan(&wave, &lives, &gold, &seed, &towers)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "no save"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load save"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"wave": wave, "lives": lives, "gold": gold, "seed": seed, "towers": towers})
	})

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

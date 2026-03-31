package main

import (
	"database/sql"
	"log"
	"net/http"

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

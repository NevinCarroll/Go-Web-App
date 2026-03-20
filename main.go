package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

func main() {
  // Create a Gin router with default middleware (logger and recovery)
  r := gin.Default()
  r.LoadHTMLGlob("templates/*") // Load templates into memory
  // testing
  // Define a simple GET endpoint
  r.GET("/test", func(c *gin.Context) {
    // Return JSON response
    c.HTML(http.StatusOK, "test.html", gin.H{

    })
  })

  // Start server on port 8080 (default)
  // Server will listen on 0.0.0.0:8080 (localhost:8080 on Windows)
  if err := r.Run(); err != nil {
    log.Fatalf("failed to run server: %v", err)
  }
}
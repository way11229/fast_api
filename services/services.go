package services

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type UserFileServer struct {
	serverPort    string
	userFilesPath string

	router *gin.Engine
	server *http.Server
	wg     sync.WaitGroup
}

func NewUserFileServer(serverPort, userFilesPath string) *UserFileServer {
	// Ensure user files directory exists
	if err := os.MkdirAll(userFilesPath, os.ModePerm); err != nil {
		log.Fatalf("Failed to create user files directory: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	s := &UserFileServer{
		serverPort:    serverPort,
		userFilesPath: userFilesPath,

		router: router,
	}

	// Routes
	router.POST("/user/add", s.addUserFile)
	router.GET("/users", s.listUsers)
	router.GET("/user/:user_id", s.checkUserFile)

	return s
}

func (s *UserFileServer) addUserFile(c *gin.Context) {
	userID := c.PostForm("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}

	// Create filename with .json extension
	filename := fmt.Sprintf("%s.json", userID)
	filepath := filepath.Join(s.userFilesPath, filename)

	// Save the uploaded file
	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully"})
}

func (s *UserFileServer) listUsers(c *gin.Context) {
	files, err := os.ReadDir(s.userFilesPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read user files"})
		return
	}

	var userFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			userFiles = append(userFiles, file.Name())
		}
	}

	c.JSON(http.StatusOK, gin.H{"users": userFiles})
}

func (s *UserFileServer) checkUserFile(c *gin.Context) {
	userID := c.Param("user_id")
	filename := fmt.Sprintf("%s.json", userID)
	filepath := filepath.Join(s.userFilesPath, filename)

	_, err := os.Stat(filepath)
	exists := !os.IsNotExist(err)

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"exists":  exists,
	})
}

func (s *UserFileServer) Start() {
	s.server = &http.Server{
		Addr:    s.serverPort,
		Handler: s.router,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("Server starting on port %s", s.serverPort)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func (s *UserFileServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	s.wg.Wait()
	log.Println("Server stopped gracefully")
}

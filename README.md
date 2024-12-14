# fast_api

使用AI生成一個API server，並簡單整理結構及code review
使用Claude模型

## 請求

給我golang用gin當web server聽80 port，要graceful shutdown，post method路徑為user/add，使用者會傳入id後，會用使用者傳入的id檔檔名寫檔寫入檔案到一個路徑。還有兩隻API是get method，一隻路徑為/users，會回傳所有已存在的所有檔案;一隻路徑為/user/{user_id}，會回傳是否存在{user_id}.json這個檔案

## 原始碼

``` golang
package main

import (
 "context"
 "fmt"
 "io"
 "log"
 "net/http"
 "os"
 "path/filepath"
 "sync"
 "time"

 "github.com/gin-gonic/gin"
)

const (
 userFilesPath = "./user_files"
 serverPort    = ":80"
)

type UserFileServer struct {
 router *gin.Engine
 server *http.Server
 wg     sync.WaitGroup
}

func NewUserFileServer() *UserFileServer {
 // Ensure user files directory exists
 if err := os.MkdirAll(userFilesPath, os.ModePerm); err != nil {
  log.Fatalf("Failed to create user files directory: %v", err)
 }

 gin.SetMode(gin.ReleaseMode)
 router := gin.Default()

 s := &UserFileServer{
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
 filepath := filepath.Join(userFilesPath, filename)

 // Save the uploaded file
 if err := c.SaveUploadedFile(file, filepath); err != nil {
  c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
  return
 }

 c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully"})
}

func (s *UserFileServer) listUsers(c *gin.Context) {
 files, err := os.ReadDir(userFilesPath)
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
 filepath := filepath.Join(userFilesPath, filename)

 _, err := os.Stat(filepath)
 exists := !os.IsNotExist(err)

 c.JSON(http.StatusOK, gin.H{
  "user_id": userID,
  "exists":  exists,
 })
}

func (s *UserFileServer) Start() {
 s.server = &http.Server{
  Addr:    serverPort,
  Handler: s.router,
 }

 s.wg.Add(1)
 go func() {
  defer s.wg.Done()
  log.Printf("Server starting on port %s", serverPort)
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

func main() {
 server := NewUserFileServer()
 server.Start()

 // Graceful shutdown handling
 quit := make(chan os.Signal, 1)
 // signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
 // <-- Uncomment and import "os/signal" if you want to use signal handling
 <-quit  // Block and wait for shutdown signal

 server.Stop()
}
```

## test

``` golang
package main

import (
 "bytes"
 "encoding/json"
 "fmt"
 "io"
 "mime/multipart"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"

 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/assert"
)

func setupTestServer() *UserFileServer {
 // Use a temporary directory for tests
 oldUserFilesPath := userFilesPath
 userFilesPath = fmt.Sprintf("%s_test", oldUserFilesPath)
 
 // Ensure the test directory is clean before each test
 os.RemoveAll(userFilesPath)
 os.MkdirAll(userFilesPath, os.ModePerm)

 return NewUserFileServer()
}

func cleanupTestServer() {
 os.RemoveAll(userFilesPath)
}

func TestAddUserFile(t *testing.T) {
 // Setup
 server := setupTestServer()
 defer cleanupTestServer()

 // Create a test file
 testContent := []byte(`{"test": "data"}`)
 testFilePath := filepath.Join(os.TempDir(), "test.json")
 err := os.WriteFile(testFilePath, testContent, 0644)
 assert.NoError(t, err)
 defer os.Remove(testFilePath)

 // Prepare multipart form data
 body := &bytes.Buffer{}
 writer := multipart.NewWriter(body)
 
 // Add ID field
 _ = writer.WriteField("id", "testuser")
 
 // Add file
 part, err := writer.CreateFormFile("file", "test.json")
 assert.NoError(t, err)
 _, err = part.Write(testContent)
 assert.NoError(t, err)
 
 writer.Close()

 // Create request
 w := httptest.NewRecorder()
 req, _ := http.NewRequest("POST", "/user/add", body)
 req.Header.Set("Content-Type", writer.FormDataContentType())
 server.router.ServeHTTP(w, req)

 // Assertions
 assert.Equal(t, http.StatusOK, w.Code)

 // Check if file was actually saved
 savedFilePath := filepath.Join(userFilesPath, "testuser.json")
 _, err = os.Stat(savedFilePath)
 assert.NoError(t, err, "File should be saved")

 // Verify file contents
 savedContent, err := os.ReadFile(savedFilePath)
 assert.NoError(t, err)
 assert.Equal(t, testContent, savedContent)
}

func TestAddUserFileWithMissingID(t *testing.T) {
 // Setup
 server := setupTestServer()
 defer cleanupTestServer()

 // Prepare multipart form data without ID
 body := &bytes.Buffer{}
 writer := multipart.NewWriter(body)
 
 // Add file without ID
 part, err := writer.CreateFormFile("file", "test.json")
 assert.NoError(t, err)
 part.Write([]byte(`{"test": "data"}`))
 
 writer.Close()

 // Create request
 w := httptest.NewRecorder()
 req, _ := http.NewRequest("POST", "/user/add", body)
 req.Header.Set("Content-Type", writer.FormDataContentType())
 server.router.ServeHTTP(w, req)

 // Assertions
 assert.Equal(t, http.StatusBadRequest, w.Code)

 var response map[string]string
 err = json.Unmarshal(w.Body.Bytes(), &response)
 assert.NoError(t, err)
 assert.Equal(t, "User ID is required", response["error"])
}

func TestListUsers(t *testing.T) {
 // Setup
 server := setupTestServer()
 defer cleanupTestServer()

 // Prepare test files
 testUsers := []string{"user1", "user2", "user3"}
 for _, user := range testUsers {
  filePath := filepath.Join(userFilesPath, fmt.Sprintf("%s.json", user))
  err := os.WriteFile(filePath, []byte(`{"user": "data"}`), 0644)
  assert.NoError(t, err)
 }

 // Create request
 w := httptest.NewRecorder()
 req, _ := http.NewRequest("GET", "/users", nil)
 server.router.ServeHTTP(w, req)

 // Assertions
 assert.Equal(t, http.StatusOK, w.Code)

 var response map[string][]string
 err := json.Unmarshal(w.Body.Bytes(), &response)
 assert.NoError(t, err)

 // Check if all test users are returned
 assert.Len(t, response["users"], 3)
 for _, user := range testUsers {
  assert.Contains(t, response["users"], fmt.Sprintf("%s.json", user))
 }
}

func TestCheckUserFile(t *testing.T) {
 // Subtests to check file existence
 t.Run("FileExists", func(t *testing.T) {
  // Setup
  server := setupTestServer()
  defer cleanupTestServer()

  // Create a test file
  testUser := "existinguser"
  filePath := filepath.Join(userFilesPath, fmt.Sprintf("%s.json", testUser))
  err := os.WriteFile(filePath, []byte(`{"user": "data"}`), 0644)
  assert.NoError(t, err)

  // Create request
  w := httptest.NewRecorder()
  req, _ := http.NewRequest("GET", fmt.Sprintf("/user/%s", testUser), nil)
  server.router.ServeHTTP(w, req)

  // Assertions
  assert.Equal(t, http.StatusOK, w.Code)

  var response map[string]interface{}
  err = json.Unmarshal(w.Body.Bytes(), &response)
  assert.NoError(t, err)
  assert.Equal(t, testUser, response["user_id"])
  assert.True(t, response["exists"].(bool))
 })

 t.Run("FileNotExists", func(t *testing.T) {
  // Setup
  server := setupTestServer()
  defer cleanupTestServer()

  // Create request for non-existent user
  testUser := "nonexistentuser"
  w := httptest.NewRecorder()
  req, _ := http.NewRequest("GET", fmt.Sprintf("/user/%s", testUser), nil)
  server.router.ServeHTTP(w, req)

  // Assertions
  assert.Equal(t, http.StatusOK, w.Code)

  var response map[string]interface{}
  err := json.Unmarshal(w.Body.Bytes(), &response)
  assert.NoError(t, err)
  assert.Equal(t, testUser, response["user_id"])
  assert.False(t, response["exists"].(bool))
 })
}

func TestMain(m *testing.M) {
 // Set Gin to Test Mode
 gin.SetMode(gin.TestMode)

 // Run tests
 os.Exit(m.Run())
}
```

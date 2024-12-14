package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const (
	USER_FILES_PATH = "./user_files"
	SERVER_PORT     = ":80"
)

var userFilesPath = USER_FILES_PATH

func setupTestServer() *UserFileServer {
	// Use a temporary directory for tests
	oldUserFilesPath := userFilesPath
	userFilesPath = fmt.Sprintf("%s_test", oldUserFilesPath)

	// Ensure the test directory is clean before each test
	os.RemoveAll(userFilesPath)
	os.MkdirAll(userFilesPath, os.ModePerm)

	return NewUserFileServer(SERVER_PORT, userFilesPath)
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

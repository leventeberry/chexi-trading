//go:build integration

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"goapi/container"
	"goapi/initializers"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/queue"
	httpserver "goapi/internal/transport/httpserver"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	testRouter    *gin.Engine
	testContainer *container.Container
	testDB        *gorm.DB
	userToken     string
	adminToken    string
	userID        string
	adminUserID   string
)

// Setup test environment
func TestMain(m *testing.M) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Deterministic 32-byte AES key for MFA integration tests (skipped when MFA tests not run).
	if os.Getenv("MFA_ENCRYPTION_KEY") == "" {
		k := make([]byte, 32)
		for i := range k {
			k[i] = byte(i + 1)
		}
		os.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(k))
	}
	if os.Getenv("WEBHOOK_ENCRYPTION_KEY") == "" {
		k := make([]byte, 32)
		for i := range k {
			k[i] = byte(i + 3)
		}
		os.Setenv("WEBHOOK_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(k))
	}

	// Initialize database (use test database if available)
	deps := initializers.Bootstrap()
	testDB = deps.DB

	jwtMgr := authinfra.NewManager(deps.Config.JWT.Secret, deps.Config.JWT.AccessTokenMinutes)
	recorder := events.NewPostgresRecorder(deps.DB)
	webhookSync := queue.WebhookDeliverWithoutWorker(deps.Config, deps.RedisClient)
	testContainer = container.NewContainer(deps.DB, deps.Cache, jwtMgr, recorder, deps.Config, deps.QueueEnqueue, deps.RedisQueue, webhookSync)

	testRouter = httpserver.NewEngine(deps.Cache, deps.Config, testContainer)

	// Run tests
	code := m.Run()

	// Cleanup if needed
	os.Exit(code)
}

// Helper function to make requests
func makeRequest(method, url string, body interface{}, token string) (*httptest.ResponseRecorder, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w, nil
}

// makeRequestAuth is like makeRequest but supports Bearer JWT and/or X-API-Key (for org API key tests).
func makeRequestAuth(method, url string, body interface{}, bearerToken, xAPIKey string) (*httptest.ResponseRecorder, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if xAPIKey != "" {
		req.Header.Set("X-API-Key", xAPIKey)
	}

	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w, nil
}

// verifyEmailForUser completes email verification using a synthetic raw token inserted into the DB
// (same approach as auth_verification_password_reset_test). Required when outbound email is enabled
// and registration no longer returns a JWT until the address is verified.
func verifyEmailForUser(t *testing.T, email string) {
	t.Helper()
	uid := mustUserIDByEmail(t, email)
	raw := fmt.Sprintf("integration-verify-%d", time.Now().UnixNano())
	replaceEmailVerificationToken(t, uid, raw, time.Now().UTC().Add(time.Hour))
	w, err := makeRequest("POST", "/api/v1/verify-email", map[string]interface{}{"token": raw}, "")
	if err != nil {
		t.Fatalf("verify-email request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("verify-email expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// registerThenAccessToken registers a user and returns a JWT: either from the response when email is disabled,
// or after synthetic verification + login when verification is required.
func registerThenAccessToken(t *testing.T, registerData map[string]interface{}) string {
	t.Helper()
	w, err := makeRequest("POST", "/api/v1/register", registerData, "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if w.Code != http.StatusCreated {
		t.Fatalf("register expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse register: %v", err)
	}
	if tok, ok := resp["token"].(map[string]interface{}); ok {
		jwt, _ := tok["jwt_token"].(string)
		if jwt == "" {
			t.Fatal("expected jwt_token when token object present")
		}
		return jwt
	}
	email := registerData["email"].(string)
	verifyEmailForUser(t, email)
	wLogin, err := makeRequest("POST", "/api/v1/login", map[string]interface{}{
		"email":    email,
		"password": registerData["password"],
	}, "")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d body=%s", wLogin.Code, wLogin.Body.String())
	}
	var loginResp map[string]interface{}
	if err := json.Unmarshal(wLogin.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("parse login: %v", err)
	}
	tokenObj, ok := loginResp["token"].(map[string]interface{})
	if !ok {
		t.Fatalf("login missing token: %#v", loginResp)
	}
	jwt, _ := tokenObj["jwt_token"].(string)
	if jwt == "" {
		t.Fatal("login jwt_token empty")
	}
	return jwt
}

// Helper function to extract user ID from user object (UUID string in JSON).
func getIDFromUser(user interface{}) string {
	userMap, ok := user.(map[string]interface{})
	if !ok {
		return ""
	}
	if v, ok := userMap["id"].(string); ok {
		return v
	}
	return ""
}

// Test 1: Health Check
func TestHealthCheck(t *testing.T) {
	w, err := makeRequest("GET", "/", nil, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["message"] != "Welcome!" {
		t.Errorf("Expected 'Welcome!', got '%v'", response["message"])
	}

	fmt.Println("✓ Health check test passed")
}

// Test 2: Register User
func TestRegisterUser(t *testing.T) {
	registerData := map[string]interface{}{
		"first_name":   "John",
		"last_name":    "Doe",
		"email":        "john.doe@test.com",
		"password":     "Password123!",
		"phone_number": "+1234567890",
		"role":         "user",
	}

	userToken = registerThenAccessToken(t, registerData)
	userID = mustUserIDByEmail(t, registerData["email"].(string))

	fmt.Println("✓ Register user test passed")
}

// Test 2b: Self-register as admin is rejected
func TestSelfRegisterAsAdminRejected(t *testing.T) {
	w, err := makeRequest("POST", "/api/v1/register", map[string]interface{}{
		"first_name": "Bad",
		"last_name":  "Actor",
		"email":      "wannabe-admin@test.com",
		"password":   "Password123!",
		"role":       "admin",
	}, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
	fmt.Println("✓ Self-register as admin rejected test passed")
}

// Test 3: Seed admin user (register as user, promote via SQL, login)
func TestRegisterAdmin(t *testing.T) {
	adminData := map[string]interface{}{
		"first_name":   "Admin",
		"last_name":    "User",
		"email":        "admin@test.com",
		"password":     "AdminPass123!",
		"phone_number": "+1234567891",
	}

	w, err := makeRequest("POST", "/api/v1/register", adminData, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	verifyEmailForUser(t, "admin@test.com")

	if testDB == nil {
		t.Fatal("testDB not initialized")
	}
	res := testDB.Exec(`UPDATE users SET role = ? WHERE LOWER(email) = LOWER(?)`, "admin", "admin@test.com")
	if res.Error != nil {
		t.Fatalf("promote seed admin: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		t.Fatal("promote seed admin: no row updated")
	}

	w, err = makeRequest("POST", "/api/v1/login", map[string]interface{}{
		"email":    "admin@test.com",
		"password": "AdminPass123!",
	}, "")
	if err != nil {
		t.Fatalf("Failed admin login: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for admin login, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}
	tokenData, ok := response["token"].(map[string]interface{})
	if !ok {
		t.Fatal("Token not found in login response")
	}
	adminToken = tokenData["jwt_token"].(string)

	if err := testDB.Raw(`SELECT id::text FROM users WHERE LOWER(email) = LOWER(?)`, "admin@test.com").Scan(&adminUserID).Error; err != nil || adminUserID == "" {
		t.Fatalf("load admin user id: %v", err)
	}

	fmt.Println("✓ Register admin test passed")
}

// Test 4: Login
func TestLogin(t *testing.T) {
	loginData := map[string]interface{}{
		"email":    "john.doe@test.com",
		"password": "Password123!",
	}

	w, err := makeRequest("POST", "/api/v1/login", loginData, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["token"] == nil {
		t.Error("Token not found in response")
	}

	fmt.Println("✓ Login test passed")
}

// Test 5: Login with Invalid Credentials
func TestLoginInvalidCredentials(t *testing.T) {
	loginData := map[string]interface{}{
		"email":    "john.doe@test.com",
		"password": "WrongPassword123!",
	}

	w, err := makeRequest("POST", "/api/v1/login", loginData, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	fmt.Println("✓ Login with invalid credentials test passed")
}

// Test 6: Get All Users (Authenticated admin)
func TestGetAllUsers(t *testing.T) {
	if adminToken == "" {
		t.Skip("Admin token not available")
	}

	w, err := makeRequest("GET", "/api/v1/admin/users", nil, adminToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var users []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(users) == 0 {
		t.Error("Expected at least one user")
	}

	fmt.Println("✓ Get all users test passed")
}

// Test 7: Get All Users (Unauthenticated)
func TestGetAllUsersUnauthenticated(t *testing.T) {
	w, err := makeRequest("GET", "/api/v1/admin/users", nil, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	fmt.Println("✓ Get all users unauthenticated test passed")
}

// Admin queue observability (requires admin JWT; Redis queue active in integration env).
func TestAdminJobs_ObservabilityAuthz(t *testing.T) {
	w, err := makeRequest("GET", "/api/v1/admin/jobs/health", nil, "")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /admin/jobs/health unauthenticated = %d, want 401", w.Code)
	}

	if userToken == "" {
		t.Skip("User token not available")
	}
	w, err = makeRequest("GET", "/api/v1/admin/jobs/health", nil, userToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("GET /admin/jobs/health as user = %d, want 403", w.Code)
	}

	if adminToken == "" {
		t.Skip("Admin token not available")
	}
	w, err = makeRequest("GET", "/api/v1/admin/jobs/health", nil, adminToken)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET /admin/jobs/health as admin = %d, body %s", w.Code, w.Body.String())
	}
	var h map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &h); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := h["queue_enabled"]; !ok {
		t.Fatal("expected queue_enabled in health response")
	}
	fmt.Println("✓ Admin jobs health authz test passed")
}

// Test 8a: GET /users/me returns profile + settings (runs after TestRegisterUser by name ordering).
func TestRegisteredUser_GetMeEnvelope(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("User token or ID not available")
	}

	w, err := makeRequest("GET", "/api/v1/users/me", nil, userToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	profile, ok := envelope["profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected profile object, got %v", envelope["profile"])
	}
	if profile["email"] != "john.doe@test.com" {
		t.Errorf("Expected profile.email 'john.doe@test.com', got '%v'", profile["email"])
	}

	fmt.Println("✓ Registered user GET /users/me envelope test passed")
}

// Test 9: Get Non-Existent User
func TestGetNonExistentUser(t *testing.T) {
	if adminToken == "" {
		t.Skip("Admin token not available")
	}

	// Valid UUID format required; this ID is not inserted by other tests.
	w, err := makeRequest("GET", "/api/v1/admin/users/00000000-0000-4000-8000-00000000dead", nil, adminToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	fmt.Println("✓ Get non-existent user test passed")
}

// Test 10: Create User (Authenticated)
func TestCreateUser(t *testing.T) {
	if adminToken == "" {
		t.Skip("Admin token not available")
	}

	createData := map[string]interface{}{
		"first_name":   "Jane",
		"last_name":    "Smith",
		"email":        "jane.smith@test.com",
		"password":     "Password123!",
		"phone_number": "+1234567892",
		"role":         "user",
	}

	w, err := makeRequest("POST", "/api/v1/admin/users", createData, adminToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var user map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if user["email"] != "jane.smith@test.com" {
		t.Errorf("Expected email 'jane.smith@test.com', got '%v'", user["email"])
	}

	fmt.Println("✓ Create user test passed")
}

// Test 10b: Regular user cannot create an admin
func TestCreateUserAsAdminForbidden(t *testing.T) {
	if userToken == "" {
		t.Skip("User token not available")
	}
	w, err := makeRequest("POST", "/api/v1/admin/users", map[string]interface{}{
		"first_name": "Elev",
		"last_name":  "Atee",
		"email":      "elevate-me@test.com",
		"password":   "Password123!",
		"role":       "admin",
	}, userToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
	fmt.Println("✓ Create admin as regular user forbidden test passed")
}

// Test 10c: Regular user cannot access admin user provisioning
func TestCreateUserAsRegularUserForbidden(t *testing.T) {
	if userToken == "" {
		t.Skip("User token not available")
	}
	w, err := makeRequest("POST", "/api/v1/admin/users", map[string]interface{}{
		"first_name": "No",
		"last_name":  "Access",
		"email":      "no-access-create@test.com",
		"password":   "Password123!",
		"role":       "user",
	}, userToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
	fmt.Println("✓ Create user as regular user forbidden test passed")
}

// Test 11: PATCH /users/me (profile subset only)
func TestUpdateUser(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("User token or ID not available")
	}

	updateData := map[string]interface{}{
		"profile": map[string]interface{}{
			"first_name": "John Updated",
			"last_name":  "Doe Updated",
		},
	}

	w, err := makeRequest(http.MethodPatch, "/api/v1/users/me", updateData, userToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	profile, ok := envelope["profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected profile object, got %v", envelope["profile"])
	}
	if profile["first_name"] != "John Updated" {
		t.Errorf("Expected profile.first_name 'John Updated', got '%v'", profile["first_name"])
	}

	fmt.Println("✓ PATCH /users/me test passed")
}

// Test 12: Delete User (Admin Only)
func TestDeleteUserAsAdmin(t *testing.T) {
	if adminToken == "" {
		t.Skip("Admin token not available")
	}

	// First create a user to delete
	createData := map[string]interface{}{
		"first_name":   "ToDelete",
		"last_name":    "User",
		"email":        "todelete@test.com",
		"password":     "Password123!",
		"phone_number": "+1234567893",
		"role":         "user",
	}

	w, err := makeRequest("POST", "/api/v1/admin/users", createData, adminToken)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if w.Code != http.StatusCreated {
		t.Skipf("Could not create user for deletion test: %d", w.Code)
		return
	}

	var user map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	deleteID := user["id"].(string)
	url := fmt.Sprintf("/api/v1/admin/users/%s", deleteID)

	w, err = makeRequest("DELETE", url, nil, adminToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	fmt.Println("✓ Delete user as admin test passed")
}

// Test 13: Delete User (Regular User - should fail)
func TestDeleteUserAsRegularUser(t *testing.T) {
	if userToken == "" || userID == "" {
		t.Skip("User token or ID not available")
	}

	url := fmt.Sprintf("/api/v1/admin/users/%s", userID)
	w, err := makeRequest("DELETE", url, nil, userToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}

	fmt.Println("✓ Delete user as regular user test passed")
}

// Test 14: Register Duplicate Email
func TestRegisterDuplicateEmail(t *testing.T) {
	registerData := map[string]interface{}{
		"first_name":   "Duplicate",
		"last_name":    "User",
		"email":        "john.doe@test.com", // Already registered
		"password":     "Password123!",
		"phone_number": "+1234567894",
		"role":         "user",
	}

	w, err := makeRequest("POST", "/api/v1/register", registerData, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d. Body: %s", w.Code, w.Body.String())
	}

	fmt.Println("✓ Register duplicate email test passed")
}

// Test 15: Register with Invalid Role
func TestRegisterInvalidRole(t *testing.T) {
	registerData := map[string]interface{}{
		"first_name":   "Invalid",
		"last_name":    "Role",
		"email":        "invalidrole@test.com",
		"password":     "Password123!",
		"phone_number": "+1234567895",
		"role":         "invalid_role",
	}

	w, err := makeRequest("POST", "/api/v1/register", registerData, "")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d. Body: %s", w.Code, w.Body.String())
	}

	fmt.Println("✓ Register with invalid role test passed")
}

// TestGetUsersPaginated tests pagination functionality
func TestGetUsersPaginated(t *testing.T) {
	loginData := map[string]interface{}{
		"email":    "admin@test.com",
		"password": "AdminPass123!",
	}

	wLogin, err := makeRequest("POST", "/api/v1/login", loginData, "")
	if err != nil {
		t.Fatalf("Failed to make login request: %v", err)
	}

	if wLogin.Code != http.StatusOK {
		t.Fatalf("Admin login required for pagination test: %d. Body: %s", wLogin.Code, wLogin.Body.String())
	}

	var loginResponse map[string]interface{}
	if err := json.Unmarshal(wLogin.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	tokenData, ok := loginResponse["token"].(map[string]interface{})
	if !ok {
		t.Fatal("Token not found in login response")
	}

	jwtToken, ok := tokenData["jwt_token"].(string)
	if !ok {
		t.Fatal("jwt_token not found in login response token object")
	}

	testToken := jwtToken

	// Seed users via public register (admin-only POST /admin/users is not needed here)
	testUsers := []map[string]interface{}{
		{"first_name": "Test1", "last_name": "User1", "email": "test1@pagtest.com", "password": "Password123!", "phone_number": "+1111111111", "role": "user"},
		{"first_name": "Test2", "last_name": "User2", "email": "test2@pagtest.com", "password": "Password123!", "phone_number": "+2222222222", "role": "user"},
		{"first_name": "Test3", "last_name": "User3", "email": "test3@pagtest.com", "password": "Password123!", "phone_number": "+3333333333", "role": "user"},
		{"first_name": "Test4", "last_name": "User4", "email": "test4@pagtest.com", "password": "Password123!", "phone_number": "+4444444444", "role": "user"},
		{"first_name": "Test5", "last_name": "User5", "email": "test5@pagtest.com", "password": "Password123!", "phone_number": "+5555555555", "role": "user"},
	}

	for _, userData := range testUsers {
		makeRequest("POST", "/api/v1/register", userData, "")
	}

	// Test 1: Pagination with page=1 and page_size=2
	w, err := makeRequest("GET", "/api/v1/admin/users?page=1&page_size=2", nil, testToken)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var paginatedResponse map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &paginatedResponse); err != nil {
		t.Fatalf("Failed to parse paginated response: %v", err)
	}

	// Verify pagination response structure
	data, ok := paginatedResponse["data"].([]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field or invalid type")
	}

	total, ok := paginatedResponse["total"].(float64)
	if !ok {
		t.Fatal("Response missing 'total' field or invalid type")
	}

	page, ok := paginatedResponse["page"].(float64)
	if !ok {
		t.Fatal("Response missing 'page' field or invalid type")
	}

	pageSize, ok := paginatedResponse["page_size"].(float64)
	if !ok {
		t.Fatal("Response missing 'page_size' field or invalid type")
	}

	totalPages, ok := paginatedResponse["total_pages"].(float64)
	if !ok {
		t.Fatal("Response missing 'total_pages' field or invalid type")
	}

	// Verify pagination values
	if len(data) != 2 {
		t.Errorf("Expected 2 users per page, got %d", len(data))
	}

	if int(page) != 1 {
		t.Errorf("Expected page 1, got %d", int(page))
	}

	if int(pageSize) != 2 {
		t.Errorf("Expected page_size 2, got %d", int(pageSize))
	}

	if total < 2 {
		t.Errorf("Expected total to be at least 2, got %d", int(total))
	}

	ps := int(pageSize)
	expectedTotalPages := (int(total) + ps - 1) / ps
	if int(totalPages) != expectedTotalPages {
		t.Errorf("Expected total_pages %d, got %d", expectedTotalPages, int(totalPages))
	}

	// Test 2: Test page 2
	w2, err := makeRequest("GET", "/api/v1/admin/users?page=2&page_size=2", nil, testToken)
	if err != nil {
		t.Fatalf("Failed to make request for page 2: %v", err)
	}

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200 for page 2, got %d. Body: %s", w2.Code, w2.Body.String())
		return
	}

	var page2Response map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &page2Response); err != nil {
		t.Fatalf("Failed to parse page 2 response: %v", err)
	}

	page2Data := page2Response["data"].([]interface{})
	page2Page := page2Response["page"].(float64)

	if int(page2Page) != 2 {
		t.Errorf("Expected page 2, got %d", int(page2Page))
	}

	// Verify page 2 has different data than page 1
	if len(page2Data) > 0 && len(data) > 0 {
		page1FirstID := getIDFromUser(data[0])
		page2FirstID := getIDFromUser(page2Data[0])
		if page1FirstID == page2FirstID {
			t.Error("Page 1 and page 2 should have different users")
		}
	}

	// Test 3: Test default pagination (page=1, page_size should default to 10)
	w3, err := makeRequest("GET", "/api/v1/admin/users?page=1", nil, testToken)
	if err != nil {
		t.Fatalf("Failed to make request with default page_size: %v", err)
	}

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200 with default page_size, got %d. Body: %s", w3.Code, w3.Body.String())
		return
	}

	var defaultResponse map[string]interface{}
	if err := json.Unmarshal(w3.Body.Bytes(), &defaultResponse); err != nil {
		t.Fatalf("Failed to parse default response: %v", err)
	}

	defaultPageSize := defaultResponse["page_size"].(float64)
	if int(defaultPageSize) != 10 {
		t.Errorf("Expected default page_size 10, got %d", int(defaultPageSize))
	}

	// Test 4: Test invalid page parameter
	w4, err := makeRequest("GET", "/api/v1/admin/users?page=0&page_size=2", nil, testToken)
	if err != nil {
		t.Fatalf("Failed to make request with invalid page: %v", err)
	}

	if w4.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid page, got %d. Body: %s", w4.Code, w4.Body.String())
	}

	// Test 5: Test max page_size limit (should cap at 100)
	w5, err := makeRequest("GET", "/api/v1/admin/users?page=1&page_size=200", nil, testToken)
	if err != nil {
		t.Fatalf("Failed to make request with large page_size: %v", err)
	}

	if w5.Code != http.StatusOK {
		t.Errorf("Expected status 200 for large page_size (should be capped), got %d. Body: %s", w5.Code, w5.Body.String())
		return
	}

	var largePageSizeResponse map[string]interface{}
	if err := json.Unmarshal(w5.Body.Bytes(), &largePageSizeResponse); err != nil {
		t.Fatalf("Failed to parse large page_size response: %v", err)
	}

	actualPageSize := largePageSizeResponse["page_size"].(float64)
	if int(actualPageSize) != 100 {
		t.Errorf("Expected page_size to be capped at 100, got %d", int(actualPageSize))
	}

	fmt.Println("✓ Pagination test passed")
}

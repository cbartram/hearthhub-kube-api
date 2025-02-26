package src

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockCognitoService is a mock implementation of the CognitoService interface
type MockCognitoService struct {
	mock.Mock
}

func (m *MockCognitoService) AuthUser(ctx context.Context, refreshToken *string, discordID *string) (*service.CognitoUser, error) {
	args := m.Called(ctx, refreshToken, discordID)
	return args.Get(0).(*service.CognitoUser), args.Error(1)
}

func (m *MockCognitoService) GetUserAttributes(ctx context.Context, accessToken *string) ([]types.AttributeType, error) {
	args := m.Called(ctx, accessToken)
	return args.Get(0).([]types.AttributeType), args.Error(1)
}

func (m *MockCognitoService) GetUser(ctx context.Context, discordId *string) (*service.CognitoUser, error) {
	args := m.Called(ctx, discordId)
	return args.Get(0).(*service.CognitoUser), args.Error(1)
}

func (m *MockCognitoService) UpdateUserAttributes(ctx context.Context, accessToken *string, attributes []types.AttributeType) error {
	args := m.Called(ctx, accessToken, attributes)
	return args.Error(0)
}

func TestLogMiddleware(t *testing.T) {
	fn := LogrusMiddleware(logrus.StandardLogger())
	assert.NotNil(t, fn)
}

func TestAuthMiddleware(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requiresMock   bool
		authHeader     string
		mockUser       *service.CognitoUser
		mockError      error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Missing Authorization Header",
			authHeader:     "",
			requiresMock:   false,
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Authorization header is required"}`,
		},
		{
			name:           "Invalid Authorization Header Format",
			requiresMock:   false,
			authHeader:     "InvalidFormat",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Invalid Authorization header format"}`,
		},
		{
			name:           "Failed to Decode Credentials",
			requiresMock:   false,
			authHeader:     "Basic InvalidBase64",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Failed to decode credentials"}`,
		},
		{
			name:           "Invalid Credentials Format",
			requiresMock:   false,
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("invalid")),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Invalid credentials format"}`,
		},
		{
			name:           "Authentication Failed",
			requiresMock:   true,
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("discord_id:refresh_token")),
			mockUser:       &service.CognitoUser{},
			mockError:      fmt.Errorf("authentication failed"),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"could not authenticate user with refresh token: authentication failed"}`,
		},
		{
			name:           "Authentication Successful",
			requiresMock:   true,
			authHeader:     "Basic " + base64.StdEncoding.EncodeToString([]byte("discord_id:refresh_token")),
			mockUser:       &service.CognitoUser{DiscordID: "discord_id"},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange

			mockCognitoService := new(MockCognitoService)
			if tt.requiresMock {
				mockCognitoService.On("AuthUser", mock.Anything, mock.Anything, mock.Anything).Return(tt.mockUser, tt.mockError)
			}

			router := gin.New()
			router.Use(AuthMiddleware(mockCognitoService))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, resp.Body.String())
			}

			mockCognitoService.AssertExpectations(t)
		})
	}
}

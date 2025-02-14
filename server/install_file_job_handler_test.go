package server

import (
	"bytes"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleFileHandlerRoute(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus int
		requestBody    io.Reader
		expectedBody   string
		requiresUser   bool
		user           *service.CognitoUser
	}{
		{
			name:           "Bad request body",
			expectedStatus: http.StatusBadRequest,
			requestBody:    bytes.NewBuffer(nil),
			expectedBody:   `{"error":"invalid request body: unexpected end of JSON input"}`,
		},
		{
			name:           "Fails request body validation",
			expectedStatus: http.StatusBadRequest,
			requestBody:    bytes.NewBuffer([]byte(`{"destination": "/bad/dest"}`)),
			expectedBody:   `{"error":"invalid request body: invalid destination: must be one of /root/.config/unity3d/IronGate/Valheim/worlds_local, /valheim/BepInEx/config, or /valheim/BepInEx/plugins"}`,
		},
		{
			name:           "No user in context",
			expectedStatus: http.StatusUnauthorized,
			requestBody:    bytes.NewBuffer([]byte(`{"destination": "/root/.config/unity3d/IronGate/Valheim/worlds_local", "operation": "write", "prefix": "/foo/bar.zip", "is_archive": true}`)),
			expectedBody:   `{"error":"user not found in context"}`,
		},
		{
			name:           "Creates Job OK",
			expectedStatus: http.StatusCreated,
			requestBody:    bytes.NewBuffer([]byte(`{"destination": "/root/.config/unity3d/IronGate/Valheim/worlds_local", "operation": "write", "prefix": "/foo/bar.zip", "is_archive": true}`)),
			expectedBody:   `{"message":"file job created: "}`,
			requiresUser:   true,
			user:           &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar"}},
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.Default()
			router.POST("/api/v1/file/install", func(c *gin.Context) {
				handler := InstallFileHandler{}
				if tt.requiresUser {
					c.Set("user", tt.user)
				}

				handler.HandleRequest(c, &FakeKubeClient{})
			})

			req, err := http.NewRequest("POST", "/api/v1/file/install", tt.requestBody)
			assert.NoError(t, err)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.JSONEq(t, tt.expectedBody, resp.Body.String())
		})
	}
}

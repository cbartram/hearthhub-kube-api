package server

import (
	"bytes"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleScaleServerHandlerRoute(t *testing.T) {
	tests := []struct {
		name                   string
		expectedStatus         int
		requestBody            io.Reader
		expectedBody           string
		requiresUser           bool
		requiresCognitoMock    bool
		cognitoAttributes      []types.AttributeType
		cognitoAttributesError error
		cognitoUpdateError     error
		user                   *service.CognitoUser
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
			requestBody:    bytes.NewBuffer([]byte(`{}`)),
			expectedBody:   `{"error":"replicas field required"}`,
		},
		{
			name:           "Bad replicas value",
			expectedStatus: http.StatusBadRequest,
			requestBody:    bytes.NewBuffer([]byte(`{"replicas": 15}`)),
			expectedBody:   `{"error":"replicas must be either 1 or 0"}`,
		},
		{
			name:           "No user in context",
			expectedStatus: http.StatusUnauthorized,
			requestBody:    bytes.NewBuffer([]byte(`{"replicas": 1}`)),
			expectedBody:   `{"error":"user not found in context"}`,
		},
		{
			name:                   "Fails getting user attributes",
			expectedStatus:         http.StatusInternalServerError,
			requestBody:            bytes.NewBuffer([]byte(`{"replicas": 0}`)),
			expectedBody:           `{"error":"could not get user attributes: attr doesnt exist"}`,
			requiresUser:           true,
			user:                   &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar", AccessToken: "bar"}},
			requiresCognitoMock:    true,
			cognitoAttributesError: errors.New("attr doesnt exist"),
		},
		{
			name:                   "No server to scale",
			expectedStatus:         http.StatusNotFound,
			requestBody:            bytes.NewBuffer([]byte(`{"replicas": 1}`)),
			expectedBody:           `{"error":"valheim server does not exist. nothing to scale."}`,
			requiresUser:           true,
			user:                   &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar", AccessToken: "bar"}},
			requiresCognitoMock:    true,
			cognitoAttributesError: nil,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
		},
		{
			name:                   "Server already scaled to: 1 replica",
			expectedStatus:         http.StatusBadRequest,
			requestBody:            bytes.NewBuffer([]byte(`{"replicas": 1}`)),
			expectedBody:           `{"error":"server already running. replicas must be 0 when server state is: RUNNING"}`,
			requiresUser:           true,
			user:                   &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar", AccessToken: "bar"}},
			requiresCognitoMock:    true,
			cognitoAttributesError: nil,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr(`{"state": "running"}`),
				},
			},
		},
		{
			name:                   "Server already scaled down to: 0",
			expectedStatus:         http.StatusBadRequest,
			requestBody:            bytes.NewBuffer([]byte(`{"replicas": 0}`)),
			expectedBody:           `{"error":"no server to terminate. replicas must be 1 when server state is: TERMINATED"}`,
			requiresUser:           true,
			user:                   &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar", AccessToken: "bar"}},
			requiresCognitoMock:    true,
			cognitoAttributesError: nil,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr(`{"state": "terminated"}`),
				},
			},
		},
		{
			name:                   "Fails getting deployment",
			expectedStatus:         http.StatusInternalServerError,
			requestBody:            bytes.NewBuffer([]byte(`{"replicas": 1}`)),
			expectedBody:           `{"error":"failed to update deployment args: deployments.apps \"valheim-foo\" not found"}`,
			requiresUser:           true,
			user:                   &service.CognitoUser{DiscordID: "foo", Credentials: service.CognitoCredentials{RefreshToken: "bar", AccessToken: "bar"}},
			requiresCognitoMock:    true,
			cognitoAttributesError: nil,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr(`{"state": "terminated"}`),
				},
			},
			cognitoUpdateError: errors.New("unauthorized field"),
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockCognitoService := new(MockCognitoService)

			if tt.requiresCognitoMock {
				mockCognitoService.On("GetUserAttributes", mock.Anything, mock.Anything).Return(tt.cognitoAttributes, tt.cognitoAttributesError)
				mockCognitoService.On("UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything).Return(tt.cognitoUpdateError)
			}

			router := gin.Default()
			router.PUT("/api/v1/server/scale", func(c *gin.Context) {
				handler := ScaleServerHandler{}
				if tt.requiresUser {
					c.Set("user", tt.user)
				}
				handler.HandleRequest(c, &FakeKubeClient{}, mockCognitoService, context.TODO())
			})

			req, err := http.NewRequest("PUT", "/api/v1/server/scale", tt.requestBody)
			assert.NoError(t, err)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.JSONEq(t, tt.expectedBody, resp.Body.String())
		})
	}
}

func TestUpdateServerDetails(t *testing.T) {
	t.Run("Fails updating user details", func(t *testing.T) {
		mockCognitoService := new(MockCognitoService)
		mockCognitoService.On("UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("could not update"))
		res, err := UpdateServerDetails(context.TODO(), mockCognitoService, &CreateServerResponse{}, &service.CognitoUser{}, "RUNNING")
		assert.NotNil(t, err)
		assert.Nil(t, res)
	})

	t.Run("Updates user details", func(t *testing.T) {
		mockCognitoService := new(MockCognitoService)
		mockCognitoService.On("UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		res, err := UpdateServerDetails(context.TODO(), mockCognitoService, &CreateServerResponse{}, &service.CognitoUser{}, "RUNNING")
		assert.NotNil(t, res)
		assert.Nil(t, err)
	})
}

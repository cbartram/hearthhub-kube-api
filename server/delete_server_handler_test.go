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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleDeleteServerRoute(t *testing.T) {
	tests := []struct {
		name                 string
		method               string
		path                 string
		expectedStatus       int
		requestBody          io.Reader
		expectedBody         string
		requiresUser         bool
		cognitoUser          *service.CognitoUser
		requiresCognito      bool
		cognitoAttributes    []types.AttributeType
		cognitoAttributesErr error
		cognitoUpdateErr     error
		requiresKube         bool
		kubeErr              error
	}{
		{
			name:            "No user in context",
			expectedStatus:  http.StatusUnauthorized,
			requiresUser:    false,
			requiresCognito: false,
			requestBody:     bytes.NewBuffer([]byte(`{}`)),
			expectedBody:    `{"error":"user not found in context"}`,
		},
		{
			name:           "Fails to get user attributes",
			expectedStatus: http.StatusInternalServerError,
			requiresUser:   true,
			cognitoUser: &service.CognitoUser{
				CognitoID:       "abc",
				DiscordUsername: "123",
				Email:           "foobar",
				DiscordID:       "123abc",
				AccountEnabled:  true,
				Credentials: service.CognitoCredentials{
					AccessToken:  "abc",
					RefreshToken: "def",
				},
			},
			requiresCognito: true,
			requiresKube:    true,
			requestBody:     bytes.NewBuffer([]byte(`{}`)),
			expectedBody:    `{"error":"failed to delete deployment/pvc: oh no"}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
			cognitoAttributesErr: errors.New("user unauthorized"),
		},
	}

	gin.SetMode(gin.TestMode)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockCognitoService := new(MockCognitoService)
			mockKubeClient := new(FakeKubeClient)

			if tt.requiresCognito {
				mockCognitoService.On("UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything).Return(tt.cognitoUpdateErr)
			}

			if tt.requiresKube {
				mockKubeClient.On("AddAction", mock.Anything).Return()
				mockKubeClient.On("Rollback").Return(errors.New("oh no"))

				actions := []service.ResourceAction{
					service.DeploymentAction{
						Deployment: &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name: "foo",
							},
						},
					},
					service.DeploymentAction{
						Deployment: &appsv1.Deployment{
							ObjectMeta: metav1.ObjectMeta{
								Name: "bar",
							},
						},
					},
				}

				mockKubeClient.On("GetActions").Return(actions)
			}

			router := gin.Default()
			router.DELETE("/api/v1/server/delete", func(c *gin.Context) {
				handler := DeleteServerHandler{}

				if tt.requiresUser {
					c.Set("user", tt.cognitoUser)
				}

				handler.HandleRequest(c, mockKubeClient, mockCognitoService, context.TODO())
			})

			req, err := http.NewRequest("DELETE", "/api/v1/server/delete", tt.requestBody)
			assert.NoError(t, err)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.JSONEq(t, tt.expectedBody, resp.Body.String())
		})
	}
}

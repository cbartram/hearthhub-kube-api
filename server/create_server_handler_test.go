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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type FakeKubeClient struct {
	mock.Mock
	ClientOpts []runtime.Object
}

func (f *FakeKubeClient) AddAction(action service.ResourceAction) {
	f.Called(action)
}

func (f *FakeKubeClient) ApplyResources() ([]string, error) {
	args := f.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (f *FakeKubeClient) GetActions() []service.ResourceAction {
	args := f.Called()
	return args.Get(0).([]service.ResourceAction)
}

func (f *FakeKubeClient) GetClient() kubernetes.Interface {
	return fake.NewClientset(f.ClientOpts...)
}

func (f *FakeKubeClient) GetClusterIp() (string, error) {
	args := f.Called()
	return args.String(0), args.Error(1)
}

func (f *FakeKubeClient) Rollback() ([]string, error) {
	args := f.Called()
	return args.Get(0).([]string), args.Error(1)
}

func TestHandleCreateServerRoute(t *testing.T) {
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
			name:            "Bad request body",
			requiresUser:    false,
			requiresCognito: false,
			expectedStatus:  http.StatusBadRequest,
			requestBody:     bytes.NewBuffer(nil),
			expectedBody:    `{"error":"invalid request body: unexpected end of JSON input"}`,
		},
		{
			name:            "Fails input validation",
			requiresUser:    false,
			requiresCognito: false,
			expectedStatus:  http.StatusBadRequest,
			requestBody:     bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar"}`)), // Missing password
			expectedBody:    `{"error":"invalid request body: missing required fields name, world, or password"}`,
		},
		{
			name:            "No user in context",
			expectedStatus:  http.StatusInternalServerError,
			requiresUser:    false,
			requiresCognito: false,
			requestBody:     bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody:    `{"error":"user not found in context"}`,
		},
		{
			name:           "Fails to get user attributes",
			expectedStatus: http.StatusUnauthorized,
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
			requestBody:     bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody:    `{"error":"could not get user attributes: user unauthorized"}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
			cognitoAttributesErr: errors.New("user unauthorized"),
		},
		{
			name:           "User already has Valheim server running",
			expectedStatus: http.StatusBadRequest,
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
			requestBody:     bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody:    `{"error":"server:  already exists for user: foobar"}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("{\"world\": \"running\"}"),
				},
			},
			cognitoAttributesErr: nil,
		},
		{
			name:            "Fails to create dedicated server deployment",
			expectedStatus:  http.StatusInternalServerError,
			requiresUser:    true,
			requiresCognito: true,
			requiresKube:    true,
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
			requestBody:  bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody: `{"error":"could not create dedicated server deployment: cannot validate manifest"}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
			cognitoAttributesErr: nil,
			kubeErr:              errors.New("cannot validate manifest"),
		},
		{
			name:            "Fails to update user attributes",
			expectedStatus:  http.StatusUnauthorized,
			requiresUser:    true,
			requiresCognito: true,
			requiresKube:    true,
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
			requestBody:  bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody: `{"error":"failed to update server details in cognito user attribute: cannot update manifest"}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
			cognitoAttributesErr: nil,
			kubeErr:              nil,
			cognitoUpdateErr:     errors.New("cannot update manifest"),
		},
		{
			name:            "Status OK",
			expectedStatus:  http.StatusOK,
			requiresUser:    true,
			requiresCognito: true,
			requiresKube:    true,
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
			requestBody:  bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody: `{"deployment_name":"bar", "mod_pvc_name":"foo", "server_ip":"123.456.789.0", "server_port":2456, "state":"running", "world_details":{"backup_count":3, "backup_interval_seconds":43200, "enable_crossplay":false, "initial_backup_seconds":7200, "instance_id":"", "modifiers":[], "name":"bar", "password":"hereisapassword", "port":"2456", "public":false, "save_interval_seconds":1800, "world":"foo"}}`,
			cognitoAttributes: []types.AttributeType{
				{
					Name:  stringPtr("custom:server_details"),
					Value: stringPtr("nil"),
				},
			},
			cognitoAttributesErr: nil,
			kubeErr:              nil,
			cognitoUpdateErr:     nil,
		},
	}

	gin.SetMode(gin.TestMode)
	os.Setenv("CPU_LIMIT", "1")
	os.Setenv("CPU_REQUEST", "1")
	os.Setenv("MEMORY_REQUEST", "128Mi")
	os.Setenv("MEMORY_LIMIT", "128Mi")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockCognitoService := new(MockCognitoService)
			mockKubeClient := new(FakeKubeClient)

			if tt.requiresCognito {
				mockCognitoService.On("GetUserAttributes", mock.Anything, mock.Anything).Return(tt.cognitoAttributes, tt.cognitoAttributesErr)
				mockCognitoService.On("UpdateUserAttributes", mock.Anything, mock.Anything, mock.Anything).Return(tt.cognitoUpdateErr)
			}

			if tt.requiresKube {
				mockKubeClient.On("ApplyResources").Return([]string{"foo", "bar"}, tt.kubeErr)
				mockKubeClient.On("AddAction", mock.Anything).Return()
				mockKubeClient.On("GetClusterIp").Return("123.456.789.0", nil)

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
			router.POST("/create-server", func(c *gin.Context) {
				handler := CreateServerHandler{}

				if tt.requiresUser {
					c.Set("user", tt.cognitoUser)
				}

				handler.HandleRequest(c, mockKubeClient, mockCognitoService, context.TODO())
			})

			req, err := http.NewRequest("POST", "/create-server", tt.requestBody)
			assert.NoError(t, err)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.JSONEq(t, tt.expectedBody, resp.Body.String())
		})
	}
}

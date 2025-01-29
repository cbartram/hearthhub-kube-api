package server

import (
	"bytes"
	"context"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"net/http"
	"net/http/httptest"
	"testing"
)

type FakeKubeClient struct {
	mock.Mock
}

func (f *FakeKubeClient) AddAction(action service.ResourceAction) {
	f.Called(action)
}

func (f *FakeKubeClient) ApplyResources() error {
	args := f.Called()
	return args.Error(0)
}

func (f *FakeKubeClient) GetActions() []service.ResourceAction {
	args := f.Called()
	return args.Get(0).([]service.ResourceAction)
}

func (f *FakeKubeClient) GetClient() kubernetes.Interface {
	return fake.NewClientset()
}

func TestHandleCreateServerRoute(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		requestBody    io.Reader
		expectedBody   string
		requiresUser   bool
	}{
		{
			name:           "Bad request body",
			method:         "POST",
			path:           "/create-server",
			requiresUser:   false,
			expectedStatus: http.StatusBadRequest,
			requestBody:    bytes.NewBuffer(nil),
			expectedBody:   `{"error":"invalid request body: unexpected end of JSON input"}`,
		},
		{
			name:           "Fails input validation",
			method:         "POST",
			path:           "/create-server",
			requiresUser:   false,
			expectedStatus: http.StatusBadRequest,
			requestBody:    bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar"}`)), // Missing password
			expectedBody:   `{"error":"invalid request body: missing required fields name, world, or password"}`,
		},
		{
			name:           "No user in context",
			method:         "POST",
			path:           "/create-server",
			expectedStatus: http.StatusInternalServerError,
			requiresUser:   false,
			requestBody:    bytes.NewBuffer([]byte(`{"world": "foo", "name": "bar", "password": "hereisapassword"}`)),
			expectedBody:   `{"error":"user not found in context"}`,
		},
	}

	gin.SetMode(gin.TestMode) // Set Gin to test mode

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.Default()
			router.POST("/create-server", func(c *gin.Context) {
				handler := CreateServerHandler{}
				handler.HandleRequest(c, &FakeKubeClient{}, &MockCognitoService{}, context.TODO())
			})

			req, err := http.NewRequest(tt.method, tt.path, tt.requestBody)
			assert.NoError(t, err)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatus, resp.Code)
			assert.JSONEq(t, tt.expectedBody, resp.Body.String())
		})
	}
}

package service

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest/fake"
	"net/http"
	"testing"
)

func FakeCognitoClient(roundTripper func(*http.Request) (*http.Response, error)) *cognitoidentityprovider.Client {
	client := cognitoidentityprovider.New(cognitoidentityprovider.Options{
		HTTPClient: fake.CreateHTTPClient(roundTripper),
		Region:     "us-east-1",
	})
	return client
}

func TestMakeCognitoService(t *testing.T) {
	cfg := aws.Config{
		Region: "us-east-1",
	}
	svc := MakeCognitoService(cfg)
	assert.NotNil(t, svc)
}

package src

import (
	"context"
	"github.com/cbartram/hearthhub-mod-api/src/handler/server"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRouter(t *testing.T) {
	r, _ := NewRouter(context.Background(), &server.FakeKubeClient{}, &MockCognitoService{})
	assert.NotNil(t, r)
}

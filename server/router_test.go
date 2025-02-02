package server

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRouter(t *testing.T) {
	r, _ := NewRouter(context.Background(), &FakeKubeClient{}, &MockCognitoService{})
	assert.NotNil(t, r)
}

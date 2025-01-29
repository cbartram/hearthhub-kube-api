package server

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter(context.Background())
	assert.NotNil(t, r)
}

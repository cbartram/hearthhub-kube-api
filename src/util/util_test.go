package util

import (
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func stringPtr(s string) *string {
	return &s
}

func TestGenerateInstanceId(t *testing.T) {
	id := GenerateInstanceId(5)
	assert.Len(t, id, 5)
}

func TestInt32Ptr(t *testing.T) {
	i := Int32Ptr(32)
	assert.Equal(t, int32(32), *i)
}

func TestGetAttribute(t *testing.T) {
	list := []types.AttributeType{
		{Name: stringPtr("FOO"), Value: stringPtr("BAR")},
		{Name: stringPtr("duo"), Value: stringPtr("lingo")},
	}
	attr := GetAttribute(list, "FOO")
	assert.Equal(t, attr, "BAR")
	missing := GetAttribute(list, "non-existing")
	assert.Equal(t, missing, "")
}

func TestMakeCognitoSecretHash(t *testing.T) {
	hash := MakeCognitoSecretHash("foo", "id", "secret")
	assert.Equal(t, hash, "2N6M4MfD7OlpR810Aw//6FyDO6uOYXJ7qCh5j2xUuew=")
}

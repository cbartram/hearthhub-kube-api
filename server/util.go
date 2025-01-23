package server

import (
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateInstanceId Generates a unique alphanumeric instance id with a given length. This is used to ensure deployments,
// and PVC's in the same namespace do not have conflicts. It is also used to generate a unique id for a playfab for the
// dedicated server so that multiple servers can run on a single port.
func GenerateInstanceId(length int) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Int32Ptr Converts an unsigned 32-bit integer into a pointer.
func Int32Ptr(i int32) *int32 {
	return &i
}

// MakeAttribute Creates a Cognito attribute that can be persisted.
func MakeAttribute(key, value string) types.AttributeType {
	attr := types.AttributeType{
		Name:  &key,
		Value: &value,
	}
	return attr
}

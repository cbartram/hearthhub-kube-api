package util

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	corev1 "k8s.io/api/core/v1"
	"math/rand"
	"os"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func GetHostname() string {
	host := os.Getenv("HOSTNAME")

	if host == "" {
		return "http://localhost:5173"
	}

	return "https://hearthhub.duckdns.org"
}

// GenerateInstanceId Generates a unique alphanumeric instance id with a given length. This is used to ensure deployments,
// and PVC's in the same namespace do not have conflicts. It is also used to generate a unique id for a playfab for the
// dedicated src so that multiple servers can run on a single port.
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

// GetAttribute Returns the string value for a given attribute name from Cognito.
func GetAttribute(attributes []types.AttributeType, attributeName string) string {
	for _, attribute := range attributes {
		if aws.ToString(attribute.Name) == attributeName {
			return aws.ToString(attribute.Value)
		}
	}

	return ""
}

func GetUserAttributeString(attributes []types.AttributeType, attributeName string) string {
	for _, attribute := range attributes {
		if aws.ToString(attribute.Name) == attributeName {
			return aws.ToString(attribute.Value)
		}
	}

	return ""
}

// MakeVolumes creates the volumes that will be mounted for both the src deployment and any file installation jobs.
func MakeVolumes(pvcName string) []corev1.Volume {
	return []corev1.Volume{
		{
			// PVC which holds mod information (used by the plugin-manager to install new mods)
			Name: "valheim-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		{
			// Unknown: this was included in the docker_start_server.sh file from Irongate. Unsure of how its used.
			Name:         "irongate",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
	}
}

// MakeVolumeMounts Creates the PVC volume mount locations for the deployment pod. These volume mounts
// are the only places files can be installed that will persist outside the life of the src.
func MakeVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "valheim-data",
			MountPath: "/valheim/BepInEx/plugins",
			SubPath:   "plugins",
		},
		{
			Name:      "valheim-data",
			MountPath: "/valheim/BepInEx/config",
			SubPath:   "config",
		},
		{
			Name:      "valheim-data",
			MountPath: "/root/.config/unity3d/IronGate/Valheim/worlds_local",
			SubPath:   "worlds_local",
		},
		{
			Name:      "irongate",
			MountPath: "/irongate",
		},
	}
}

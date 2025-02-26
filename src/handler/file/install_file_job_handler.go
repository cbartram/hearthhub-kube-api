package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type FilePayload struct {
	Prefix      *string `json:"prefix"`
	Destination string  `json:"destination"`
	IsArchive   bool    `json:"is_archive"`
	Operation   string  `json:"operation"`
}

// Validate Validates that the payload provide is not malformed or missing information.
func (f *FilePayload) Validate() error {
	validDestinations := []string{
		"/root/.config/unity3d/IronGate/Valheim/worlds_local",
		"/valheim/BepInEx/config",
		"/valheim/BepInEx/plugins",
	}

	validDestination := false
	for _, dest := range validDestinations {
		if strings.HasPrefix(f.Destination, dest) {
			validDestination = true
			break
		}
	}
	if !validDestination {
		return errors.New("invalid destination: must be prefixed by /root/.config/unity3d/IronGate/Valheim/worlds_local, /valheim/BepInEx/config, or /valheim/BepInEx/plugins")
	}

	// Validate Operation
	validOperations := []string{"write", "delete", "copy"}
	validOperation := false
	for _, op := range validOperations {
		if f.Operation == op {
			validOperation = true
			break
		}
	}
	if !validOperation {
		return errors.New("invalid operation: must be either 'write', 'copy', or 'delete'")
	}

	if f.Prefix == nil {
		return errors.New("prefix is required and cannot be empty")
	}

	return nil
}

type InstallFileHandler struct{}

func (h *InstallFileHandler) HandleRequest(c *gin.Context, kubeService service.KubernetesService) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not read body from request: %v", err)})
		return
	}

	var reqBody FilePayload
	if err = json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if err = reqBody.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
	}
	user := tmp.(*service.CognitoUser)
	name, err := CreateFileJob(kubeService.GetClient(), &reqBody, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not create file management job: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("file job created: %s", *name)})
}

// CreateFileJob Creates a new kubernetes job which attaches the valheim src PVC, downloads mods from S3,
// and installs mods onto the PVC before restarting the Valheim src.
func CreateFileJob(clientset kubernetes.Interface, payload *FilePayload, user *service.CognitoUser) (*string, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("mod-install-%s-", user.DiscordID),
			Labels: map[string]string{
				"tenant-discord-id": user.DiscordID,
			},
			Namespace: "hearthhub",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: ptr.To(int32(0)), // Ensures jobs are not retried (generally if a job fails it's a misconfiguration)
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: fmt.Sprintf("%s:%s", os.Getenv("FILE_MANAGER_IMAGE_NAME"), os.Getenv("FILE_MANAGER_IMAGE_VERSION")),
							Args: []string{
								"./plugin-manager",
								"-discord_id",
								user.DiscordID,
								"-refresh_token",
								user.Credentials.RefreshToken,
								"-prefix",
								*payload.Prefix,
								"-destination",
								payload.Destination,
								"-op",
								payload.Operation,
								"-archive",
								strconv.FormatBool(payload.IsArchive),
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "src-config",
										},
									},
								},
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "aws-creds",
										},
									},
								},
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "rabbitmq-secrets",
										},
									},
								},
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "cognito-secrets",
										},
									},
								},
							},
							VolumeMounts: util.MakeVolumeMounts(),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("750Mi"),
									corev1.ResourceCPU:    resource.MustParse("250m"),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       util.MakeVolumes(fmt.Sprintf("valheim-pvc-%s", user.DiscordID)),
				},
			},
		},
	}

	// Create the job in the specified namespace
	createdJob, err := clientset.BatchV1().Jobs("hearthhub").Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %v", err)
	}

	log.Infof("job successfully created: %s", createdJob.Name)
	return &createdJob.Name, nil
}

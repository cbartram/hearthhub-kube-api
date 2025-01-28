package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"path/filepath"
	"strconv"
)

type InstallFilePayload struct {
	DiscordId    string `json:"discord_id"`
	RefreshToken string `json:"refresh_token"`
	Prefix       string `json:"prefix"`
	Destination  string `json:"destination"`
	IsArchive    bool   `json:"is_archive"`
}

type InstallFileHandler struct{}

func (h *InstallFileHandler) HandleRequest(c *gin.Context, kubeService *service.KubernetesService, ctx context.Context) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody InstallFilePayload
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	name, err := CreateFileInstallJob(kubeService.Client, &reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not create mod install job: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("install file job created: %s", *name)})
}

// CreateFileInstallJob Creates a new kubernetes job which attaches the valheim server PVC, downloads mods from S3,
// and installs mods onto the PVC before restarting the Valheim server.
func CreateFileInstallJob(clientset *kubernetes.Clientset, payload *InstallFilePayload) (*string, error) {
	fileName := filepath.Base(payload.Prefix)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("mod-install-%s-", payload.DiscordId),
			Labels: map[string]string{
				"tenant-discord-id": payload.DiscordId,
				"file-name":         fileName,
			},
			Namespace: "hearthhub",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "main",
							Image: "cbartram/hearthhub-plugin-manager:0.0.5",
							Args: []string{
								"./plugin-manager",
								"-discord_id",
								payload.DiscordId,
								"-refresh_token",
								payload.RefreshToken,
								"-prefix",
								payload.Prefix,
								"-destination",
								payload.Destination,
								"-archive",
								strconv.FormatBool(payload.IsArchive),
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "server-config",
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
							},
							VolumeMounts: MakeVolumeMounts(),
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
					Volumes:       MakeVolumes(fmt.Sprintf("valheim-pvc-%s", payload.DiscordId)),
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

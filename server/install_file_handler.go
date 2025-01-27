package server

import (
	"context"
	"encoding/json"
	"fmt"
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

func (h *InstallFileHandler) HandleRequest(c *gin.Context, clientset *kubernetes.Clientset, ctx context.Context) {
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

	name, err := CreateFileInstallJob(clientset, &reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not create mod install job: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("mod install job created: %s", *name)})
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
							Image: "cbartram/hearthhub-plugin-manager:0.0.4",
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
											Name: "server-resource-config",
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "valheim-pvc",
									MountPath: "/valheim/BepInEx/plugins/",
									SubPath:   "plugins",
								},
								{
									Name:      "valheim-world-pvc",
									MountPath: "/root/.config/unity3d/IronGate/Valheim",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{
							Name: "valheim-pvc",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("valheim-pvc-%s", payload.DiscordId),
								},
							},
						},
						{
							Name: "valheim-world-pvc",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("valheim-world-pvc-%s", payload.DiscordId),
								},
							},
						},
					},
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

package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/cbartram/hearthhub-common/model"
	"github.com/cbartram/hearthhub-mod-api/src/service"
	"github.com/cbartram/hearthhub-mod-api/src/util"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"os"
	"strconv"
	"time"
)

type CreateServerRequest struct {
	Name                  *string          `json:"name"`
	World                 *string          `json:"world"`
	MemoryRequest         *int             `json:"memory_request"`
	CpuRequest            *int             `json:"cpu_request"`
	Password              *string          `json:"password"`
	Port                  *string          `json:"port"`
	EnableCrossplay       *bool            `json:"enable_crossplay,omitempty"`
	Public                *bool            `json:"public,omitempty"`
	Modifiers             []model.Modifier `json:"modifiers,omitempty"`
	SaveIntervalSeconds   *int             `json:"save_interval_seconds,omitempty"`
	BackupCount           *int             `json:"backup_count,omitempty"`
	InitialBackupSeconds  *int             `json:"initial_backup_seconds,omitempty"`
	BackupIntervalSeconds *int             `json:"backup_interval_seconds,omitempty"`
}

// MakeWorldWithDefaults creates a new struct holding WorldDetails like name, port, backup count etc... with default values
// that can be selectively overridden by provided options
func MakeWorldWithDefaults(options *CreateServerRequest) *model.WorldDetails {
	cpuLimit, _ := strconv.Atoi(os.Getenv("CPU_LIMIT"))
	memLimit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))

	worldDetails := &model.WorldDetails{
		Name:                  *options.Name,
		World:                 *options.World,
		Port:                  "2456",
		Password:              *options.Password,
		EnableCrossplay:       false,
		Public:                false,
		InstanceID:            util.GenerateInstanceId(8),
		SaveIntervalSeconds:   1800,
		BackupCount:           3,
		InitialBackupSeconds:  7200,
		BackupIntervalSeconds: 43200,
		Modifiers:             []model.Modifier{},
	}

	// If no cpu/memory were provided (nil) default to the limits. If cpu and mem were provided
	// but are greater than the limits set to the limits, finally cpu and mem were provided and within the limits
	// so set to the provided value
	if options.CpuRequest == nil {
		log.Infof("no cpu request specified in req: defaulting to limit: %d", cpuLimit)
		worldDetails.CPURequests = cpuLimit
	} else if *options.CpuRequest > cpuLimit {
		log.Infof("CPU limit (%d) exceeds maximum CPU limit (%d)", *options.CpuRequest, cpuLimit)
		worldDetails.CPURequests = cpuLimit
	} else {
		worldDetails.CPURequests = *options.CpuRequest
	}

	if options.MemoryRequest == nil {
		log.Infof("no memory request specified in req: defaulting to limit: %d", memLimit)
		worldDetails.MemoryRequests = memLimit
	} else if *options.MemoryRequest > memLimit {
		log.Infof("memory request (%d) exceeds maximum memory limit (%d)", *options.MemoryRequest, memLimit)
		worldDetails.MemoryRequests = memLimit
	} else {
		worldDetails.MemoryRequests = *options.MemoryRequest
	}

	// Override defaults with any provided options
	if options.Port != nil {
		worldDetails.Port = *options.Port
	}
	if options.EnableCrossplay != nil {
		worldDetails.EnableCrossplay = *options.EnableCrossplay
	}
	if options.Public != nil {
		worldDetails.Public = *options.Public
	}
	if len(options.Modifiers) > 0 {
		worldDetails.Modifiers = options.Modifiers
	}
	if options.SaveIntervalSeconds != nil {
		worldDetails.SaveIntervalSeconds = *options.SaveIntervalSeconds
	}
	if options.BackupCount != nil {
		worldDetails.BackupCount = *options.BackupCount
	}
	if options.InitialBackupSeconds != nil {
		worldDetails.InitialBackupSeconds = *options.InitialBackupSeconds
	}
	if options.BackupIntervalSeconds != nil {
		worldDetails.BackupIntervalSeconds = *options.BackupIntervalSeconds
	}

	return worldDetails
}

func (c *CreateServerRequest) Validate() error {
	if c.Name == nil || c.World == nil || c.Password == nil {
		return errors.New("missing required fields name, world, or password")
	}

	var validModifiers = map[string][]string{
		"combat":       {model.VERY_EASY, model.EASY, model.HARD, model.VERY_HARD},
		"deathpenalty": {model.CASUAL, model.VERY_EASY, model.EASY, model.HARD, model.HARDCORE}, // TODO unsure if this is camel or all lowercase
		"resources":    {model.MUCH_LESS, model.LESS, model.MORE, model.MUCHMORE, model.MOST},
		"raids":        {model.NONE, model.MUCH_LESS, model.LESS, model.MORE, model.MUCHMORE},
		"portals":      {model.CASUAL, model.HARD, model.VERY_HARD},
	}

	for _, modifier := range c.Modifiers {
		validValues, exists := validModifiers[modifier.Key]
		if !exists {
			return fmt.Errorf("invalid modifier key: \"%s\"", modifier.Key)
		}

		for _, validValue := range validValues {
			if modifier.Value == validValue {
				return nil // Valid value found
			}
		}

		return fmt.Errorf("invalid value for modifier \"%s\": \"%s\" valid values are: \"%v\"", modifier.Key, modifier.Value, validModifiers[modifier.Key])
	}

	return nil
}

type CreateServerHandler struct{}

// HandleRequest Handles the /api/v1/src/create to create a new Valheim dedicated src container. This route is
// responsible for creating the initial deployment and pvc which in turn creates the replicaset and pod for the src.
// Future src management like mod installation, user termination requests, custom world uploads, etc... will use
// the /api/v1/src/scale route to scale the replicas to 0-1 without removing the deployment or PVC.
func (h *CreateServerHandler) HandleRequest(c *gin.Context, ctx context.Context, w *service.Wrapper) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody CreateServerRequest
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	err = reqBody.Validate()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %s", err)})
		return
	}

	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*model.User)
	if len(user.Servers) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "server already exists for user"})
		return
	}

	limits, err := w.StripeService.GetSubscriptionLimits(user.SubscriptionId)
	if err != nil {
		log.Errorf("failed to get user subscription limits: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get subscription limit: %v", err)})
		return
	}

	user.SubscriptionLimits = *limits

	if *reqBody.BackupCount > user.SubscriptionLimits.MaxBackups {
		reqBody.BackupCount = &user.SubscriptionLimits.MaxBackups
		log.Infof("request max backups > users subscription limit: %d, new backup count set to limit: %d", user.SubscriptionLimits.MaxBackups, *reqBody.BackupCount)
	}

	world := MakeWorldWithDefaults(&reqBody)
	valheimServer, err := CreateDedicatedServerDeployment(world, w.KubeService, user)
	if err != nil {
		log.Errorf("could not create dedicated src deployment: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create dedicated src deployment: " + err.Error()})
		return
	}

	user.Servers = append(user.Servers, *valheimServer)
	tx := w.HearthhubDb.Save(user)
	if tx.Error != nil {
		log.Errorf("could not update user with server details: %s", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update user with server details: " + tx.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, valheimServer)
}

// CreateDedicatedServerDeployment Creates the valheim dedicated src deployment and pvc given the src configuration.
func CreateDedicatedServerDeployment(world *model.WorldDetails, kubeService service.KubernetesService, user *model.User) (*model.Server, error) {
	serverArgs := world.ToStringArgs()
	serverPort, _ := strconv.Atoi(world.Port)

	// Deployments & PVC are always tied to the discord ID. When a src is terminated and re-created it
	// will be made with a different pod name but the same deployment name making for easy replica scaling.
	pvcName := fmt.Sprintf("valheim-pvc-%s", user.DiscordID)
	deploymentName := fmt.Sprintf("valheim-%s", user.DiscordID)

	log.Infof("server requests/limits: cpu=%d mem=%d, server args: %v", world.CPURequests, world.MemoryRequests, serverArgs)
	labels := map[string]string{
		"app":               "valheim",
		"created-by":        deploymentName,
		"tenant-discord-id": user.DiscordID,
	}

	// Create deployment object
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: "hearthhub",
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.Int32Ptr(1),
			// Important: this ensures old pods are deleted before new ones are created. It results in src downtime
			// but ensures that we don't get stuck with 2 servers trying to start without enough resources while 1 gets
			// caught in a pending scheduling loop.
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "hearthhub-api-sa",
					Containers: []corev1.Container{
						{
							Name:    "valheim",
							Image:   fmt.Sprintf("%s:%s", os.Getenv("VALHEIM_IMAGE_NAME"), os.Getenv("VALHEIM_IMAGE_VERSION")),
							Command: []string{"sh", "-c"},
							Args:    []string{serverArgs},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(serverPort),
									Name:          "game",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/valheim/health_check.sh"},
									},
								},
								InitialDelaySeconds: 15, // This value sets the min time it takes to "start" the src.
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    25, // Essentially 250 extra seconds for the src to startup
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(world.CPURequests)),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(world.MemoryRequests) + "Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(world.CPURequests)),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(world.MemoryRequests) + "Gi"),
								},
							},
							VolumeMounts: util.MakeVolumeMounts(),
						},
						{
							Name:    "backup-manager",
							Image:   fmt.Sprintf("%s:%s", os.Getenv("BACKUP_MANAGER_IMAGE_NAME"), os.Getenv("BACKUP_MANAGER_IMAGE_VERSION")),
							Command: []string{"sh", "-c"},
							Args:    []string{fmt.Sprintf("/app/main -mode backup -max-backups %d -token %s", user.SubscriptionLimits.MaxBackups, user.Credentials.RefreshToken)},

							// This container immediately tries to hit the kube api for pod labels and pod metrics. This startup probe
							// ensures no timeouts occur while the pod data is propagating through etcd and the control plane API.
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c",
											"curl -s --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt -H \"Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)\" https://kubernetes.default.svc/api/v1/namespaces/hearthhub/pods/$HOSTNAME",
										},
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       2,
								SuccessThreshold:    1,
								FailureThreshold:    10,
							},

							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("256m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("256m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},

							// Although these actions don't pertain to the actual valheim-src container they do pertain to the same pod so the information
							// delivered to users will still be quite accurate (if not slightly inflated).
							Lifecycle: &corev1.Lifecycle{
								PostStart: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/app/main", "-mode", "publish", "-type", "PostStart"},
									},
								},
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/app/main", "-mode", "publish", "-type", "PreStop"},
									},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
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
											Name: "mysql-secrets",
										},
									},
								},
								// Required if the mode is set to publish
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "rabbitmq-secrets",
										},
									},
								},
								// Required so backups which are persisted to s3 can also update user attributes
								// letting the frontend know which backups are auto installed.
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "cognito-secrets",
										},
									},
								},
								// AWS_REGION, RABBITMQ_BASE_URL and BACKUP_FREQ env vars are part of this CM which are also required
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "server-config",
										},
									},
								},
							},
							VolumeMounts: util.MakeVolumeMounts(),
						},
					},
					Volumes: util.MakeVolumes(pvcName),
				},
			},
		},
	}

	// If PVC already exists remove the finalizers and delete
	if kubeService.DoesPvcExist(pvcName) {
		log.Infof("pvc: %s already exists, removing finalizers and deleting before re-creation", pvcName)
		err := kubeService.RemoveFinalizersAndDelete(pvcName)
		if err != nil {
			log.Errorf("failed to remove finalizers and pvc: %s, error: %v", pvcName, err)
			return nil, err
		}

		// Sleeping gives a buffer for the pvc to be deleted before its attempt to be re-created.
		time.Sleep(5 * time.Second)
	}

	kubeService.AddAction(&service.PVCAction{PVC: MakePvc(pvcName, deploymentName, user.DiscordID)})
	kubeService.AddAction(&service.DeploymentAction{Deployment: deployment})

	names, err := kubeService.ApplyResources()
	if err != nil {
		log.Errorf("failed to apply kubernetes resource: %v", err)
		return nil, err
	}

	if len(names) != 2 {
		return nil, errors.New(fmt.Sprintf("failed to apply all kubernetes resources (%d/2)", len(names)))
	}

	ip, err := kubeService.GetClusterIp()
	if err != nil {
		log.Errorf("failed to get cluster ip: %v", err)
	}

	cpuLimit, _ := strconv.Atoi(os.Getenv("CPU_LIMIT"))
	memLimit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))

	// Rm the instance id from the response it's not useful for users and makes
	// testing harder since it generates a pseudo-random alphanumeric string with
	// each invocation
	world.InstanceID = ""
	return &model.Server{
		ServerIP:       ip,
		ServerPort:     serverPort,
		ServerCPU:      world.CPURequests,
		ServerMemory:   world.MemoryRequests,
		CPULimit:       cpuLimit,
		MemoryLimit:    memLimit,
		WorldDetails:   *world,
		PVCName:        names[0],
		DeploymentName: names[1],
		State:          model.RUNNING,
	}, nil
}

// MakePvc Returns the PVC object from the Kubernetes API for creating a new volume.
func MakePvc(name string, deploymentName string, discordId string) *corev1.PersistentVolumeClaim {
	// We only need a persistent volume for the plugins that will be installed. Need to shut down src, mount
	// pvc to a Job, install plugins to pvc, restart src, re-mount pvc
	// Game files like backups and world files will be (eventually) persisted to s3 by
	// the sidecar container so EmptyDir{} can be used for those.
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "hearthhub",
			Labels: map[string]string{
				"app":               "valheim",
				"created-by":        deploymentName,
				"tenant-discord-id": discordId,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
			},
		},
	}
}

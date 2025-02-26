package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/src/cfg"
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
)

type CreateServerRequest struct {
	Name                  *string        `json:"name"`
	World                 *string        `json:"world"`
	MemoryRequest         *int           `json:"memory_request"`
	CpuRequest            *int           `json:"cpu_request"`
	Password              *string        `json:"password"`
	Port                  *string        `json:"port"`
	EnableCrossplay       *bool          `json:"enable_crossplay,omitempty"`
	Public                *bool          `json:"public,omitempty"`
	Modifiers             []cfg.Modifier `json:"modifiers,omitempty"`
	SaveIntervalSeconds   *int           `json:"save_interval_seconds,omitempty"`
	BackupCount           *int           `json:"backup_count,omitempty"`
	InitialBackupSeconds  *int           `json:"initial_backup_seconds,omitempty"`
	BackupIntervalSeconds *int           `json:"backup_interval_seconds,omitempty"`
}

// MakeConfigWithDefaults creates a new ServerConfig with default values
// that can be selectively overridden by provided options
func MakeConfigWithDefaults(options *CreateServerRequest) *cfg.Config {
	cpuLimit, _ := strconv.Atoi(os.Getenv("CPU_LIMIT"))
	memLimit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))

	config := &cfg.Config{
		Name:                  *options.Name,
		World:                 *options.World,
		Port:                  "2456",
		Password:              *options.Password,
		EnableCrossplay:       false,
		Public:                false,
		InstanceId:            util.GenerateInstanceId(8),
		SaveIntervalSeconds:   1800,
		BackupCount:           3,
		InitialBackupSeconds:  7200,
		BackupIntervalSeconds: 43200,
		Modifiers:             []cfg.Modifier{},
	}

	// If no cpu/memory were provided (nil) default to the limits. If cpu and mem were provided
	// but are greater than the limits set to the limits, finally cpu and mem were provided and within the limits
	// so set to the provided value
	if options.CpuRequest == nil {
		log.Infof("no cpu request specified in req: defaulting to limit: %d", cpuLimit)
		config.CpuRequest = cpuLimit
	} else if *options.CpuRequest > cpuLimit {
		log.Infof("CPU limit (%d) exceeds maximum CPU limit (%d)", *options.CpuRequest, cpuLimit)
		config.CpuRequest = cpuLimit
	} else {
		config.CpuRequest = *options.CpuRequest
	}

	if options.MemoryRequest == nil {
		log.Infof("no memory request specified in req: defaulting to limit: %d", memLimit)
		config.MemoryRequest = memLimit
	} else if *options.MemoryRequest > memLimit {
		log.Infof("memory request (%d) exceeds maximum memory limit (%d)", *options.MemoryRequest, memLimit)
		config.MemoryRequest = memLimit
	} else {
		config.MemoryRequest = *options.MemoryRequest
	}

	// Override defaults with any provided options
	if options.Port != nil {
		config.Port = *options.Port
	}
	if options.EnableCrossplay != nil {
		config.EnableCrossplay = *options.EnableCrossplay
	}
	if options.Public != nil {
		config.Public = *options.Public
	}
	if len(options.Modifiers) > 0 {
		config.Modifiers = options.Modifiers
	}
	if options.SaveIntervalSeconds != nil {
		config.SaveIntervalSeconds = *options.SaveIntervalSeconds
	}
	if options.BackupCount != nil {
		config.BackupCount = *options.BackupCount
	}
	if options.InitialBackupSeconds != nil {
		config.InitialBackupSeconds = *options.InitialBackupSeconds
	}
	if options.BackupIntervalSeconds != nil {
		config.BackupIntervalSeconds = *options.BackupIntervalSeconds
	}

	return config
}

func (c *CreateServerRequest) Validate() error {
	if c.Name == nil || c.World == nil || c.Password == nil {
		return errors.New("missing required fields name, world, or password")
	}

	var validModifiers = map[string][]string{
		"combat":       {cfg.VERY_EASY, cfg.EASY, cfg.HARD, cfg.VERY_HARD},
		"deathpenalty": {cfg.CASUAL, cfg.VERY_EASY, cfg.EASY, cfg.HARD, cfg.HARDCORE}, // TODO unsure if this is camel or all lowercase
		"resources":    {cfg.MUCH_LESS, cfg.LESS, cfg.MORE, cfg.MUCHMORE, cfg.MOST},
		"raids":        {cfg.NONE, cfg.MUCH_LESS, cfg.LESS, cfg.MORE, cfg.MUCHMORE},
		"portals":      {cfg.CASUAL, cfg.HARD, cfg.VERY_HARD},
	}

	for _, modifier := range c.Modifiers {
		validValues, exists := validModifiers[modifier.ModifierKey]
		if !exists {
			return fmt.Errorf("invalid modifier key: \"%s\"", modifier.ModifierKey)
		}

		for _, validValue := range validValues {
			if modifier.ModifierValue == validValue {
				return nil // Valid value found
			}
		}

		return fmt.Errorf("invalid value for modifier \"%s\": \"%s\" valid values are: \"%v\"", modifier.ModifierKey, modifier.ModifierValue, validModifiers[modifier.ModifierKey])
	}

	return nil
}

type CreateServerResponse struct {
	ServerIp       string     `json:"server_ip"`
	ServerPort     int        `json:"server_port"`
	ServerMemory   int        `json:"server_memory"`
	ServerCpu      int        `json:"server_cpu"`
	CpuLimit       int        `json:"cpu_limit"`
	MemoryLimit    int        `json:"memory_limit"`
	WorldDetails   cfg.Config `json:"world_details"`
	PvcName        string     `json:"mod_pvc_name"`
	DeploymentName string     `json:"deployment_name"`
	State          string     `json:"state"`
}

type CreateServerHandler struct{}

// HandleRequest Handles the /api/v1/src/create to create a new Valheim dedicated src container. This route is
// responsible for creating the initial deployment and pvc which in turn creates the replicaset and pod for the src.
// Future src management like mod installation, user termination requests, custom world uploads, etc... will use
// the /api/v1/src/scale route to scale the replicas to 0-1 without removing the deployment or PVC.
func (h *CreateServerHandler) HandleRequest(c *gin.Context, kubeService service.KubernetesService, cognito service.CognitoService, ctx context.Context) {
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

	user := tmp.(*service.CognitoUser)

	// Verify that src details is "nil". This avoids a scenario where a
	// user could create more than 1 src.
	attributes, err := cognito.GetUserAttributes(ctx, &user.Credentials.AccessToken)
	serverDetails := util.GetAttribute(attributes, "custom:server_details")
	res := CreateServerResponse{}
	if err != nil {
		log.Errorf("could not get user attributes: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not get user attributes: %s", err)})
		return
	}

	// If src is nil it's the first time the user is booting up.
	if serverDetails != "nil" {
		json.Unmarshal([]byte(serverDetails), &res)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("src: %s already exists for user: %s", res.DeploymentName, user.Email)})
		return
	}

	config := MakeConfigWithDefaults(&reqBody)
	valheimServer, err := CreateDedicatedServerDeployment(config, kubeService, user)
	if err != nil {
		log.Errorf("could not create dedicated src deployment: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create dedicated src deployment: " + err.Error()})
		return
	}

	serverData, err := json.Marshal(valheimServer)
	if err != nil {
		log.Errorf("failed to marshall src data to json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to marshall src data to json: %s", err.Error())})
		return
	}

	attr := util.MakeAttribute("custom:server_details", string(serverData))
	err = cognito.UpdateUserAttributes(ctx, &user.Credentials.AccessToken, []types.AttributeType{attr})
	if err != nil {
		log.Errorf("failed to update src details in cognito user attribute: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("failed to update src details in cognito user attribute: %v", err)})
		return
	}

	c.JSON(http.StatusOK, valheimServer)
}

// CreateDedicatedServerDeployment Creates the valheim dedicated src deployment and pvc given the src configuration.
func CreateDedicatedServerDeployment(config *cfg.Config, kubeService service.KubernetesService, user *service.CognitoUser) (*CreateServerResponse, error) {
	serverArgs := config.ToStringArgs()
	serverPort, _ := strconv.Atoi(config.Port)

	// Deployments & PVC are always tied to the discord ID. When a src is terminated and re-created it
	// will be made with a different pod name but the same deployment name making for easy replica scaling.
	pvcName := fmt.Sprintf("valheim-pvc-%s", user.DiscordID)
	deploymentName := fmt.Sprintf("valheim-%s", user.DiscordID)

	log.Infof("src requests/limits: cpu=%d mem=%d, src args: %v", config.CpuRequest, config.MemoryRequest, serverArgs)
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
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(config.CpuRequest)),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(config.MemoryRequest) + "Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(config.CpuRequest)),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(config.MemoryRequest) + "Gi"),
								},
							},
							VolumeMounts: util.MakeVolumeMounts(),
						},
						{
							Name:    "backup-manager",
							Image:   fmt.Sprintf("%s:%s", os.Getenv("BACKUP_MANAGER_IMAGE_NAME"), os.Getenv("BACKUP_MANAGER_IMAGE_VERSION")),
							Command: []string{"sh", "-c"},
							Args:    []string{fmt.Sprintf("/app/main -mode backup -token %s", user.Credentials.RefreshToken)},

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
							// Ensure this container gets AWS creds so it can upload to S3
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "aws-creds",
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
											Name: "src-config",
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
	config.InstanceId = ""
	return &CreateServerResponse{
		ServerIp:       ip,
		ServerPort:     serverPort,
		ServerCpu:      config.CpuRequest,
		ServerMemory:   config.MemoryRequest,
		CpuLimit:       cpuLimit,
		MemoryLimit:    memLimit,
		WorldDetails:   *config,
		PvcName:        names[0],
		DeploymentName: names[1],
		State:          cfg.RUNNING,
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

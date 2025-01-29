package server

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/cbartram/hearthhub-mod-api/server/service"
	"github.com/cbartram/hearthhub-mod-api/server/util"
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

const (
	//Combat: veryeasy, easy, hard, veryhard
	//DeathPenalty: casual, veryeasy, easy, hard, hardcore
	//Resources: muchless, less, more, muchmore, most
	//Raids: none, muchless, less, more, muchmore
	//Portals: casual, hard, veryhard

	// Difficulties & Death penalties
	VERY_EASY = "veryeasy"
	EASY      = "easy"
	HARD      = "hard"     // only valid for portals
	VERY_HARD = "veryhard" // combat only & only valid for portals
	CASUAL    = "casual"   // only valid for portals
	HARDCORE  = "hardcore" // deathpenalty only

	// Resources & Raids
	NONE      = "none" // Raid only
	MUCH_LESS = "muchless"
	LESS      = "less"
	MORE      = "more"
	MUCHMORE  = "muchmore"
	MOST      = "most" // resource only

	// Modifier Keys
	COMBAT        = "combat"
	DEATH_PENALTY = "deathpenalty"
	RESOURCES     = "resources"
	RAIDS         = "raids"
	PORTALS       = "portals"

	// Server states
	RUNNING    = "running"
	TERMINATED = "terminated"
)

type ServerConfig struct {
	Name                  string
	Port                  string
	World                 string
	Password              string
	EnableCrossplay       bool
	Public                bool
	SaveIntervalSeconds   int
	BackupCount           int
	InitialBackupSeconds  int
	BackupIntervalSeconds int
	InstanceId            string
	Modifiers             []ServerModifier
}

type ServerModifier struct {
	ModifierKey   string `json:"key"`
	ModifierValue string `json:"value"`
}

type CreateServerRequest struct {
	Name            string           `json:"name"`
	World           string           `json:"world"`
	Password        string           `json:"password"`
	EnableCrossplay bool             `json:"enable_crossplay"`
	Public          bool             `json:"public"`
	Modifiers       []ServerModifier `json:"modifiers"`
}

type ValheimDedicatedServer struct {
	ServerIp       string              `json:"server_ip"`
	ServerPort     int                 `json:"server_port"`
	WorldDetails   CreateServerRequest `json:"world_details"`
	PvcName        string              `json:"mod_pvc_name"`
	DeploymentName string              `json:"deployment_name"`
	State          string              `json:"state"`
}

type CreateServerHandler struct{}

// HandleRequest Handles the /api/v1/server/create to create a new Valheim dedicated server container. This route is
// responsible for creating the initial deployment and pvc which in turn creates the replicaset and pod for the server.
// Future server management like mod installation, user termination requests, custom world uploads, etc... will use
// the /api/v1/server/scale route to scale the replicas to 0-1 without removing the deployment or PVC.
func (h *CreateServerHandler) HandleRequest(c *gin.Context, kubeService *service.KubernetesService, ctx context.Context) {
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("could not read body from request: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read body from request: " + err.Error()})
		return
	}

	var reqBody CreateServerRequest
	if err := json.Unmarshal(bodyRaw, &reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// TODO Validate port, modifiers, etc...
	cognito := service.MakeCognitoService()
	tmp, exists := c.Get("user")
	if !exists {
		log.Errorf("user not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found in context"})
		return
	}

	user := tmp.(*service.CognitoUser)

	// Verify that server details is "nil". This avoids a scenario where a
	// user could create more than 1 server.
	attributes, err := cognito.GetUserAttributes(ctx, &user.Credentials.AccessToken)
	serverDetails := util.GetAttribute(attributes, "custom:server_details")
	tmpServer := ValheimDedicatedServer{}
	log.Infof("user attributes: %v", serverDetails)
	if err != nil {
		log.Errorf("could not get user attributes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not get user attributes: %s", err)})
		return
	}

	// If server is nil it's the first time the user is booting up.
	if serverDetails != "nil" {
		json.Unmarshal([]byte(serverDetails), &tmpServer)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("server: %s already exists for user: %s. use PUT /api/v1/server/scale to manage replicas.", tmpServer.DeploymentName, user.Email)})
		return
	}

	config := MakeServerConfigWithDefaults(reqBody.Name, reqBody.World, reqBody.Password, reqBody.EnableCrossplay, reqBody.Public, reqBody.Modifiers)
	valheimServer, err := CreateDedicatedServerDeployment(config, kubeService, &reqBody, user.DiscordID)
	if err != nil {
		log.Errorf("could not create dedicated server deployment: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create dedicated server deployment: " + err.Error()})
		return
	}

	serverData, err := json.Marshal(valheimServer)
	if err != nil {
		log.Errorf("failed to marshall server data to json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to marshall server data to json: %s", err.Error())})
		return
	}

	attr := util.MakeAttribute("custom:server_details", string(serverData))
	err = cognito.UpdateUserAttributes(ctx, &user.Credentials.AccessToken, []types.AttributeType{attr})
	if err != nil {
		log.Errorf("failed to update server details in cognito user attribute: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("failed to update server details in cognito user attribute: %v", err)})
		return
	}

	c.JSON(http.StatusOK, valheimServer)
}

func MakeServerConfigWithDefaults(name, world, password string, crossplay bool, public bool, modifiers []ServerModifier) *ServerConfig {
	return &ServerConfig{
		Name:                  name,
		World:                 world,
		Port:                  "2456",
		Password:              password,
		EnableCrossplay:       crossplay,
		Public:                public,
		InstanceId:            util.GenerateInstanceId(8),
		Modifiers:             modifiers,
		SaveIntervalSeconds:   1800,
		BackupCount:           3,
		InitialBackupSeconds:  7200,
		BackupIntervalSeconds: 43200,
	}
}

// CreateDedicatedServerDeployment Creates the valheim dedicated server deployment and pvc given the server configuration.
func CreateDedicatedServerDeployment(serverConfig *ServerConfig, kubeService *service.KubernetesService, request *CreateServerRequest, discordId string) (*ValheimDedicatedServer, error) {
	// Ensures the refresh token doesn't get echo'd in the response
	serverArgs := []string{
		"./valheim_server.x86_64",
		"-name",
		serverConfig.Name,
		"-port",
		serverConfig.Port,
		"-world",
		serverConfig.World,
		"-password",
		serverConfig.Password,
		"-instanceid",
		serverConfig.InstanceId,
		"-backups",
		strconv.Itoa(serverConfig.BackupCount),
		"-backupshort",
		strconv.Itoa(serverConfig.InitialBackupSeconds),
		"-backuplong",
		strconv.Itoa(serverConfig.BackupIntervalSeconds),
	}

	if serverConfig.EnableCrossplay {
		serverArgs = append(serverArgs, "-crossplay")
	}

	if serverConfig.Public {
		serverArgs = append(serverArgs, "-public", "1")
	} else {
		serverArgs = append(serverArgs, "-public", "0")
	}

	for _, modifier := range serverConfig.Modifiers {
		serverArgs = append(serverArgs, "-modifier", modifier.ModifierKey, modifier.ModifierValue)
	}

	serverPort, _ := strconv.Atoi(serverConfig.Port)

	// Deployments & PVC are always tied to the discord ID. When a server is terminated and re-created it
	// will be made with a different pod name but the same deployment name making for easy replica scaling.
	pvcName := fmt.Sprintf("valheim-pvc-%s", discordId)
	deploymentName := fmt.Sprintf("valheim-%s", discordId)

	log.Infof("server args: %v", serverArgs)

	// Create deployment object
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: "hearthhub",
			Labels: map[string]string{
				"app": "valheim",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "valheim",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":               "valheim",
						"created-by":        deploymentName,
						"tenant-discord-id": discordId,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "hearthhub-api-sa",
					Containers: []corev1.Container{
						{
							Name:  "valheim",
							Image: fmt.Sprintf("%s:%s", os.Getenv("VALHEIM_IMAGE_NAME"), os.Getenv("VALHEIM_IMAGE_VERSION")),
							Args:  serverArgs,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(serverPort),
									Name:          "game",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(os.Getenv("CPU_LIMIT")),
									corev1.ResourceMemory: resource.MustParse(os.Getenv("MEMORY_LIMIT")),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(os.Getenv("CPU_REQUEST")),
									corev1.ResourceMemory: resource.MustParse(os.Getenv("MEMORY_REQUEST")),
								},
							},
							VolumeMounts: MakeVolumeMounts(),
						},
						{
							Name:  "backup-manager",
							Image: fmt.Sprintf("%s:%s", os.Getenv("BACKUP_MANAGER_IMAGE_NAME"), os.Getenv("BACKUP_MANAGER_IMAGE_VERSION")),

							// Ensure this container gets AWS creds so it can upload to S3
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "aws-creds",
										},
									},
								},
								// AWS_REGION and BACKUP_FREQ env vars are part of this CM which are also required
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "server-config",
										},
									},
								},
							},
							VolumeMounts: MakeVolumeMounts(),
						},
					},
					Volumes: MakeVolumes(pvcName),
				},
			},
		},
	}

	kubeService.AddAction(&service.PVCAction{PVC: MakePvc(pvcName, deploymentName, discordId)})
	kubeService.AddAction(&service.DeploymentAction{Deployment: deployment})

	err := kubeService.ApplyResources()
	if err != nil {
		log.Errorf("failed to apply kubernetes resource: %v", err)
		return nil, err
	}

	ip, err := util.GetPublicIP()
	if err != nil {
		log.Errorf("failed to get public ip: %v", err)
	}

	return &ValheimDedicatedServer{
		ServerIp:       ip,
		ServerPort:     serverPort,
		WorldDetails:   *request,
		PvcName:        kubeService.ResourceActions[0].Name(),
		DeploymentName: kubeService.ResourceActions[1].Name(),
		State:          RUNNING,
	}, nil
}

// MakePvc Returns the PVC object from the Kubernetes API for creating a new volume.
func MakePvc(name string, deploymentName string, discordId string) *corev1.PersistentVolumeClaim {
	// We only need a persistent volume for the plugins that will be installed. Need to shut down server, mount
	// pvc to a Job, install plugins to pvc, restart server, re-mount pvc
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

// MakeVolumes creates the volumes that will be mounted for both the server deployment and any file installation jobs.
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
// are the only places files can be installed that will persist outside the life of the server.
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

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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"strconv"
	"time"
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
	SaveIntervalSeconds   int64
	BackupCount           int
	InitialBackupSeconds  int64
	BackupIntervalSeconds int64
	InstanceId            string
	Modifiers             []ServerModifier
}

type ServerModifier struct {
	ModifierKey   string `json:"key"`
	ModifierValue string `json:"value"`
}

type CreateServerRequest struct {
	DiscordId       string           `json:"discord_id"`
	RefreshToken    string           `json:"refresh_token,omitempty"`
	Name            string           `json:"name"`
	Port            string           `json:"port"`
	World           string           `json:"world"`
	Password        string           `json:"password"`
	EnableCrossplay bool             `json:"enable_crossplay"`
	Public          bool             `json:"public"`
	Modifiers       []ServerModifier `json:"modifiers"`
}

type ValheimDedicatedServer struct {
	WorldDetails   CreateServerRequest `json:"world_details"`
	PvcName        string              `json:"pvc_name"`
	DeploymentName string              `json:"deployment_name"`
	State          string              `json:"state"`
}

type CreateServerHandler struct{}

// HandleRequest Handles the /api/v1/server/create to create a new Valheim dedicated server container.
func (h *CreateServerHandler) HandleRequest(c *gin.Context, ctx context.Context) {
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
	// Update user info in Cognito with valheim server data.
	cognito := service.MakeCognitoService()
	log.Infof("authenticating user with discord id: %s", reqBody.DiscordId)
	user, err := cognito.AuthUser(ctx, &reqBody.RefreshToken, &reqBody.DiscordId)
	if err != nil {
		log.Errorf("could not authenticate user with refresh token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("could not authenticate user with refresh token: %s", err)})
		return
	}

	log.Infof("user authenticated: %s", user.Email)

	config := MakeServerConfigWithDefaults(reqBody.Name, reqBody.World, reqBody.Port, reqBody.Password, reqBody.EnableCrossplay, reqBody.Public, reqBody.Modifiers)
	valheimServer, err := CreateDedicatedServerDeployment(config, &reqBody)
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

func MakeServerConfigWithDefaults(name, world, port, password string, crossplay bool, public bool, modifiers []ServerModifier) *ServerConfig {
	// 2456 default port
	return &ServerConfig{
		Name:                  name,
		Port:                  port,
		World:                 world,
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
func CreateDedicatedServerDeployment(serverConfig *ServerConfig, request *CreateServerRequest) (*ValheimDedicatedServer, error) {
	// Ensures the refresh token doesn't get echo'd in the response
	request.RefreshToken = ""
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("could not create in cluster config: %v", err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

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
		string(rune(serverConfig.BackupCount)),
		"-backupshort",
		string(rune(serverConfig.InitialBackupSeconds)),
		"-backuplong",
		string(rune(serverConfig.BackupIntervalSeconds)),
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
	pvcName := fmt.Sprintf("valheim-pvc-%s", serverConfig.InstanceId)
	deploymentName := fmt.Sprintf("valheim-%s", serverConfig.InstanceId)
	now := time.Now().Format(time.RFC3339)

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
						"app":        "valheim",
						"created-by": deploymentName,
						"created-at": now,
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
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "valheim-plugin-data",
									MountPath: "/valheim/BepInEx/plugins/",
									SubPath:   "plugins",
								},
								{
									Name:      "valheim-server-data",
									MountPath: "/root/.config/unity3d/IronGate/Valheim",
								},
								{
									Name:      "irongate",
									MountPath: "/irongate",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "valheim-plugin-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
						{
							Name:         "valheim-server-data",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
						{
							Name:         "irongate",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					},
				},
			},
		},
	}

	// We only need a persistent volume for the plugins that will be installed. Need to shut down server, mount
	// pvc to a Job, install plugins to pvc, restart server, re-mount pvc
	// Game files like backups and world files will be (eventually) persisted to s3 by
	// the sidecar container so EmptyDir{} can be used for those.
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: "hearthhub",
			Labels: map[string]string{
				"app":        "valheim",
				"created-by": deploymentName,
				"created-at": now,
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

	// Create Deployment and PVC in the cluster
	pvcCreateResult, err := clientset.CoreV1().PersistentVolumeClaims("hearthhub").Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error creating pvc: %v", err)
		return nil, err
	}

	log.Infof("created PVC: %s", pvcCreateResult.Name)

	result, err := clientset.AppsV1().Deployments("hearthhub").Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("error creating deployment: %v", err)
		return nil, err
	}
	log.Infof("created deployment %q in namespace %q\n", result.GetObjectMeta().GetName(), result.GetObjectMeta().GetNamespace())

	return &ValheimDedicatedServer{
		WorldDetails:   *request,
		PvcName:        pvcCreateResult.GetName(),
		DeploymentName: result.GetObjectMeta().GetName(),
		State:          RUNNING,
	}, nil
}

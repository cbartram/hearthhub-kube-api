package server

import (
	"fmt"
	"github.com/cbartram/hearthhub-mod-api/server/util"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
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

	// Server states
	RUNNING    = "running"
	TERMINATED = "terminated"
)

type Config struct {
	Name                  string     `json:"name"`
	World                 string     `json:"world"`
	CpuRequest            int        `json:"cpu_requests"`
	MemoryRequest         int        `json:"memory_requests"`
	Port                  string     `json:"port"`
	Password              string     `json:"password"`
	EnableCrossplay       bool       `json:"enable_crossplay"`
	Public                bool       `json:"public"`
	InstanceId            string     `json:"instance_id"`
	Modifiers             []Modifier `json:"modifiers"`
	SaveIntervalSeconds   int        `json:"save_interval_seconds"`
	BackupCount           int        `json:"backup_count"`
	InitialBackupSeconds  int        `json:"initial_backup_seconds"`
	BackupIntervalSeconds int        `json:"backup_interval_seconds"`
}

type Modifier struct {
	ModifierKey   string `json:"key"`
	ModifierValue string `json:"value"`
}

// MakeConfigWithDefaults creates a new ServerConfig with default values
// that can be selectively overridden by provided options
func MakeConfigWithDefaults(options *CreateServerRequest) *Config {
	cpuLimit, _ := strconv.Atoi(os.Getenv("CPU_LIMIT"))
	memLimit, _ := strconv.Atoi(os.Getenv("MEMORY_LIMIT"))

	config := &Config{
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
		Modifiers:             []Modifier{},
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

func (c *Config) ToStringArgs() string {
	var sb strings.Builder
	args := fmt.Sprintf("/valheim/valheim_server.x86_64 -name %s -port %s -world %s -password %s -instanceid %s -backups %s -backupshort %s -backuplong %s ",
		c.Name, c.Port, c.World, c.Password, c.InstanceId, strconv.Itoa(c.BackupCount), strconv.Itoa(c.InitialBackupSeconds), strconv.Itoa(c.BackupIntervalSeconds))

	if c.EnableCrossplay {
		sb.WriteString("-crossplay ")
	}

	if c.Public {
		sb.WriteString("-public 1 ")
	} else {
		sb.WriteString("-public 0 ")
	}

	for _, modifier := range c.Modifiers {
		sb.WriteString(fmt.Sprintf("-modifier %s %s ", modifier.ModifierKey, modifier.ModifierValue))
	}

	// Write the logs to a shared mount on the pvc so that the sidecar can tail these looking
	// for the join code.
	sb.WriteString("-logFile /valheim/BepInEx/config/server-logs.txt")
	return args + sb.String()
}

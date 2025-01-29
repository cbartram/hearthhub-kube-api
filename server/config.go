package server

import (
	"github.com/cbartram/hearthhub-mod-api/server/util"
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

	// Server states
	RUNNING    = "running"
	TERMINATED = "terminated"
)

type Config struct {
	Name                  string     `json:"name"`
	World                 string     `json:"world"`
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

func (c *Config) ToStringArgs() []string {
	serverArgs := []string{
		"./valheim_server.x86_64",
		"-name",
		c.Name,
		"-port",
		c.Port,
		"-world",
		c.World,
		"-password",
		c.Password,
		"-instanceid",
		c.InstanceId,
		"-backups",
		strconv.Itoa(c.BackupCount),
		"-backupshort",
		strconv.Itoa(c.InitialBackupSeconds),
		"-backuplong",
		strconv.Itoa(c.BackupIntervalSeconds),
	}

	if c.EnableCrossplay {
		serverArgs = append(serverArgs, "-crossplay")
	}

	if c.Public {
		serverArgs = append(serverArgs, "-public", "1")
	} else {
		serverArgs = append(serverArgs, "-public", "0")
	}

	for _, modifier := range c.Modifiers {
		serverArgs = append(serverArgs, "-modifier", modifier.ModifierKey, modifier.ModifierValue)
	}

	return serverArgs
}

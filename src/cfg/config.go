package cfg

import (
	"fmt"
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
	sb.WriteString("-logFile /valheim/BepInEx/config/src-logs.txt")
	return args + sb.String()
}

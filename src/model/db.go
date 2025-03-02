package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"os"
	"time"

	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:               fmt.Sprintf("%s:%s@tcp(%s:3306)/hearthhub?charset=utf8mb4&parseTime=True&loc=Local", os.Getenv("MYSQL_USER"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_HOST")),
		DefaultStringSize: 256,
	}), &gorm.Config{})

	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	err = db.AutoMigrate(
		&User{},
		&Server{},
		&BackupFile{},
		&ConfigFile{},
		&ModFile{},
		&WorldFile{},
	)

	if err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}

	return db
}

// WorldDetails represents the nested JSON structure for server world configuration
type WorldDetails struct {
	Name                  string   `json:"name"`
	World                 string   `json:"world"`
	CPURequests           int      `json:"cpu_requests"`
	MemoryRequests        int      `json:"memory_requests"`
	Port                  string   `json:"port"`
	Password              string   `json:"password"`
	EnableCrossplay       bool     `json:"enable_crossplay"`
	Public                bool     `json:"public"`
	InstanceID            string   `json:"instance_id"`
	Modifiers             []string `json:"modifiers"`
	SaveIntervalSeconds   int      `json:"save_interval_seconds"`
	BackupCount           int      `json:"backup_count"`
	InitialBackupSeconds  int      `json:"initial_backup_seconds"`
	BackupIntervalSeconds int      `json:"backup_interval_seconds"`
}

// Value implements the driver.Valuer interface for WorldDetails
func (wd WorldDetails) Value() (driver.Value, error) {
	return json.Marshal(wd)
}

// Scan implements the sql.Scanner interface for WorldDetails
func (wd *WorldDetails) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, &wd)
}

// CognitoCredentials stores user authentication data
type CognitoCredentials struct {
	RefreshToken    string `json:"refresh_token,omitempty"`
	TokenExpiration int32  `json:"token_expiration_seconds,omitempty"`
	AccessToken     string `json:"access_token,omitempty"`
	IdToken         string `json:"id_token,omitempty"`
}

// Value implements the driver.Valuer interface for CognitoCredentials
func (cc CognitoCredentials) Value() (driver.Value, error) {
	return json.Marshal(cc)
}

// Scan implements the sql.Scanner interface for CognitoCredentials
func (cc *CognitoCredentials) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, &cc)
}

// User represents a user of the system
type User struct {
	ID              uint               `gorm:"primaryKey" json:"id"`
	CognitoID       string             `gorm:"column:cognito_id;uniqueIndex" json:"cognitoId,omitempty"`
	DiscordUsername string             `gorm:"column:discord_username" json:"discordUsername,omitempty"`
	Email           string             `gorm:"column:email;uniqueIndex" json:"email,omitempty"`
	AvatarId        string             `gorm:"column:avatar_id" json:"avatarId"`
	Enabled         bool               `gorm:"column:enabled" json:"enabled"`
	DiscordID       string             `gorm:"column:discord_id" json:"discordId,omitempty"`
	CustomerId      string             `gorm:"column:customer_id" json:"customerId,omitempty"`
	SubscriptionId  string             `gorm:"column:subscription_id" json:"subscriptionId"`
	Credentials     CognitoCredentials `gorm:"type:json" json:"credentials,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	DeletedAt       gorm.DeletedAt     `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	Servers     []Server     `gorm:"foreignKey:UserID" json:"servers,omitempty"`
	ModFiles    []ModFile    `gorm:"foreignKey:UserID" json:"mod_files,omitempty"`
	ConfigFiles []ConfigFile `gorm:"foreignKey:UserID" json:"config_files,omitempty"`
	BackupFiles []BackupFile `gorm:"foreignKey:UserID" json:"backup_files,omitempty"`
	WorldFiles  []WorldFile  `gorm:"foreignKey:UserID" json:"world_files,omitempty"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

// Server represents a Valheim game server
type Server struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         uint           `gorm:"column:user_id;index" json:"user_id"`
	ServerIP       string         `gorm:"column:server_ip" json:"server_ip"`
	ServerPort     int            `gorm:"column:server_port" json:"server_port"`
	ServerMemory   int            `gorm:"column:server_memory" json:"server_memory"`
	ServerCPU      int            `gorm:"column:server_cpu" json:"server_cpu"`
	CPULimit       int            `gorm:"column:cpu_limit" json:"cpu_limit"`
	MemoryLimit    int            `gorm:"column:memory_limit" json:"memory_limit"`
	WorldDetails   WorldDetails   `gorm:"type:json" json:"world_details"`
	ModPVCName     string         `gorm:"column:mod_pvc_name" json:"mod_pvc_name"`
	DeploymentName string         `gorm:"column:deployment_name" json:"deployment_name"`
	State          string         `gorm:"column:state" json:"state"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	User       User        `gorm:"foreignKey:UserID" json:"-"`
	WorldFiles []WorldFile `gorm:"foreignKey:ServerID" json:"world_files,omitempty"`
}

// TableName overrides the table name
func (Server) TableName() string {
	return "servers"
}

// BaseFile contains common fields for all file types
type BaseFile struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"column:user_id;index" json:"user_id"`
	FileName  string         `gorm:"column:file_name;uniqueIndex:idx_file_user" json:"file_name"`
	Installed bool           `gorm:"column:installed" json:"installed"`
	S3Key     string         `gorm:"column:s3_key" json:"s3_key"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Relations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// BackupFile represents a server backup file
type BackupFile struct {
	BaseFile
}

// TableName overrides the table name
func (BackupFile) TableName() string {
	return "backup_files"
}

// ConfigFile represents a server configuration file
type ConfigFile struct {
	BaseFile
}

// TableName overrides the table name
func (ConfigFile) TableName() string {
	return "config_files"
}

// ModFile represents a server mod file
type ModFile struct {
	BaseFile
	UpVotes            int       `gorm:"column:upvotes" json:"upvotes"`
	Downloads          int       `gorm:"column:downloads" json:"downloads"`
	OriginalUploadDate time.Time `gorm:"column:original_upload_date" json:"original_upload_date"`
	LatestUploadDate   time.Time `gorm:"column:latest_upload_date" json:"latest_upload_date"`
	Creator            string    `json:"creator"`
	HeroImage          string    `json:"hero_image"`
	Description        string    `json:"description"`
}

// TableName overrides the table name
func (ModFile) TableName() string {
	return "mod_files"
}

// WorldFile represents a world save file
type WorldFile struct {
	BaseFile
	ServerID uint `gorm:"column:server_id;index" json:"server_id"`

	// Relations
	Server Server `gorm:"foreignKey:ServerID" json:"-"`
}

// TableName overrides the table name
func (WorldFile) TableName() string {
	return "world_files"
}

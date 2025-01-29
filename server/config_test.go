package server

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func TestMakeConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name     string
		options  *CreateServerRequest
		expected *Config
	}{
		{
			name: "All defaults",
			options: &CreateServerRequest{
				Name:     stringPtr("MyServer"),
				World:    stringPtr("MyWorld"),
				Password: stringPtr("MyPassword"),
			},
			expected: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "2456",
				Password:              "MyPassword",
				EnableCrossplay:       false,
				Public:                false,
				InstanceId:            "", // InstanceId is generated dynamically, so we skip checking it
				SaveIntervalSeconds:   1800,
				BackupCount:           3,
				InitialBackupSeconds:  7200,
				BackupIntervalSeconds: 43200,
				Modifiers:             []Modifier{},
			},
		},
		{
			name: "Override all options",
			options: &CreateServerRequest{
				Name:                  stringPtr("MyServer"),
				World:                 stringPtr("MyWorld"),
				Password:              stringPtr("MyPassword"),
				Port:                  stringPtr("1234"),
				EnableCrossplay:       boolPtr(true),
				Public:                boolPtr(true),
				Modifiers:             []Modifier{{ModifierKey: "mod1", ModifierValue: "value1"}},
				SaveIntervalSeconds:   intPtr(900),
				BackupCount:           intPtr(5),
				InitialBackupSeconds:  intPtr(3600),
				BackupIntervalSeconds: intPtr(86400),
			},
			expected: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "1234",
				Password:              "MyPassword",
				EnableCrossplay:       true,
				Public:                true,
				InstanceId:            "", // InstanceId is generated dynamically, so we skip checking it
				SaveIntervalSeconds:   900,
				BackupCount:           5,
				InitialBackupSeconds:  3600,
				BackupIntervalSeconds: 86400,
				Modifiers:             []Modifier{{ModifierKey: "mod1", ModifierValue: "value1"}},
			},
		},
		{
			name: "Partial overrides",
			options: &CreateServerRequest{
				Name:                  stringPtr("MyServer"),
				World:                 stringPtr("MyWorld"),
				Password:              stringPtr("MyPassword"),
				Port:                  stringPtr("1234"),
				EnableCrossplay:       boolPtr(true),
				Modifiers:             []Modifier{{ModifierKey: "mod1", ModifierValue: "value1"}},
				BackupIntervalSeconds: intPtr(86400),
			},
			expected: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "1234",
				Password:              "MyPassword",
				EnableCrossplay:       true,
				Public:                false,
				InstanceId:            "", // InstanceId is generated dynamically, so we skip checking it
				SaveIntervalSeconds:   1800,
				BackupCount:           3,
				InitialBackupSeconds:  7200,
				BackupIntervalSeconds: 86400,
				Modifiers:             []Modifier{{ModifierKey: "mod1", ModifierValue: "value1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeConfigWithDefaults(tt.options)

			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.World, result.World)
			assert.Equal(t, tt.expected.Port, result.Port)
			assert.Equal(t, tt.expected.Password, result.Password)
			assert.Equal(t, tt.expected.EnableCrossplay, result.EnableCrossplay)
			assert.Equal(t, tt.expected.Public, result.Public)
			assert.Equal(t, tt.expected.SaveIntervalSeconds, result.SaveIntervalSeconds)
			assert.Equal(t, tt.expected.BackupCount, result.BackupCount)
			assert.Equal(t, tt.expected.InitialBackupSeconds, result.InitialBackupSeconds)
			assert.Equal(t, tt.expected.BackupIntervalSeconds, result.BackupIntervalSeconds)
			assert.Equal(t, tt.expected.Modifiers, result.Modifiers)

			// Check that InstanceId is not empty (since it's dynamically generated)
			assert.NotEmpty(t, result.InstanceId)
		})
	}
}

func TestToStringArgs(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected []string
	}{
		{
			name: "All fields with defaults",
			config: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "2456",
				Password:              "MyPassword",
				InstanceId:            "12345",
				EnableCrossplay:       false,
				Public:                false,
				BackupCount:           3,
				InitialBackupSeconds:  7200,
				BackupIntervalSeconds: 43200,
				Modifiers:             []Modifier{},
			},
			expected: []string{
				"./valheim_server.x86_64",
				"-name", "MyServer",
				"-port", "2456",
				"-world", "MyWorld",
				"-password", "MyPassword",
				"-instanceid", "12345",
				"-backups", "3",
				"-backupshort", "7200",
				"-backuplong", "43200",
				"-public", "0",
			},
		},
		{
			name: "Enable crossplay and public server",
			config: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "2456",
				Password:              "MyPassword",
				InstanceId:            "12345",
				EnableCrossplay:       true,
				Public:                true,
				BackupCount:           3,
				InitialBackupSeconds:  7200,
				BackupIntervalSeconds: 43200,
				Modifiers:             []Modifier{},
			},
			expected: []string{
				"./valheim_server.x86_64",
				"-name", "MyServer",
				"-port", "2456",
				"-world", "MyWorld",
				"-password", "MyPassword",
				"-instanceid", "12345",
				"-backups", "3",
				"-backupshort", "7200",
				"-backuplong", "43200",
				"-crossplay",
				"-public", "1",
			},
		},
		{
			name: "With modifiers",
			config: &Config{
				Name:                  "MyServer",
				World:                 "MyWorld",
				Port:                  "2456",
				Password:              "MyPassword",
				InstanceId:            "12345",
				EnableCrossplay:       false,
				Public:                false,
				BackupCount:           3,
				InitialBackupSeconds:  7200,
				BackupIntervalSeconds: 43200,
				Modifiers: []Modifier{
					{ModifierKey: "mod1", ModifierValue: "value1"},
					{ModifierKey: "mod2", ModifierValue: "value2"},
				},
			},
			expected: []string{
				"./valheim_server.x86_64",
				"-name", "MyServer",
				"-port", "2456",
				"-world", "MyWorld",
				"-password", "MyPassword",
				"-instanceid", "12345",
				"-backups", "3",
				"-backupshort", "7200",
				"-backuplong", "43200",
				"-public", "0",
				"-modifier", "mod1", "value1",
				"-modifier", "mod2", "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := tt.config.ToStringArgs()

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

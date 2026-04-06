package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	JAMFSchool JAMFSchoolConfig `yaml:"jamf_school"`
	SnipeIT    SnipeITConfig    `yaml:"snipe_it"`
	Sync       SyncConfig       `yaml:"sync"`
	Log        LogConfig        `yaml:"log"`
}

type JAMFSchoolConfig struct {
	URL       string `yaml:"url"`
	NetworkID string `yaml:"network_id"`
	APIKey    string `yaml:"api_key"`
}

type SnipeITConfig struct {
	URL             string `yaml:"url"`
	APIKey          string `yaml:"api_key"`
	ManufacturerID  int    `yaml:"manufacturer_id"`
	DefaultStatusID int    `yaml:"default_status_id"`
	CategoryID      int    `yaml:"category_id"`
	FieldsetID      int    `yaml:"fieldset_id"`
}

type SyncConfig struct {
	DryRun             bool                 `yaml:"dry_run"`
	Force              bool                 `yaml:"force"`
	RateLimit          bool                 `yaml:"rate_limit"`
	UpdateOnly         bool                 `yaml:"update_only"`
	SetName            bool                 `yaml:"set_name"`
	DeviceTypes        []string             `yaml:"device_types"`
	FieldMapping       map[string]string    `yaml:"field_mapping"`
	LocationSync       LocationSyncConfig   `yaml:"location_sync"`
	BetaUserAssignment UserAssignmentConfig `yaml:"beta_user_assignment"`
}

type LocationSyncConfig struct {
	Enabled    bool           `yaml:"enabled"`
	AutoCreate bool           `yaml:"auto_create"`
	Mapping    map[string]int `yaml:"mapping"`
}

type UserAssignmentConfig struct {
	Beta            bool     `yaml:"beta"`
	Enabled         bool     `yaml:"enabled"`
	MatchOn         string   `yaml:"match_on"`
	DeviceSources   []string `yaml:"device_sources"`
	RequireJamfUser bool     `yaml:"require_jamf_user"`
}

type LogConfig struct {
	File   string `yaml:"file"`
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if v := os.Getenv("JAMFSCHOOL_URL"); v != "" {
		cfg.JAMFSchool.URL = v
	}
	if v := os.Getenv("JAMFSCHOOL_NETWORK_ID"); v != "" {
		cfg.JAMFSchool.NetworkID = v
	}
	if v := os.Getenv("JAMFSCHOOL_API_KEY"); v != "" {
		cfg.JAMFSchool.APIKey = v
	}
	if v := os.Getenv("SNIPEIT_URL"); v != "" {
		cfg.SnipeIT.URL = v
	}
	if v := os.Getenv("SNIPEIT_API_KEY"); v != "" {
		cfg.SnipeIT.APIKey = v
	}

	if cfg.Sync.BetaUserAssignment.MatchOn == "" {
		cfg.Sync.BetaUserAssignment.MatchOn = "auto"
	}
	if len(cfg.Sync.BetaUserAssignment.DeviceSources) == 0 {
		cfg.Sync.BetaUserAssignment.DeviceSources = []string{"notes"}
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if err := c.ValidateJAMFSchool(); err != nil {
		return err
	}
	if err := c.ValidateSnipeIT(); err != nil {
		return err
	}
	if c.Sync.BetaUserAssignment.Enabled && !c.Sync.BetaUserAssignment.Beta {
		return fmt.Errorf("sync.beta_user_assignment.enabled requires sync.beta_user_assignment.beta=true because user assignment is a beta heuristic feature")
	}
	return nil
}

func (c *Config) ValidateJAMFSchool() error {
	if c.JAMFSchool.URL == "" {
		return fmt.Errorf("jamf_school.url is required")
	}
	if c.JAMFSchool.NetworkID == "" {
		return fmt.Errorf("jamf_school.network_id is required")
	}
	if c.JAMFSchool.APIKey == "" {
		return fmt.Errorf("jamf_school.api_key is required")
	}
	return nil
}

func (c *Config) ValidateSnipeIT() error {
	if c.SnipeIT.URL == "" {
		return fmt.Errorf("snipe_it.url is required")
	}
	if c.SnipeIT.APIKey == "" {
		return fmt.Errorf("snipe_it.api_key is required")
	}
	if c.SnipeIT.ManufacturerID == 0 {
		return fmt.Errorf("snipe_it.manufacturer_id is required")
	}
	if c.SnipeIT.DefaultStatusID == 0 {
		return fmt.Errorf("snipe_it.default_status_id is required")
	}
	if c.SnipeIT.CategoryID == 0 {
		return fmt.Errorf("snipe_it.category_id is required")
	}
	return nil
}

func (c *SyncConfig) DeviceTypeAllowed(deviceType string) bool {
	if len(c.DeviceTypes) == 0 {
		return true
	}
	for _, allowed := range c.DeviceTypes {
		if strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(deviceType)) {
			return true
		}
	}
	return false
}

func (c *SyncConfig) LocationMappingID(keys ...string) (int, bool) {
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if id, ok := c.LocationSync.Mapping[key]; ok {
			return id, true
		}
		if id, ok := c.LocationSync.Mapping[strings.ToLower(key)]; ok {
			return id, true
		}
	}
	return 0, false
}

func (c *SyncConfig) LocationMappingKey(id int64, name string) []string {
	keys := []string{}
	if id != 0 {
		keys = append(keys, strconv.FormatInt(id, 10))
	}
	if name != "" {
		keys = append(keys, name, strings.ToLower(name))
	}
	return keys
}

func MergeFieldMapping(path string, newMappings map[string]string, replaceValues map[string]bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping at root")
	}

	syncNode := findOrCreateMapping(root, "sync")
	fieldMappingNode := findOrCreateMapping(syncNode, "field_mapping")

	if len(replaceValues) > 0 {
		var kept []*yaml.Node
		for i := 0; i < len(fieldMappingNode.Content)-1; i += 2 {
			if !replaceValues[fieldMappingNode.Content[i+1].Value] {
				kept = append(kept, fieldMappingNode.Content[i], fieldMappingNode.Content[i+1])
			}
		}
		fieldMappingNode.Content = kept
	}

	existing := make(map[string]bool)
	for i := 0; i < len(fieldMappingNode.Content)-1; i += 2 {
		existing[fieldMappingNode.Content[i].Value] = true
	}

	for dbCol, source := range newMappings {
		if dbCol == "" || source == "" || existing[dbCol] {
			continue
		}
		fieldMappingNode.Content = append(fieldMappingNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: dbCol, Tag: "!!str"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: source, Tag: "!!str"},
		)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

func findOrCreateMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(parent.Content)-1; i += 2 {
		if parent.Content[i].Value == key {
			val := parent.Content[i+1]
			if val.Kind != yaml.MappingNode {
				val.Kind = yaml.MappingNode
				val.Tag = "!!map"
				val.Value = ""
				val.Content = nil
			}
			return val
		}
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valNode)
	return valNode
}

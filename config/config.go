package config

import (
	"errors"
	"fmt"
	"net/url"
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
	var errs []string

	if err := c.ValidateJAMFSchool(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := c.ValidateSnipeIT(); err != nil {
		errs = append(errs, err.Error())
	}
	if c.Sync.BetaUserAssignment.Enabled && !c.Sync.BetaUserAssignment.Beta {
		errs = append(errs, "sync.beta_user_assignment.enabled is true but beta is false - set beta: true to acknowledge this is a heuristic feature")
	}
	if len(errs) > 0 {
		return errors.New("configuration errors:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

func (c *Config) ValidateJAMFSchool() error {
	var errs []string
	if c.JAMFSchool.URL == "" {
		errs = append(errs, "jamf_school.url is required (or set JAMFSCHOOL_URL)")
	} else if err := validateHTTPS(c.JAMFSchool.URL, "jamf_school.url"); err != nil {
		errs = append(errs, err.Error())
	}
	if c.JAMFSchool.NetworkID == "" {
		errs = append(errs, "jamf_school.network_id is required (or set JAMFSCHOOL_NETWORK_ID)")
	}
	if c.JAMFSchool.APIKey == "" || c.JAMFSchool.APIKey == "your-jamf-school-api-key" {
		errs = append(errs, "jamf_school.api_key is required (or set JAMFSCHOOL_API_KEY)")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (c *Config) ValidateSnipeIT() error {
	var errs []string
	if c.SnipeIT.URL == "" {
		errs = append(errs, "snipe_it.url is required (or set SNIPEIT_URL)")
	} else if err := validateHTTPS(c.SnipeIT.URL, "snipe_it.url"); err != nil {
		errs = append(errs, err.Error())
	}
	if c.SnipeIT.APIKey == "" || c.SnipeIT.APIKey == "your-snipe-it-api-key" {
		errs = append(errs, "snipe_it.api_key is required (or set SNIPEIT_API_KEY)")
	}
	if c.SnipeIT.ManufacturerID <= 0 {
		errs = append(errs, "snipe_it.manufacturer_id must be a positive integer")
	}
	if c.SnipeIT.DefaultStatusID <= 0 {
		errs = append(errs, "snipe_it.default_status_id must be a positive integer")
	}
	if c.SnipeIT.CategoryID <= 0 {
		errs = append(errs, "snipe_it.category_id must be a positive integer")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validateHTTPS(rawURL, field string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", field, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s must use https (got %q) - credentials must not be sent over plain HTTP", field, u.Scheme)
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

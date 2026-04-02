package sync

import (
	"context"
	"fmt"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/Jamf-Concepts/jamfschool-go-sdk/jamfschool"
	snipeit "github.com/michellepellon/go-snipeit"
	"github.com/sirupsen/logrus"

	"github.com/jamescrawford/jamfschool2snipeIT/config"
	"github.com/jamescrawford/jamfschool2snipeIT/snipe"
)

var log = logrus.New()

func SetLogLevel(level logrus.Level) {
	log.SetLevel(level)
}

func SetLogFormatter(formatter logrus.Formatter) {
	log.SetFormatter(formatter)
}

func SetLogOutput(output io.Writer) {
	log.SetOutput(output)
}

type Stats struct {
	Total    int
	Created  int
	Updated  int
	Skipped  int
	Errors   int
	ModelNew int
}

type assignmentCandidate struct {
	Value string
	User  *jamfschool.User
}

type Engine struct {
	jamf            *jamfschool.Client
	snipe           *snipe.Client
	cfg             *config.Config
	stats           Stats
	models          map[string]int
	snipeLocations  map[string]int
	jamfLocations   map[int64]jamfschool.Location
	snipeUsersEmail map[string]int
	snipeUsersUser  map[string]int
	jamfUsersEmail  map[string]jamfschool.User
	jamfUsersUser   map[string]jamfschool.User
}

func NewEngine(jamfClient *jamfschool.Client, snipeClient *snipe.Client, cfg *config.Config) *Engine {
	return &Engine{
		jamf:            jamfClient,
		snipe:           snipeClient,
		cfg:             cfg,
		models:          make(map[string]int),
		snipeLocations:  make(map[string]int),
		jamfLocations:   make(map[int64]jamfschool.Location),
		snipeUsersEmail: make(map[string]int),
		snipeUsersUser:  make(map[string]int),
		jamfUsersEmail:  make(map[string]jamfschool.User),
		jamfUsersUser:   make(map[string]jamfschool.User),
	}
}

func (e *Engine) RunSingle(ctx context.Context, serial string) (*Stats, error) {
	if err := e.prepare(ctx); err != nil {
		return nil, err
	}

	devices, err := e.fetchDevices(ctx)
	if err != nil {
		return nil, err
	}
	for _, device := range devices {
		if strings.EqualFold(device.SerialNumber, serial) {
			if err := e.processDevice(ctx, device); err != nil {
				e.stats.Errors++
				return &e.stats, err
			}
			return &e.stats, nil
		}
	}
	return nil, fmt.Errorf("device %s not found in JAMF School", serial)
}

func (e *Engine) Run(ctx context.Context) (*Stats, error) {
	log.Info("Starting jamfschool2snipeIT sync")

	if err := e.prepare(ctx); err != nil {
		return nil, err
	}

	devices, err := e.fetchDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching JAMF School devices: %w", err)
	}
	log.Infof("Fetched %d devices from JAMF School", len(devices))

	for _, device := range devices {
		if err := ctx.Err(); err != nil {
			return &e.stats, err
		}
		if err := e.processDevice(ctx, device); err != nil {
			log.WithError(err).WithField("serial", device.SerialNumber).Error("Failed to process device")
			e.stats.Errors++
		}
	}

	return &e.stats, nil
}

func (e *Engine) prepare(ctx context.Context) error {
	if err := e.loadModels(ctx); err != nil {
		return fmt.Errorf("loading snipe models: %w", err)
	}
	if e.cfg.Sync.LocationSync.Enabled {
		if err := e.loadLocations(ctx); err != nil {
			return fmt.Errorf("loading locations: %w", err)
		}
	}
	if e.cfg.Sync.UserAssignment.Enabled {
		if err := e.loadUsers(ctx); err != nil {
			return fmt.Errorf("loading users: %w", err)
		}
	}
	return nil
}

func (e *Engine) loadModels(ctx context.Context) error {
	models, err := e.snipe.ListAllModels(ctx)
	if err != nil {
		return err
	}
	for _, model := range models {
		if model.Name != "" {
			e.models[strings.ToLower(model.Name)] = model.ID
		}
		if model.ModelNumber != "" {
			e.models[strings.ToLower(model.ModelNumber)] = model.ID
		}
	}
	return nil
}

func (e *Engine) loadLocations(ctx context.Context) error {
	jamfLocations, err := e.jamf.GetLocations(ctx)
	if err != nil {
		return err
	}
	for _, location := range jamfLocations {
		e.jamfLocations[location.ID] = location
	}

	snipeLocations, err := e.snipe.ListAllLocations(ctx)
	if err != nil {
		return err
	}
	for _, location := range snipeLocations {
		e.snipeLocations[strings.ToLower(strings.TrimSpace(location.Name))] = location.ID
	}
	return nil
}

func (e *Engine) loadUsers(ctx context.Context) error {
	jamfUsers, err := e.jamf.GetUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range jamfUsers {
		if user.Email != "" {
			e.jamfUsersEmail[strings.ToLower(user.Email)] = user
		}
		if user.Username != "" {
			e.jamfUsersUser[strings.ToLower(user.Username)] = user
		}
	}

	snipeUsers, err := e.snipe.ListAllUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range snipeUsers {
		if user.Email != "" {
			e.snipeUsersEmail[strings.ToLower(user.Email)] = user.ID
		}
		if user.Username != "" {
			e.snipeUsersUser[strings.ToLower(user.Username)] = user.ID
		}
	}
	return nil
}

func (e *Engine) fetchDevices(ctx context.Context) ([]jamfschool.Device, error) {
	devices, err := e.jamf.GetDevices(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]jamfschool.Device, 0, len(devices))
	for _, device := range devices {
		if !e.cfg.Sync.DeviceTypeAllowed(device.Model.Type) {
			continue
		}
		filtered = append(filtered, device)
	}
	return filtered, nil
}

func (e *Engine) processDevice(ctx context.Context, device jamfschool.Device) error {
	e.stats.Total++

	serial := strings.TrimSpace(device.SerialNumber)
	if serial == "" {
		e.stats.Skipped++
		return nil
	}

	logger := log.WithFields(logrus.Fields{
		"serial": serial,
		"udid":   device.UDID,
	})

	existing, err := e.snipe.GetAssetBySerial(ctx, serial)
	if err != nil {
		return err
	}
	if existing.Total == 0 && e.cfg.Sync.UpdateOnly {
		logger.Info("Skipping asset not found in Snipe-IT (update_only mode)")
		e.stats.Skipped++
		return nil
	}

	locationID, locationName, err := e.resolveLocation(ctx, device)
	if err != nil {
		logger.WithError(err).Warn("Could not resolve location, continuing without it")
	}
	userID, matchedUser := e.resolveUser(device)

	switch existing.Total {
	case 0:
		modelID, err := e.ensureModel(ctx, device)
		if err != nil {
			return err
		}
		return e.createAsset(ctx, logger, device, modelID, locationID, userID, matchedUser, locationName)
	case 1:
		return e.updateAsset(ctx, logger, device, &existing.Rows[0], locationID, userID, matchedUser, locationName)
	default:
		logger.Warnf("Multiple assets (%d) found for serial, skipping", existing.Total)
		e.stats.Skipped++
		return nil
	}
}

func (e *Engine) ensureModel(ctx context.Context, device jamfschool.Device) (int, error) {
	candidates := []string{
		strings.TrimSpace(device.Model.Identifier),
		strings.TrimSpace(device.Model.Name),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if id, ok := e.models[strings.ToLower(candidate)]; ok {
			return id, nil
		}
	}

	modelName := firstNonEmpty(device.Model.Name, device.Name)
	modelNumber := firstNonEmpty(device.Model.Identifier, modelName)
	if modelName == "" {
		return 0, fmt.Errorf("device %q has no model name or identifier", device.SerialNumber)
	}

	if e.cfg.Sync.UpdateOnly {
		return 0, fmt.Errorf("model %q not found (update_only mode)", modelName)
	}
	if e.cfg.Sync.DryRun {
		log.WithFields(logrus.Fields{
			"model_name":   modelName,
			"model_number": modelNumber,
		}).Info("[DRY RUN] Would create model")
		e.stats.ModelNew++
		return 0, nil
	}

	model := snipeit.Model{
		CommonFields: snipeit.CommonFields{Name: modelName},
		ModelNumber:  modelNumber,
		Category: snipeit.Category{
			CommonFields: snipeit.CommonFields{ID: e.cfg.SnipeIT.CategoryID},
		},
		Manufacturer: snipeit.Manufacturer{
			CommonFields: snipeit.CommonFields{ID: e.cfg.SnipeIT.ManufacturerID},
		},
		FieldsetID: e.cfg.SnipeIT.FieldsetID,
	}

	newModel, err := e.snipe.CreateModel(ctx, model)
	if err != nil {
		return 0, err
	}
	e.models[strings.ToLower(modelName)] = newModel.ID
	e.models[strings.ToLower(modelNumber)] = newModel.ID
	e.stats.ModelNew++
	return newModel.ID, nil
}

func (e *Engine) resolveLocation(ctx context.Context, device jamfschool.Device) (int, string, error) {
	if !e.cfg.Sync.LocationSync.Enabled {
		return 0, "", nil
	}

	location, ok := e.jamfLocations[device.LocationID]
	if !ok {
		return 0, "", fmt.Errorf("jamf location %d not found", device.LocationID)
	}

	keys := e.cfg.Sync.LocationMappingKey(location.ID, location.Name)
	if id, ok := e.cfg.Sync.LocationMappingID(keys...); ok {
		return id, location.Name, nil
	}

	nameKey := strings.ToLower(strings.TrimSpace(location.Name))
	if id, ok := e.snipeLocations[nameKey]; ok {
		return id, location.Name, nil
	}

	if !e.cfg.Sync.LocationSync.AutoCreate {
		return 0, location.Name, fmt.Errorf("location %q not found in Snipe-IT and auto_create is disabled", location.Name)
	}

	if e.cfg.Sync.DryRun {
		log.WithField("location", location.Name).Info("[DRY RUN] Would create location")
		return 0, location.Name, nil
	}

	snipeLocation := snipeit.Location{
		CommonFields: snipeit.CommonFields{Name: location.Name},
	}
	if location.Street != nil {
		snipeLocation.Address = *location.Street
	}
	if location.City != nil {
		snipeLocation.City = *location.City
	}
	if location.PostalCode != nil {
		snipeLocation.Zip = *location.PostalCode
	}

	created, err := e.snipe.CreateLocation(ctx, snipeLocation)
	if err != nil {
		return 0, location.Name, err
	}
	e.snipeLocations[nameKey] = created.ID
	return created.ID, location.Name, nil
}

func (e *Engine) resolveUser(device jamfschool.Device) (int, string) {
	if !e.cfg.Sync.UserAssignment.Enabled {
		return 0, ""
	}

	candidates := e.userCandidates(device)
	for _, candidate := range candidates {
		switch strings.ToLower(e.cfg.Sync.UserAssignment.MatchOn) {
		case "email":
			if id, ok := e.findUserByEmail(candidate); ok {
				return id, candidate.Value
			}
		case "username":
			if id, ok := e.findUserByUsername(candidate); ok {
				return id, candidate.Value
			}
		default:
			if id, ok := e.findUserByEmail(candidate); ok {
				return id, candidate.Value
			}
			if id, ok := e.findUserByUsername(candidate); ok {
				return id, candidate.Value
			}
		}
	}

	return 0, ""
}

func (e *Engine) userCandidates(device jamfschool.Device) []assignmentCandidate {
	seen := make(map[string]bool)
	out := []assignmentCandidate{}

	for _, source := range e.cfg.Sync.UserAssignment.DeviceSources {
		for _, value := range extractIdentifiersFromDevice(device, source) {
			key := strings.ToLower(value)
			if value == "" || seen[key] {
				continue
			}
			seen[key] = true

			candidate := assignmentCandidate{Value: value}
			if user, ok := e.jamfUsersEmail[key]; ok {
				candidate.User = &user
			}
			if candidate.User == nil {
				if user, ok := e.jamfUsersUser[key]; ok {
					candidate.User = &user
				}
			}

			if e.cfg.Sync.UserAssignment.RequireJamfUser && candidate.User == nil {
				continue
			}
			out = append(out, candidate)
		}
	}

	return out
}

func (e *Engine) findUserByEmail(candidate assignmentCandidate) (int, bool) {
	if candidate.User != nil && candidate.User.Email != "" {
		if id, ok := e.snipeUsersEmail[strings.ToLower(candidate.User.Email)]; ok {
			return id, true
		}
	}
	if id, ok := e.snipeUsersEmail[strings.ToLower(candidate.Value)]; ok {
		return id, true
	}
	return 0, false
}

func (e *Engine) findUserByUsername(candidate assignmentCandidate) (int, bool) {
	if candidate.User != nil && candidate.User.Username != "" {
		if id, ok := e.snipeUsersUser[strings.ToLower(candidate.User.Username)]; ok {
			return id, true
		}
	}
	if id, ok := e.snipeUsersUser[strings.ToLower(candidate.Value)]; ok {
		return id, true
	}
	return 0, false
}

func (e *Engine) createAsset(ctx context.Context, logger *logrus.Entry, device jamfschool.Device, modelID, locationID, userID int, matchedUser, locationName string) error {
	asset := snipeit.Asset{
		CommonFields: snipeit.CommonFields{
			CustomFields: make(map[string]string),
		},
		Serial:   device.SerialNumber,
		AssetTag: firstNonEmpty(device.AssetTag, device.SerialNumber),
		Model: snipeit.Model{
			CommonFields: snipeit.CommonFields{ID: modelID},
		},
		StatusLabel: snipeit.StatusLabel{
			CommonFields: snipeit.CommonFields{ID: e.cfg.SnipeIT.DefaultStatusID},
		},
	}

	if e.cfg.Sync.SetName {
		asset.Name = firstNonEmpty(device.Name, device.Model.Name, device.SerialNumber)
	}
	if locationID > 0 {
		asset.Location = snipeit.Location{CommonFields: snipeit.CommonFields{ID: locationID}}
	}
	if userID > 0 {
		asset.User = &snipeit.FlexUser{User: snipeit.User{CommonFields: snipeit.CommonFields{ID: userID}}}
		asset.AssignedType = "user"
	}

	e.applyFieldMapping(&asset, device, matchedUser, locationName)
	applyManagedNotes(&asset, device, matchedUser, locationName)

	if e.cfg.Sync.DryRun {
		logger.WithField("payload", asset).Info("[DRY RUN] Would create asset")
		e.stats.Created++
		return nil
	}

	if _, err := e.snipe.CreateAsset(ctx, asset); err != nil {
		return err
	}

	logger.Info("Created asset in Snipe-IT")
	e.stats.Created++
	return nil
}

func (e *Engine) updateAsset(ctx context.Context, logger *logrus.Entry, device jamfschool.Device, existing *snipeit.Asset, locationID, userID int, matchedUser, locationName string) error {
	desired := snipeit.Asset{
		CommonFields: snipeit.CommonFields{
			CustomFields: make(map[string]string),
			Notes:        existing.Notes,
		},
		AssetTag: firstNonEmpty(device.AssetTag, device.SerialNumber),
	}

	if e.cfg.Sync.SetName {
		desired.Name = firstNonEmpty(device.Name, device.Model.Name, device.SerialNumber)
	}
	if locationID > 0 {
		desired.Location = snipeit.Location{CommonFields: snipeit.CommonFields{ID: locationID}}
	}
	if userID > 0 {
		desired.User = &snipeit.FlexUser{User: snipeit.User{CommonFields: snipeit.CommonFields{ID: userID}}}
		desired.AssignedType = "user"
	}

	e.applyFieldMapping(&desired, device, matchedUser, locationName)
	applyManagedNotes(&desired, device, matchedUser, locationName)

	update := &desired
	if !e.cfg.Sync.Force {
		update = e.diffAsset(&desired, existing)
		if update == nil {
			logger.Debug("All fields already match, skipping update")
			e.stats.Skipped++
			return nil
		}
	}

	if e.cfg.Sync.DryRun {
		logger.WithField("payload", update).Info("[DRY RUN] Would update asset")
		e.stats.Updated++
		return nil
	}

	if _, err := e.snipe.PatchAsset(ctx, existing.ID, *update); err != nil {
		return err
	}

	logger.Info("Updated asset in Snipe-IT")
	e.stats.Updated++
	return nil
}

func (e *Engine) diffAsset(desired *snipeit.Asset, existing *snipeit.Asset) *snipeit.Asset {
	diff := snipeit.Asset{
		CommonFields: snipeit.CommonFields{
			CustomFields: make(map[string]string),
		},
	}
	hasChanges := false

	if desired.Name != "" && desired.Name != existing.Name {
		diff.Name = desired.Name
		hasChanges = true
	}
	if desired.AssetTag != "" && desired.AssetTag != existing.AssetTag {
		diff.AssetTag = desired.AssetTag
		hasChanges = true
	}
	if desired.Location.ID != 0 && desired.Location.ID != existing.Location.ID {
		diff.Location = desired.Location
		hasChanges = true
	}
	if desired.User != nil && desired.User.User.ID != 0 {
		currentID := 0
		if existing.User != nil {
			currentID = existing.User.User.ID
		}
		if desired.User.User.ID != currentID {
			diff.User = desired.User
			diff.AssignedType = desired.AssignedType
			hasChanges = true
		}
	}
	if desired.Notes != "" && desired.Notes != html.UnescapeString(existing.Notes) {
		diff.Notes = desired.Notes
		hasChanges = true
	}
	for key, value := range desired.CustomFields {
		if html.UnescapeString(existing.CustomFields[key]) != value {
			diff.CustomFields[key] = value
			hasChanges = true
		}
	}

	if !hasChanges {
		return nil
	}
	return &diff
}

func (e *Engine) applyFieldMapping(asset *snipeit.Asset, device jamfschool.Device, matchedUser, locationName string) {
	for snipeField, sourceField := range e.cfg.Sync.FieldMapping {
		value := fieldValue(device, sourceField, matchedUser, locationName)
		if value == "" {
			continue
		}
		switch snipeField {
		case "asset_tag":
			asset.AssetTag = value
		case "name":
			asset.Name = value
		default:
			asset.CustomFields[snipeField] = value
		}
	}
}

func fieldValue(device jamfschool.Device, sourceField, matchedUser, locationName string) string {
	switch strings.ToLower(strings.TrimSpace(sourceField)) {
	case "serial_number", "serialnumber":
		return device.SerialNumber
	case "asset_tag", "assettag":
		return device.AssetTag
	case "name":
		return device.Name
	case "udid":
		return device.UDID
	case "notes":
		return device.Notes
	case "last_checkin", "lastcheckin":
		return device.LastCheckin
	case "model_name", "modelname":
		return device.Model.Name
	case "model_identifier", "modelidentifier":
		return device.Model.Identifier
	case "model_type", "modeltype":
		return device.Model.Type
	case "os_name", "osprefix":
		return device.OS.Prefix
	case "os_version", "osversion":
		return device.OS.Version
	case "device_enroll_type", "deviceenrolltype":
		return device.DeviceEnrollType
	case "battery_level", "batterylevel":
		if device.BatteryLevel == 0 {
			return ""
		}
		return strconv.FormatFloat(device.BatteryLevel*100, 'f', 0, 64) + "%"
	case "total_capacity", "totalcapacity":
		if device.TotalCapacity == 0 {
			return ""
		}
		return strconv.FormatFloat(device.TotalCapacity, 'f', -1, 64)
	case "location_id", "locationid":
		if device.LocationID == 0 {
			return ""
		}
		return strconv.FormatInt(device.LocationID, 10)
	case "location_name", "locationname":
		return locationName
	case "managed", "ismanaged":
		return strconv.FormatBool(device.IsManaged)
	case "supervised", "issupervised":
		return strconv.FormatBool(device.IsSupervised)
	case "matched_user", "user_match":
		return matchedUser
	}
	return ""
}

const (
	managedNotesStart = "=== jamfschool2snipeIT:managed-start ==="
	managedNotesEnd   = "=== jamfschool2snipeIT:managed-end ==="
)

func applyManagedNotes(asset *snipeit.Asset, device jamfschool.Device, matchedUser, locationName string) {
	rows := []string{
		"JAMF School Device Summary",
		fmt.Sprintf("Name: %s", firstNonEmpty(device.Name, "N/A")),
		fmt.Sprintf("Serial: %s", firstNonEmpty(device.SerialNumber, "N/A")),
		fmt.Sprintf("UDID: %s", firstNonEmpty(device.UDID, "N/A")),
		fmt.Sprintf("Model: %s", firstNonEmpty(device.Model.Name, "N/A")),
		fmt.Sprintf("Model Identifier: %s", firstNonEmpty(device.Model.Identifier, "N/A")),
		fmt.Sprintf("Model Type: %s", firstNonEmpty(device.Model.Type, "N/A")),
		fmt.Sprintf("OS: %s %s", strings.TrimSpace(device.OS.Prefix), strings.TrimSpace(device.OS.Version)),
		fmt.Sprintf("Managed: %t", device.IsManaged),
		fmt.Sprintf("Supervised: %t", device.IsSupervised),
		fmt.Sprintf("Last Check-in: %s", firstNonEmpty(device.LastCheckin, "N/A")),
		fmt.Sprintf("Location: %s", firstNonEmpty(locationName, "N/A")),
	}
	if matchedUser != "" {
		rows = append(rows, fmt.Sprintf("Matched User: %s", matchedUser))
	}
	if device.Notes != "" {
		rows = append(rows, fmt.Sprintf("JAMF Notes: %s", device.Notes))
	}

	block := managedNotesStart + "\n" + strings.Join(rows, "\n") + "\n" + managedNotesEnd
	existing := asset.Notes
	startIdx := strings.Index(existing, managedNotesStart)
	if startIdx >= 0 {
		endIdx := strings.Index(existing[startIdx:], managedNotesEnd)
		if endIdx >= 0 {
			endIdx += startIdx
			before := strings.TrimSpace(existing[:startIdx])
			after := strings.TrimSpace(existing[endIdx+len(managedNotesEnd):])
			switch {
			case before != "" && after != "":
				asset.Notes = before + "\n\n" + block + "\n\n" + after
			case before != "":
				asset.Notes = before + "\n\n" + block
			case after != "":
				asset.Notes = block + "\n\n" + after
			default:
				asset.Notes = block
			}
			return
		}
	}
	if strings.TrimSpace(existing) == "" {
		asset.Notes = block
		return
	}
	asset.Notes = strings.TrimSpace(existing) + "\n\n" + block
}

var emailRegexp = regexp.MustCompile(`(?i)[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}`)

func extractIdentifiersFromDevice(device jamfschool.Device, source string) []string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "name":
		return tokenizeIdentifiers(device.Name)
	case "notes":
		return tokenizeIdentifiers(device.Notes)
	case "asset_tag", "assettag":
		return tokenizeIdentifiers(device.AssetTag)
	default:
		return nil
	}
}

func tokenizeIdentifiers(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	parts := []string{}
	for _, match := range emailRegexp.FindAllString(text, -1) {
		parts = append(parts, match)
	}

	separators := regexp.MustCompile(`[,\s;:()<>]+`)
	for _, token := range separators.Split(text, -1) {
		token = strings.TrimSpace(strings.Trim(token, "\"'"))
		if token == "" {
			continue
		}
		parts = append(parts, token)
	}
	return parts
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

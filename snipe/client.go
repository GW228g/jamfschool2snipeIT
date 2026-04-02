package snipe

import (
	"context"
	"fmt"
	"io"
	"strings"

	snipeit "github.com/michellepellon/go-snipeit"
	"github.com/sirupsen/logrus"
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

var ErrDryRun = fmt.Errorf("write blocked: dry-run mode is enabled")

type Client struct {
	*snipeit.Client
	DryRun bool
}

type FieldDef struct {
	Name        string
	Element     string
	Format      string
	HelpText    string
	FieldValues string
}

type snipeLogger struct{}

func (l *snipeLogger) LogRequest(method, url string, body []byte) {
	log.WithFields(logrus.Fields{"method": method, "url": url}).Debug("snipe-it request")
}

func (l *snipeLogger) LogResponse(method, url string, statusCode int, body []byte) {
	log.WithFields(logrus.Fields{"method": method, "url": url, "status": statusCode}).Debug("snipe-it response")
}

func NewClient(baseURL, apiKey string, rateLimit bool) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	opts := &snipeit.ClientOptions{Logger: &snipeLogger{}}
	if rateLimit {
		opts.RateLimiter = snipeit.NewTokenBucketRateLimiter(2, 5)
	}

	sc, err := snipeit.NewClientWithOptions(baseURL, apiKey, opts)
	if err != nil {
		return nil, fmt.Errorf("creating snipe-it client: %w", err)
	}
	return &Client{Client: sc}, nil
}

func (c *Client) ListAllModels(ctx context.Context) ([]snipeit.Model, error) {
	var all []snipeit.Model
	offset := 0
	limit := 500

	for {
		resp, _, err := c.Models.ListContext(ctx, &snipeit.ListOptions{Limit: limit, Offset: offset})
		if err != nil {
			return nil, fmt.Errorf("listing models: %w", err)
		}
		all = append(all, resp.Rows...)
		if len(all) >= resp.Total {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) ListAllLocations(ctx context.Context) ([]snipeit.Location, error) {
	var all []snipeit.Location
	offset := 0
	limit := 500

	for {
		resp, _, err := c.Locations.ListContext(ctx, &snipeit.ListOptions{Limit: limit, Offset: offset})
		if err != nil {
			return nil, fmt.Errorf("listing locations: %w", err)
		}
		all = append(all, resp.Rows...)
		if len(all) >= resp.Total {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) ListAllUsers(ctx context.Context) ([]snipeit.User, error) {
	var all []snipeit.User
	offset := 0
	limit := 500

	for {
		resp, _, err := c.Users.ListContext(ctx, &snipeit.ListOptions{Limit: limit, Offset: offset})
		if err != nil {
			return nil, fmt.Errorf("listing users: %w", err)
		}
		all = append(all, resp.Rows...)
		if len(all) >= resp.Total {
			break
		}
		offset += limit
	}
	return all, nil
}

func (c *Client) GetAssetBySerial(ctx context.Context, serial string) (*snipeit.AssetsResponse, error) {
	resp, _, err := c.Assets.GetAssetBySerialContext(ctx, serial)
	if err != nil {
		return nil, fmt.Errorf("looking up serial %s: %w", serial, err)
	}

	exact := resp.Rows[:0]
	for _, asset := range resp.Rows {
		if strings.EqualFold(asset.Serial, serial) {
			exact = append(exact, asset)
		}
	}
	resp.Rows = exact
	resp.Total = len(exact)
	return resp, nil
}

func (c *Client) CreateModel(ctx context.Context, model snipeit.Model) (*snipeit.Model, error) {
	if c.DryRun {
		return nil, ErrDryRun
	}
	resp, _, err := c.Models.CreateContext(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("creating model: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("creating model failed: %s", resp.Message)
	}
	return &resp.Payload, nil
}

func (c *Client) CreateLocation(ctx context.Context, location snipeit.Location) (*snipeit.Location, error) {
	if c.DryRun {
		return nil, ErrDryRun
	}
	resp, _, err := c.Locations.CreateContext(ctx, location)
	if err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("creating location failed: %s", resp.Message)
	}
	return &resp.Payload, nil
}

func (c *Client) CreateAsset(ctx context.Context, asset snipeit.Asset) (*snipeit.Asset, error) {
	if c.DryRun {
		return nil, ErrDryRun
	}
	resp, _, err := c.Assets.CreateContext(ctx, asset)
	if err != nil {
		return nil, fmt.Errorf("creating asset: %w", err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("creating asset failed: %s", resp.Message)
	}
	return &resp.Payload, nil
}

func (c *Client) PatchAsset(ctx context.Context, id int, asset snipeit.Asset) (*snipeit.Asset, error) {
	if c.DryRun {
		return nil, ErrDryRun
	}
	resp, _, err := c.Assets.PatchContext(ctx, id, asset)
	if err != nil {
		return nil, fmt.Errorf("updating asset %d: %w", id, err)
	}
	if resp.Status != "success" {
		return nil, fmt.Errorf("updating asset %d failed: %s", id, resp.Message)
	}
	return &resp.Payload, nil
}

func (c *Client) SetupFields(fieldsetID int, fields []FieldDef) (map[string]string, error) {
	if c.DryRun {
		return nil, ErrDryRun
	}

	existing, _, err := c.Fields.List(nil)
	if err != nil {
		return nil, fmt.Errorf("listing existing fields: %w", err)
	}
	existingByName := make(map[string]snipeit.Field)
	for _, field := range existing.Rows {
		existingByName[field.Name] = field
	}

	results := make(map[string]string)
	for _, def := range fields {
		field := snipeit.Field{
			CommonFields: snipeit.CommonFields{Name: def.Name},
			Element:      def.Element,
			Format:       def.Format,
			HelpText:     def.HelpText,
			FieldValues:  def.FieldValues,
		}

		var fieldID int
		var dbColumn string

		if existingField, ok := existingByName[def.Name]; ok {
			resp, _, err := c.Fields.Update(existingField.ID, field)
			if err != nil {
				return results, fmt.Errorf("updating field %q: %w", def.Name, err)
			}
			if resp.Status != "success" {
				return results, fmt.Errorf("updating field %q: %s", def.Name, resp.Message)
			}
			fieldID = resp.Payload.ID
			dbColumn = resp.Payload.DBColumnName
			if dbColumn == "" {
				dbColumn = existingField.DBColumnName
			}
		} else {
			resp, _, err := c.Fields.Create(field)
			if err != nil {
				return results, fmt.Errorf("creating field %q: %w", def.Name, err)
			}
			if resp.Status != "success" {
				return results, fmt.Errorf("creating field %q: %s", def.Name, resp.Message)
			}
			fieldID = resp.Payload.ID
			dbColumn = resp.Payload.DBColumnName
		}

		results[def.Name] = dbColumn
		if fieldsetID > 0 {
			if _, err := c.Fields.Associate(fieldID, fieldsetID); err != nil {
				return results, fmt.Errorf("associating field %q with fieldset %d: %w", def.Name, fieldsetID, err)
			}
		}
	}

	return results, nil
}

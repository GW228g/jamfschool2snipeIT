package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Jamf-Concepts/jamfschool-go-sdk/jamfschool"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jamescrawford/jamfschool2snipeIT/config"
	"github.com/jamescrawford/jamfschool2snipeIT/snipe"
	jsssync "github.com/jamescrawford/jamfschool2snipeIT/sync"
)

var (
	Cfg        *config.Config
	ConfigFile string
	Version    string

	verbose   bool
	debug     bool
	logFile   string
	logFormat string
	logFileFD *os.File
)

var log = logrus.New()

func LoadConfig(cmd *cobra.Command) error {
	var err error
	Cfg, err = config.Load(ConfigFile)
	if err != nil {
		if cmd.Flags().Changed("config") {
			return fmt.Errorf("loading config: %w", err)
		}
		Cfg = &config.Config{}
	}

	applyBoolFlag(cmd, "dry-run", &Cfg.Sync.DryRun)
	applyBoolFlag(cmd, "force", &Cfg.Sync.Force)
	applyBoolFlag(cmd, "update-only", &Cfg.Sync.UpdateOnly)

	effectiveDebug := debug
	effectiveVerbose := verbose
	effectiveLogFile := logFile
	effectiveLogFormat := logFormat

	if Cfg.Log.Level != "" && !cmd.Flags().Changed("debug") && !cmd.Flags().Changed("verbose") {
		switch strings.ToLower(Cfg.Log.Level) {
		case "debug":
			effectiveDebug = true
		case "info":
			effectiveVerbose = true
		}
	}
	if Cfg.Log.File != "" && !cmd.Flags().Changed("log-file") {
		effectiveLogFile = Cfg.Log.File
	}
	if Cfg.Log.Format != "" && !cmd.Flags().Changed("log-format") {
		effectiveLogFormat = Cfg.Log.Format
	}

	level := logrus.WarnLevel
	switch {
	case effectiveDebug:
		level = logrus.DebugLevel
	case effectiveVerbose:
		level = logrus.InfoLevel
	}
	setAllLogLevels(level)

	var formatter logrus.Formatter = &logrus.TextFormatter{FullTimestamp: true}
	switch strings.ToLower(effectiveLogFormat) {
	case "", "text":
	case "json":
		formatter = &logrus.JSONFormatter{}
	default:
		return fmt.Errorf("invalid log format %q: must be 'text' or 'json'", effectiveLogFormat)
	}
	setAllLogFormatters(formatter)

	setAllLogOutputs(os.Stderr)
	if logFileFD != nil {
		_ = logFileFD.Close()
		logFileFD = nil
	}
	if effectiveLogFile != "" {
		f, err := os.OpenFile(effectiveLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		logFileFD = f
		setAllLogOutputs(io.MultiWriter(os.Stderr, f))
	}

	return nil
}

func setAllLogLevels(level logrus.Level) {
	log.SetLevel(level)
	jsssync.SetLogLevel(level)
	snipe.SetLogLevel(level)
}

func setAllLogFormatters(formatter logrus.Formatter) {
	log.SetFormatter(formatter)
	jsssync.SetLogFormatter(formatter)
	snipe.SetLogFormatter(formatter)
}

func setAllLogOutputs(output io.Writer) {
	log.SetOutput(output)
	jsssync.SetLogOutput(output)
	snipe.SetLogOutput(output)
}

func applyBoolFlag(cmd *cobra.Command, name string, dst *bool) {
	if cmd.Flags().Changed(name) {
		*dst, _ = cmd.Flags().GetBool(name)
	}
}

func contextWithSignal() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			log.Infof("Received signal %v, shutting down...", sig)
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

type jamfLogger struct{}

func (l *jamfLogger) LogRequest(_ context.Context, method, url string, _ http.Header, _ []byte) {
	log.WithFields(logrus.Fields{"method": method, "url": url}).Debug("jamf school request")
}

func (l *jamfLogger) LogResponse(_ context.Context, statusCode int, _ http.Header, _ []byte) {
	log.WithField("status", statusCode).Debug("jamf school response")
}

func newJamfClient() *jamfschool.Client {
	log.Info("Connecting to JAMF School...")
	return jamfschool.NewClient(
		Cfg.JAMFSchool.URL,
		Cfg.JAMFSchool.NetworkID,
		Cfg.JAMFSchool.APIKey,
		jamfschool.WithUserAgent("jamfschool2snipeIT/"+Version),
		jamfschool.WithLogger(&jamfLogger{}),
	)
}

func newSnipeClient() (*snipe.Client, error) {
	log.Info("Connecting to Snipe-IT...")
	client, err := snipe.NewClient(Cfg.SnipeIT.URL, Cfg.SnipeIT.APIKey, Cfg.Sync.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("creating Snipe-IT client: %w", err)
	}
	client.DryRun = Cfg.Sync.DryRun
	return client, nil
}

func Execute() {
	rootCmd := &cobra.Command{
		Use:          "jamfschool2snipeIT",
		Short:        "Sync JAMF School devices into Snipe-IT",
		Long:         "jamfschool2snipeIT syncs JAMF School inventory into Snipe-IT asset management.",
		Version:      Version,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return LoadConfig(cmd)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if logFileFD != nil {
				_ = logFileFD.Close()
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&ConfigFile, "config", "settings.yaml", "Path to YAML config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output (INFO level)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug output (DEBUG level)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Append log output to this file (in addition to stderr)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format: text or json")

	syncCmd := NewSyncCmd()
	testCmd := NewTestCmd()
	setupCmd := NewSetupCmd()

	for _, cmd := range []*cobra.Command{syncCmd, setupCmd} {
		cmd.Flags().Bool("dry-run", false, "Simulate without making changes")
	}

	rootCmd.AddCommand(syncCmd, testCmd, setupCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

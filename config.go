// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2015-2016 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/decred/dcrd/dcrutil/v3"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultBaseURL               = "http://127.0.0.1:8000"
	defaultConfigFilename        = "testnetfaucet.conf"
	defaultDataDirname           = "data"
	defaultLogLevel              = "info"
	defaultLogDirname            = "logs"
	defaultLogFilename           = "testnetfaucet.log"
	defaultListen                = ":8000"
	defaultPublicPath            = "public"
	defaultTemplatePath          = "views"
	defaultWalletAccount         = "default"
	defaultWallertCert           = "~/.dcrwallet/rpc.cert"
	defaultWithdrawalAmount      = 2
	defaultWithdrawalTimeSeconds = 30
)

var (
	testnetFaucetHomeDir = dcrutil.AppDataDir("testnetfaucet", false)
	defaultConfigFile    = filepath.Join(testnetFaucetHomeDir, defaultConfigFilename)
	defaultDataDir       = filepath.Join(testnetFaucetHomeDir, defaultDataDirname)
	defaultLogDir        = filepath.Join(testnetFaucetHomeDir, defaultLogDirname)
	defaultWalletAddress = "TsfDLrRkk9ciUuwfp2b8PawwnukYD7yAjGd"
)

// runServiceCommand is only set to a real function on Windows.  It is used
// to parse and execute service commands specified via the -s flag.
var runServiceCommand func(string) error

// config defines the configuration options for dcrd.
//
// See loadConfig for details on the configuration load process.
type config struct {
	ShowVersion         bool    `short:"V" long:"version" description:"Display version information and exit"`
	ConfigFile          string  `short:"C" long:"configfile" description:"Path to configuration file"`
	DataDir             string  `short:"b" long:"datadir" description:"Directory to store data"`
	LogDir              string  `long:"logdir" description:"Directory to log output."`
	Listen              string  `long:"listen" description:"Listen for connections on the specified interface/port (default all interfaces port: 9113, testnet: 19113)"`
	TestNet             bool    `long:"testnet" description:"Use the test network"`
	SimNet              bool    `long:"simnet" description:"Use the simulation test network"`
	Profile             string  `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	CPUProfile          string  `long:"cpuprofile" description:"Write CPU profile to the specified file"`
	MemProfile          string  `long:"memprofile" description:"Write mem profile to the specified file"`
	DebugLevel          string  `short:"d" long:"debuglevel" description:"Logging level for all subsystems {trace, debug, info, warn, error, critical} -- You may also specify <subsystem>=<level>,<subsystem2>=<level>,... to set the log level for individual subsystems -- Use show to list available subsystems"`
	OverrideToken       string  `long:"overridetoken" description:"Secret override token to skip time check."`
	PublicPath          string  `long:"publicpath" description:"Path to the public folder which contains css/fonts/images/javascript."`
	TemplatePath        string  `long:"templatepath" description:"Path to the views folder which contains html files."`
	WalletAccount       string  `long:"walletaccount" description:"Account to send funds from."`
	WalletAddress       string  `long:"walletaddress" description:"Wallet address for returning coins."`
	WalletHost          string  `long:"wallethost" description:"Hostname for wallet server."`
	WalletUser          string  `long:"walletuser" description:"Username for wallet server."`
	WalletPassword      string  `long:"walletpassword" description:"Password for wallet server."`
	WalletCert          string  `long:"walletcert" description:"Certificate path for wallet server."`
	WithdrawalTimeLimit int64   `long:"withdrawaltimelimit" description:"Number of seconds before a second withdrawal can be made."`
	WithdrawalAmount    float64 `long:"withdrawalamount" description:"Amount of testnet DCR to send with each request."`
	Version             string

	withdrawalAmount    dcrutil.Amount
	withdrawalTimeLimit time.Duration
}

// serviceOptions defines the configuration options for the daemon as a service
// on Windows.
type serviceOptions struct {
	ServiceCommand string `short:"s" long:"service" description:"Service command {install, remove, start, stop}"`
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(testnetFaucetHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// validLogLevel returns whether or not logLevel is a valid debug log level.
func validLogLevel(logLevel string) bool {
	switch logLevel {
	case "trace":
		fallthrough
	case "debug":
		fallthrough
	case "info":
		fallthrough
	case "warn":
		fallthrough
	case "error":
		fallthrough
	case "critical":
		return true
	}
	return false
}

// supportedSubsystems returns a sorted slice of the supported subsystems for
// logging purposes.
func supportedSubsystems() []string {
	// Convert the subsystemLoggers map keys to a slice.
	subsystems := make([]string, 0, len(subsystemLoggers))
	for subsysID := range subsystemLoggers {
		subsystems = append(subsystems, subsysID)
	}

	// Sort the subsytems for stable display.
	sort.Strings(subsystems)
	return subsystems
}

// parseAndSetDebugLevels attempts to parse the specified debug level and set
// the levels accordingly.  An appropriate error is returned if anything is
// invalid.
func parseAndSetDebugLevels(debugLevel string) error {
	// When the specified string doesn't have any delimters, treat it as
	// the log level for all subsystems.
	if !strings.Contains(debugLevel, ",") && !strings.Contains(debugLevel, "=") {
		// Validate debug log level.
		if !validLogLevel(debugLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, debugLevel)
		}

		// Change the logging level for all subsystems.
		setLogLevels(debugLevel)

		return nil
	}

	// Split the specified string into subsystem/level pairs while detecting
	// issues and update the log levels accordingly.
	for _, logLevelPair := range strings.Split(debugLevel, ",") {
		if !strings.Contains(logLevelPair, "=") {
			str := "The specified debug level contains an invalid " +
				"subsystem/level pair [%v]"
			return fmt.Errorf(str, logLevelPair)
		}

		// Extract the specified subsystem and log level.
		fields := strings.Split(logLevelPair, "=")
		subsysID, logLevel := fields[0], fields[1]

		// Validate subsystem.
		if _, exists := subsystemLoggers[subsysID]; !exists {
			str := "The specified subsystem [%v] is invalid -- " +
				"supported subsytems %v"
			return fmt.Errorf(str, subsysID, supportedSubsystems())
		}

		// Validate log level.
		if !validLogLevel(logLevel) {
			str := "The specified debug level [%v] is invalid"
			return fmt.Errorf(str, logLevel)
		}

		setLogLevel(subsysID, logLevel)
	}

	return nil
}

// removeDuplicateAddresses returns a new slice with all duplicate entries in
// addrs removed.
func removeDuplicateAddresses(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	seen := map[string]struct{}{}
	for _, val := range addrs {
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = struct{}{}
		}
	}
	return result
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

// normalizeAddresses returns a new slice with all the passed peer addresses
// normalized with the given default port, and all duplicates removed.
func normalizeAddresses(addrs []string, defaultPort string) []string {
	for i, addr := range addrs {
		addrs[i] = normalizeAddress(addr, defaultPort)
	}

	return removeDuplicateAddresses(addrs)
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// newConfigParser returns a new command line flags parser.
func newConfigParser(cfg *config, so *serviceOptions, options flags.Options) *flags.Parser {
	parser := flags.NewParser(cfg, options)
	if runtime.GOOS == "windows" {
		parser.AddGroup("Service Options", "Service Options", so)
	}
	return parser
}

// loadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
//
// The above results in daemon functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func loadConfig() (*config, []string, error) {
	// Default config.
	cfg := config{
		ConfigFile:          defaultConfigFile,
		DebugLevel:          defaultLogLevel,
		DataDir:             defaultDataDir,
		LogDir:              defaultLogDir,
		Listen:              defaultListen,
		WalletAccount:       defaultWalletAccount,
		WalletAddress:       defaultWalletAddress,
		WalletCert:          defaultWallertCert,
		WithdrawalAmount:    defaultWithdrawalAmount,
		WithdrawalTimeLimit: defaultWithdrawalTimeSeconds,
		Version:             version(),
	}

	// Service options which are only added on Windows.
	serviceOpts := serviceOptions{}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := cfg
	preParser := newConfigParser(&preCfg, &serviceOpts, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version())
		os.Exit(0)
	}

	// Perform service command and exit if specified.  Invalid service
	// commands show an appropriate error.  Only runs on Windows since
	// the runServiceCommand function will be nil when not on Windows.
	if serviceOpts.ServiceCommand != "" && runServiceCommand != nil {
		err := runServiceCommand(serviceOpts.ServiceCommand)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(0)
	}

	// Load additional config from file.
	var configFileError error
	parser := newConfigParser(&cfg, &serviceOpts, flags.Default)
	if !(preCfg.SimNet) || preCfg.ConfigFile !=
		defaultConfigFile {

		err := flags.NewIniParser(parser).ParseFile(preCfg.ConfigFile)
		if err != nil {
			if _, ok := err.(*os.PathError); !ok {
				fmt.Fprintf(os.Stderr, "Error parsing config "+
					"file: %v\n", err)
				fmt.Fprintln(os.Stderr, usageMessage)
				return nil, nil, err
			}
			configFileError = err
		}
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, err := parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, nil, err
	}

	// Create the home directory if it doesn't already exist.
	funcName := "loadConfig"
	err = os.MkdirAll(testnetFaucetHomeDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		str := "%s: Failed to create home directory: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Count number of network flags passed; assign active network params
	// while we're at it
	activeNetParams = &testNet3Params

	// Append the network type to the data directory so it is "namespaced"
	// per network.  In addition to the block database, there are other
	// pieces of data that are saved to disk such as address manager state.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	cfg.DataDir = cleanAndExpandPath(cfg.DataDir)
	cfg.DataDir = filepath.Join(cfg.DataDir, netName(activeNetParams))

	// Append the network type to the log directory so it is "namespaced"
	// per network in the same fashion as the data directory.
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, netName(activeNetParams))

	cfg.WalletCert = cleanAndExpandPath(cfg.WalletCert)

	// Special show command to list supported subsystems and exit.
	if cfg.DebugLevel == "show" {
		fmt.Println("Supported subsystems", supportedSubsystems())
		os.Exit(0)
	}

	// Initialize log rotation.  After log rotation has been initialized, the
	// logger variables may be used.
	initLogRotator(filepath.Join(cfg.LogDir, defaultLogFilename))

	// Parse, validate, and set debug log level(s).
	if err := parseAndSetDebugLevels(cfg.DebugLevel); err != nil {
		err := fmt.Errorf("%s: %v", funcName, err.Error())
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usageMessage)
		return nil, nil, err
	}

	// Validate profile port number
	if cfg.Profile != "" {
		profilePort, err := strconv.Atoi(cfg.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			str := "%s: The profile port must be between 1024 and 65535"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, nil, err
		}
	}

	if cfg.OverrideToken == "" {
		str := "%s: OverrideToken is not set in config"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.WalletHost == "" {
		str := "%s: wallethost is not set in config"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.WalletCert == "" {
		str := "%s: walletcert is not set in config"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.WalletUser == "" {
		str := "%s: walletuser is not set in config"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.WalletPassword == "" {
		str := "%s: walletpassword is not set in config"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	cfg.withdrawalAmount, err = dcrutil.NewAmount(cfg.WithdrawalAmount)
	if err != nil || cfg.withdrawalAmount <= 0 {
		str := "%s: Invalid withdrawal amount: %v"
		err := fmt.Errorf(str, funcName, cfg.WithdrawalAmount)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.WithdrawalTimeLimit <= 0 {
		str := "%s: WithdrawalTimeLimit cannot be <= 0"
		err := fmt.Errorf(str, funcName)
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}
	cfg.withdrawalTimeLimit = time.Duration(cfg.WithdrawalTimeLimit) * time.Second

	// Add default wallet port for the active network if there's no port specified
	cfg.WalletHost = normalizeAddress(cfg.WalletHost, activeNetParams.WalletRPCServerPort)

	if !fileExists(cfg.WalletCert) {
		relativePath := filepath.Join(testnetFaucetHomeDir, cfg.WalletCert)
		if !fileExists(relativePath) {
			str := "%s: walletcert " + cfg.WalletCert + " and " +
				relativePath + " don't exist"
			err := fmt.Errorf(str, funcName)
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
	}

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		log.Warnf("%v", configFileError)
	}

	return &cfg, remainingArgs, nil
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the configuration struct for the service.
type Config struct {
	// Markets represents the tracked markets.
	Markets []string
	// FMPAPIkey is the FMP service API Key.
	FMPAPIKey string
	// Backtest is the backtesting flag.
	Backtest bool
	// BacktestDataFilepath is the filepath to the backtest data.
	BacktestDataFilepath string

	registeredFlags map[string]bool
}

// Validate asserts the config sane inputs.
func (cfg *Config) Validate() error {
	var errs error

	switch cfg.Backtest {
	case true:
		if cfg.BacktestDataFilepath == "" {
			errs = errors.Join(errs, fmt.Errorf("backtest data filepath cannot be an empty string"))
		}
	case false:
		if len(cfg.Markets) == 0 {
			errs = errors.Join(errs, fmt.Errorf("no markets provided for entry service"))
		}
		if cfg.FMPAPIKey == "" {
			errs = errors.Join(errs, fmt.Errorf("fmp api key cannot be an empty string"))
		}
	}

	return errs
}

// registerFlag registers command line arguments of any type and tracks them to avoid reregistration.
func (cfg *Config) registerFlag(name string, value interface{}, usage string) error {
	if cfg.registeredFlags == nil {
		cfg.registeredFlags = make(map[string]bool)
	}

	if cfg.registeredFlags[name] {
		return nil
	}

	cfg.registeredFlags[name] = true

	defValue := os.Getenv(name)
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("%s: value must be a non-nil pointer", name)
	}

	switch val.Elem().Kind() {
	case reflect.String:
		flag.StringVar(value.(*string), name, defValue, usage)
	case reflect.Bool:
		var def bool
		if defValue != "" {
			def, _ = strconv.ParseBool(defValue)
		}
		flag.BoolVar(value.(*bool), name, def, usage)
	case reflect.Int:
		var def int
		if defValue != "" {
			def, _ = strconv.Atoi(defValue)
		}
		flag.IntVar(value.(*int), name, def, usage)
	case reflect.Slice:
		// Only handle []string
		if val.Elem().Type().Elem().Kind() == reflect.String {
			var def []string
			if defValue != "" {
				def = strings.Split(defValue, ",")
			}
			flag.Func(name, usage, func(s string) error {
				*value.(*[]string) = strings.Split(s, ",")
				return nil
			})
			// Set default if not provided via flag
			if len(def) > 0 {
				*value.(*[]string) = def
			}
		} else {
			return fmt.Errorf("%s: unsupported slice type", name)
		}
	default:
		return fmt.Errorf("%s: unsupported type", name)
	}

	return nil
}

// loadConfig loads the configuration from environment variables and command line flags.
func loadConfig(cfg *Config, path string) error {
	if path == "" {
		path = ".env"
	}

	// Check if the expected .env file exists before loading it.
	_, err := os.Stat(path)
	if err == nil {
		err := godotenv.Load(path)
		if err != nil {
			return fmt.Errorf("loading .env file: %w", err)
		}
	}

	// Register command line arguments using loaded environment variables as defaults.
	err = cfg.registerFlag("markets", &cfg.Markets, "the tracked markets")
	if err != nil {
		return err
	}
	err = cfg.registerFlag("fmpapikey", &cfg.FMPAPIKey, "the FMP api key")
	if err != nil {
		return err
	}
	err = cfg.registerFlag("backtest", &cfg.Backtest, "the backtest flag")
	if err != nil {
		return err
	}
	err = cfg.registerFlag("backtestdatafilepath", &cfg.BacktestDataFilepath, "the backtest data filepath")
	if err != nil {
		return err
	}

	// Parse command-line flags.
	flag.Parse()

	return cfg.Validate()
}

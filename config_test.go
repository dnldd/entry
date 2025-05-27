package main

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr []string
	}{
		{
			name: "valid config, not backtest",
			cfg: Config{
				Markets:   []string{"AAPL", "GOOG"},
				FMPAPIKey: "apikey",
				Backtest:  false,
			},
			wantErr: nil,
		},
		{
			name: "missing markets, not backtest",
			cfg: Config{
				Markets:   []string{},
				FMPAPIKey: "apikey",
				Backtest:  false,
			},
			wantErr: []string{"no markets provided for entry service"},
		},
		{
			name: "missing FMPAPIKey, not backtest",
			cfg: Config{
				Markets:   []string{"AAPL"},
				FMPAPIKey: "",
				Backtest:  false,
			},
			wantErr: []string{"fmp api key cannot be an empty string"},
		},
		{
			name: "missing both markets and FMPAPIKey, not backtest",
			cfg: Config{
				Markets:   []string{},
				FMPAPIKey: "",
				Backtest:  false,
			},
			wantErr: []string{
				"no markets provided for entry service",
				"fmp api key cannot be an empty string",
			},
		},
		{
			name: "backtest true, valid filepath",
			cfg: Config{
				Backtest:             true,
				BacktestDataFilepath: "/tmp/data.csv",
			},
			wantErr: nil,
		},
		{
			name: "backtest true, missing filepath",
			cfg: Config{
				Backtest:             true,
				BacktestDataFilepath: "",
			},
			wantErr: []string{"backtest data filepath cannot be an empty string"},
		},
		{
			name: "backtest true, other fields missing",
			cfg: Config{
				Backtest:             true,
				BacktestDataFilepath: "/tmp/data.csv",
				Markets:              nil,
				FMPAPIKey:            "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error(s) %v, got none", tt.wantErr)
					return
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected error to contain %q, got %v", want, err)
					}
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Save and restore original os.Args and environment
	origArgs := os.Args
	origEnv := os.Environ()
	defer func() {
		os.Args = origArgs
		for _, kv := range origEnv {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}()

	tests := []struct {
		name        string
		env         map[string]string
		args        []string
		expectErr   bool
		expectInErr []string
		expectCfg   Config
	}{
		{
			name: "all from env, not backtest",
			env: map[string]string{
				"markets":   "AAPL,GOOG",
				"fmpapikey": "apikey",
				"backtest":  "false",
			},
			args:      []string{"cmd"},
			expectErr: false,
			expectCfg: Config{
				Markets:   []string{"AAPL", "GOOG"},
				FMPAPIKey: "apikey",
				Backtest:  false,
			},
		},
		{
			name:      "all from flags, not backtest",
			env:       map[string]string{},
			args:      []string{"cmd", "-markets=AAPL,GOOG", "-fmpapikey=apikey", "-backtest=false"},
			expectErr: false,
			expectCfg: Config{
				Markets:   []string{"AAPL", "GOOG"},
				FMPAPIKey: "apikey",
				Backtest:  false,
			},
		},
		{
			name:        "missing markets and fmpapikey",
			env:         map[string]string{},
			args:        []string{"cmd"},
			expectErr:   true,
			expectInErr: []string{"no markets provided for entry service", "fmp api key cannot be an empty string"},
		},
		{
			name: "backtest true, missing filepath",
			env: map[string]string{
				"backtest": "true",
			},
			args:        []string{"cmd"},
			expectErr:   true,
			expectInErr: []string{"backtest data filepath cannot be an empty string"},
		},
		{
			name: "backtest true, filepath from flag",
			env: map[string]string{
				"backtest": "true",
			},
			args:      []string{"cmd", "-backtestdatafilepath=/tmp/data.csv"},
			expectErr: false,
			expectCfg: Config{
				Backtest:             true,
				BacktestDataFilepath: "/tmp/data.csv",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// Set environment variables
			for k, v := range tt.env {
				os.Setenv(k, v)
			}

			// Set command-line arguments
			os.Args = tt.args

			var cfg Config
			err := loadConfig(&cfg, "") // don't load .env file

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				for _, want := range tt.expectInErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("expected error to contain %q, got %v", want, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				// Only check fields that are set in expectCfg
				if len(tt.expectCfg.Markets) != len(cfg.Markets) {
					t.Errorf("Markets: got %v (%d), want %v (%d)", cfg.Markets, len(tt.expectCfg.Markets), tt.expectCfg.Markets, len(cfg.Markets))
				}
				if tt.expectCfg.FMPAPIKey != "" && cfg.FMPAPIKey != tt.expectCfg.FMPAPIKey {
					t.Errorf("FMPAPIKey: got %v, want %v", cfg.FMPAPIKey, tt.expectCfg.FMPAPIKey)
				}
				if cfg.Backtest != tt.expectCfg.Backtest {
					t.Errorf("Backtest: got %v, want %v", cfg.Backtest, tt.expectCfg.Backtest)
				}
				if tt.expectCfg.BacktestDataFilepath != "" && cfg.BacktestDataFilepath != tt.expectCfg.BacktestDataFilepath {
					t.Errorf("BacktestDataFilepath: got %v, want %v", cfg.BacktestDataFilepath, tt.expectCfg.BacktestDataFilepath)
				}
			}

			// Clean up env
			for k := range tt.env {
				os.Unsetenv(k)
			}
		})
	}
}

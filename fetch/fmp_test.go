package fetch

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/dnldd/entry/shared"
	"github.com/peterldowns/testy/assert"
)

// todo: mock the http client and return valid data.

func TestFMPClient(t *testing.T) {
	// Ensure the fmp client can be created.
	cfg := &FMPConfig{
		APIKey:  "key",
		BaseURL: "http://base",
	}

	fc, err := NewFMPClient(cfg)
	assert.NoError(t, err)

	// Ensure urls can be formed accurately.
	params := url.Values{}
	params.Add("a", "bbb")
	params.Add("b", "ccc")

	path := "/path"
	formedUrl := fc.formURL(path, params.Encode())
	assert.Equal(t, formedUrl, "http://base/path?a=bbb&b=ccc")

	// Ensure fetching historical candles can fail if the client is not configured properly.

	market := "^GSPC"
	timeframe := shared.FiveMinute
	now, _, err := shared.NewYorkTime()
	assert.NoError(t, err)

	threeMonthsAgo := now.AddDate(0, -3, 0)
	_, err = fc.FetchIndexIntradayHistorical(context.Background(), market, timeframe, threeMonthsAgo, time.Time{})
	assert.Error(t, err)
}
func TestValidateFMPConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FMPConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     FMPConfig{APIKey: "key", BaseURL: "http://base"},
			wantErr: false,
		},
		{
			name:    "missing APIKey",
			cfg:     FMPConfig{APIKey: "", BaseURL: "http://base"},
			wantErr: true,
		},
		{
			name:    "missing BaseURL",
			cfg:     FMPConfig{APIKey: "key", BaseURL: ""},
			wantErr: true,
		},
		{
			name:    "missing both APIKey and BaseURL",
			cfg:     FMPConfig{APIKey: "", BaseURL: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/config"
	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

// oidcConfigRow is the single-row persistence of OIDCConfig (PK pinned at 1,
// same pattern as samlConfigRow).
//
// ClientSecret is stored in plaintext for simplicity; it is never returned
// in the admin GET response (the handler returns a has_client_secret
// boolean instead).
type oidcConfigRow struct {
	ID      int64 `gorm:"primaryKey"`
	Enabled bool

	IssuerURL    string `gorm:"size:255"`
	ClientID     string `gorm:"size:255"`
	ClientSecret string `gorm:"size:512"`
	RedirectURL  string `gorm:"size:255"`
	Scopes       jsonStrings

	AttrUsername    string `gorm:"size:128"`
	AttrEmail       string `gorm:"size:128"`
	AttrDisplayName string `gorm:"size:128"`
	AttrGroups      string `gorm:"size:128"`

	AdminGroupIDs    jsonStrings
	DefaultGroupSlug string `gorm:"size:64"`

	NewUserExpireDays         int
	NewUserTrafficLimitBytes  int64
	NewUserTrafficResetPeriod string `gorm:"size:16"`

	UpdatedAt time.Time
}

func (oidcConfigRow) TableName() string { return "oidc_config" }

func (r *oidcConfigRow) toDomain() *config.OIDCConfig {
	c := &config.OIDCConfig{
		Enabled:      r.Enabled,
		IssuerURL:    r.IssuerURL,
		ClientID:     r.ClientID,
		ClientSecret: r.ClientSecret,
		RedirectURL:  r.RedirectURL,
		Scopes:       []string(r.Scopes),
		AttributeMapping: config.OIDCAttributeMap{
			Username:    r.AttrUsername,
			Email:       r.AttrEmail,
			DisplayName: r.AttrDisplayName,
			Groups:      r.AttrGroups,
		},
		AdminGroupIDs:    []string(r.AdminGroupIDs),
		DefaultGroupSlug: r.DefaultGroupSlug,
		NewUserDefaults: config.SAMLNewUserDefaults{
			ExpireDays:         r.NewUserExpireDays,
			TrafficLimitBytes:  r.NewUserTrafficLimitBytes,
			TrafficResetPeriod: r.NewUserTrafficResetPeriod,
		},
	}
	config.ApplyOIDCDefaults(c)
	return c
}

func oidcConfigFromDomain(c *config.OIDCConfig) *oidcConfigRow {
	return &oidcConfigRow{
		ID:                        1,
		Enabled:                   c.Enabled,
		IssuerURL:                 c.IssuerURL,
		ClientID:                  c.ClientID,
		ClientSecret:              c.ClientSecret,
		RedirectURL:               c.RedirectURL,
		Scopes:                    jsonStrings(c.Scopes),
		AttrUsername:              c.AttributeMapping.Username,
		AttrEmail:                 c.AttributeMapping.Email,
		AttrDisplayName:           c.AttributeMapping.DisplayName,
		AttrGroups:                c.AttributeMapping.Groups,
		AdminGroupIDs:             jsonStrings(c.AdminGroupIDs),
		DefaultGroupSlug:          c.DefaultGroupSlug,
		NewUserExpireDays:         c.NewUserDefaults.ExpireDays,
		NewUserTrafficLimitBytes:  c.NewUserDefaults.TrafficLimitBytes,
		NewUserTrafficResetPeriod: c.NewUserDefaults.TrafficResetPeriod,
	}
}

type oidcConfigRepo struct{ db *gorm.DB }

func (r *oidcConfigRepo) Load(ctx context.Context) (*config.OIDCConfig, error) {
	var row oidcConfigRow
	if err := r.db.WithContext(ctx).First(&row, 1).Error; err != nil {
		err = wrapNotFound(err)
		if errors.Is(err, domain.ErrNotFound) {
			c := &config.OIDCConfig{Enabled: false}
			config.ApplyOIDCDefaults(c)
			return c, nil
		}
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *oidcConfigRepo) Save(ctx context.Context, c *config.OIDCConfig) error {
	return r.db.WithContext(ctx).Save(oidcConfigFromDomain(c)).Error
}

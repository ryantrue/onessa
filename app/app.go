package app

import (
	"context"
)

func Init(ctx context.Context, cfg Config) error {
	SetConfig(cfg)
	if err := InitDB(cfg.DataDir); err != nil {
		return err
	}
	EnsureLDAPDataLoaded(ctx)
	StartBackgroundLDAPSync(ctx)
	return nil
}

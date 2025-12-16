package app

import (
	"context"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/ryantrue/onessa/internal/logging"
)

var (
	ldapCronOnce sync.Once
)

// StartBackgroundLDAPSync запускает периодическую синхронизацию LDAP (пользователи + ПК).
// Повторный вызов безопасен (используется sync.Once).
func StartBackgroundLDAPSync(ctx context.Context) {
	ldapCronOnce.Do(func() {
		cfg := getConfig()
		if !ldapEnabled() {
			logging.Warnf("background ldap sync: LDAP is disabled")
			return
		}

		c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(loggingAdapter{})))

		// На старте — сразу делаем sync (по умолчанию включено).
		if cfg.LDAPSyncOnStartup {
			go func() {
				EnsureLDAPDataLoaded(ctx)
			}()
		}

		spec := "@every " + cfg.LDAPSyncEvery.String()
		if _, err := c.AddFunc(spec, func() {
			EnsureLDAPDataLoaded(ctx)
		}); err != nil {
			logging.Warnf("background ldap sync: cannot schedule %q: %v", spec, err)
			return
		}

		c.Start()
		logging.Infof("background ldap sync scheduled: %s", spec)

		go func() {
			<-ctx.Done()
			ctxStop := c.Stop()
			<-ctxStop.Done()
			logging.Infof("background ldap sync stopped")
		}()
	})
}

// robfig/cron ожидает интерфейс с Printf; адаптируемся к нашему логгеру.
type loggingAdapter struct{}

func (loggingAdapter) Printf(format string, args ...any) {
	logging.Infof(format, args...)
}

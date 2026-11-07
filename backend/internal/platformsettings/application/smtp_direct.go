package application

import (
	"bidora/backend/internal/platformsettings/domain"
	"context"
	"time"
)

// SMTP alone is an immediate, audited Owner setting. Other integrations retain
// their existing maker/checker activation workflow.
type DirectSMTPPublisher interface {
	PublishSMTP(context.Context, domain.Version, Head, domain.ID, domain.ID, time.Time) error
}

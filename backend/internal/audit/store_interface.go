package audit

import "time"

type Storer interface {
	Append(event AuditEvent) error
	ReadAll() ([]AuditEvent, error)
	QuerySince(since time.Time) ([]AuditEvent, error)
	AggregateThreatSources(since time.Time) ([]ThreatSourceRow, error)
	Close() error
}

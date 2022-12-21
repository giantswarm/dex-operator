package idp

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricNamespace = "dex_operator"
	metricSubsystem = "idp"
)

// Gauge for secret expiry time
var (
	infoLabels = []string{
		"app_name",
		"app_namespace",
		"app_owner",
		"provider_type",
		"provider_name",
		"app_registration_name",
	}

	AppInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: metricSubsystem,
			Name:      "secret_expiry_time",
			Help:      "Gives secret expiry time for all dex app registrations.",
		},
		infoLabels,
	)
)

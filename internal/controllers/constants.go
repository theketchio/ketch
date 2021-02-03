package controllers

import "time"

const (
	KetchNamespace = "ketch-system"
	// reconcileTimeout is the default timeout to trigger Operator reconcile
	reconcileTimeout = 10 * time.Minute
)

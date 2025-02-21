package logic

const (
	// ReadyCondition indicates the resource is ready and fully reconciled.
	// If the Condition is False, the resource SHOULD be considered to be in the process of reconciling and not a
	// representation of actual state.
	ReadyCondition string = "Ready"

	// ReconciliationSucceededReason represents the fact that the reconciliation succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"

	// ReconciliationFailedReason represents the fact that the reconciliation failed.
	ReconciliationFailedReason string = "ReconciliationFailed"

	// SetupFailedReason represents the fact that the setup failed for reconciliation.
	SetupFailedReason string = "SetupFailed"
)

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

	// NamespaceNotAllowedReason represents the fact that the Konfiguration is outside the privileged
	// namespaces, so the operator does not reconcile it.
	NamespaceNotAllowedReason string = "NamespaceNotAllowed"

	// InvalidReferenceReason represents the fact that the Konfiguration references a GitRepository,
	// KonfigurationSchema or destination namespace that is not a valid name.
	InvalidReferenceReason string = "InvalidReference"
)

package cmd

// Process exit codes.
const (
	// ExitOK indicates a clean run.
	ExitOK = 0
	// ExitGeneric indicates an unclassified error.
	ExitGeneric = 1
	// ExitUsage indicates a CLI usage error.
	ExitUsage = 2
	// ExitVault indicates a vault read or write failure.
	ExitVault = 3
	// ExitNotFound indicates a requested entry or day was not found.
	ExitNotFound = 4
	// ExitEditor indicates the user editor exited non-zero.
	ExitEditor = 5
)

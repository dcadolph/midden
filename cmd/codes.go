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
	// ExitLLM indicates an embedding or chat provider failure.
	ExitLLM = 6
	// ExitGit indicates a git repository operation failure.
	ExitGit = 7
)

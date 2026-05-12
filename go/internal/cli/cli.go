package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"mistral-file-sync/internal/api"
	"mistral-file-sync/internal/config"
	"mistral-file-sync/internal/models"
	"mistral-file-sync/internal/sync"
)

var (
	configFile string
	apiKey     string
	baseURL    string
	timeout    int
	rateLimit  float64
	maxRetries int
	verbose    bool
	debug      bool
)

var rootCmd = &cobra.Command{
	Use:   "mrlib",
	Short: "CLI for managing Mistral AI Libraries and Documents",
	Long: `mrlib is a command-line tool for managing Mistral AI Libraries and Documents
with advanced two-way synchronization capabilities.

Features:
- Create, read, update, delete libraries
- Upload, download, list, delete documents
- Two-way synchronization with multiple modes (mirror, additive, safe)
- File filtering by extension and patterns
- Rate limiting and retry logic
- Progress bars for large operations
- JSON output for scripting`,
}

var librariesCmd = &cobra.Command{
	Use:   "lib",
	Short: "Manage Mistral AI Libraries",
	Long:  "Commands for managing Mistral AI Libraries: list, get, create, update, delete",
}

var documentsCmd = &cobra.Command{
	Use:   "doc",
	Short: "Manage documents in libraries",
	Long:  "Commands for managing documents: list, get, upload, delete",
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize local files with Mistral libraries",
	Long:  "Two-way synchronization between local filesystem and Mistral AI Libraries",
}

func init() {
	// Root command flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Mistral API key")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "API base URL")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 120, "Request timeout in seconds")
	rootCmd.PersistentFlags().Float64Var(&rateLimit, "rate-limit-delay", 1.0, "Delay between requests in seconds")
	rootCmd.PersistentFlags().IntVar(&maxRetries, "max-retries", 3, "Maximum number of retries")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Debug mode")

	// Add subcommands
	rootCmd.AddCommand(librariesCmd, documentsCmd, syncCmd)

	// Libraries subcommands
	librariesCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List all libraries",
			RunE:  runLibrariesList,
		},
		&cobra.Command{
			Use:   "get [LIBRARY_ID]",
			Short: "Get a specific library",
			Args:  cobra.ExactArgs(1),
			RunE:  runLibrariesGet,
		},
		&cobra.Command{
			Use:   "create [NAME] --description [DESCRIPTION]",
			Short: "Create a new library",
			Args:  cobra.ExactArgs(1),
			RunE:  runLibrariesCreate,
		},
		&cobra.Command{
			Use:   "update [LIBRARY_ID] [NEW_NAME] --description [DESCRIPTION]",
			Short: "Update a library",
			Args:  cobra.RangeArgs(1, 2),
			RunE:  runLibrariesUpdate,
		},
		&cobra.Command{
			Use:   "delete [LIBRARY_ID] --force",
			Short: "Delete a library",
			Args:  cobra.ExactArgs(1),
			RunE:  runLibrariesDelete,
		},
	)

	// Documents subcommands
	documentsCmd.AddCommand(
		&cobra.Command{
			Use:   "list [LIBRARY_ID]",
			Short: "List documents in a library",
			Args:  cobra.ExactArgs(1),
			RunE:  runDocumentsList,
		},
		&cobra.Command{
			Use:   "get [LIBRARY_ID] [DOCUMENT_ID]",
			Short: "Get/download a document",
			Args:  cobra.ExactArgs(2),
			RunE:  runDocumentsGet,
		},
		&cobra.Command{
			Use:   "upload [LIBRARY_ID]",
			Short: "Upload a file to a library",
			Args:  cobra.ExactArgs(1),
			RunE:  runDocumentsUpload,
		},
		&cobra.Command{
			Use:   "delete [LIBRARY_ID] [DOCUMENT_ID]",
			Short: "Delete a document",
			Args:  cobra.ExactArgs(2),
			RunE:  runDocumentsDelete,
		},
	)

	// Add flags to documents commands after they're created
	for _, cmd := range documentsCmd.Commands() {
		switch cmd.Use {
		case "get [LIBRARY_ID] [DOCUMENT_ID]":
			cmd.Flags().String("output", "", "Output file path")
		case "upload [LIBRARY_ID]":
			cmd.Flags().String("file", "", "File to upload")
		case "delete [LIBRARY_ID] [DOCUMENT_ID]":
			cmd.Flags().Bool("force", false, "Force delete without confirmation")
		}
	}

	// Sync subcommands
	syncCmd.AddCommand(
		&cobra.Command{
			Use:   "once [LIBRARY_ID] [LOCAL_PATH]",
			Short: "Perform a one-time sync",
			Args:  cobra.ExactArgs(2),
			RunE:  runSyncOnce,
		},
		&cobra.Command{
			Use:   "continuous [LIBRARY_ID] [LOCAL_PATH] --interval [SECONDS]",
			Short: "Run continuous sync",
			Args:  cobra.ExactArgs(2),
			RunE:  runSyncContinuous,
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show sync status",
			RunE:  runSyncStatus,
		},
	)

	// Add flags to commands
	addSyncFlags(syncCmd)
}

func addSyncFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("direction", "both", "Sync direction: up, down, or both")
	cmd.PersistentFlags().String("mode", "safe", "Sync mode: mirror, additive, or safe")
	cmd.PersistentFlags().Int("batch-size", 10, "Batch size for uploads")
	cmd.PersistentFlags().Int("max-workers", 4, "Maximum parallel workers")
	cmd.PersistentFlags().Bool("dry-run", false, "Preview changes without applying")
	cmd.PersistentFlags().Bool("force", false, "Force re-upload even if hash matches")
	cmd.PersistentFlags().String("state-file", ".mistral_sync_state.json", "Path to sync state file")
	cmd.PersistentFlags().StringSlice("extensions", []string{}, "File extensions to include")
	cmd.PersistentFlags().StringSlice("exclude", []string{}, "Patterns to exclude")
	cmd.PersistentFlags().StringSlice("include", []string{}, "Patterns to include")
	cmd.PersistentFlags().Bool("json", false, "Output in JSON format")
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getClient() (*api.Client, error) {
	// Load config
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Override with command-line flags
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if timeout > 0 {
		cfg.Timeout = time.Duration(timeout) * time.Second
	}
	if rateLimit > 0 {
		cfg.RateLimitDelay = time.Duration(rateLimit) * time.Second
	}
	if maxRetries > 0 {
		cfg.MaxRetries = maxRetries
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required. Set MISTRAL_API_KEY environment variable or use --api-key flag")
	}

	return api.NewClient(
		cfg.APIKey,
		cfg.BaseURL,
		cfg.Timeout,
		cfg.RateLimitDelay,
		cfg.MaxRetries,
	), nil
}

// Libraries commands

func runLibrariesList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	libraries, err := client.ListLibraries()
	if err != nil {
		return err
	}

	if verbose {
		fmt.Printf("Found %d libraries\n", len(libraries))
	}

	// Output as table or JSON
	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(libraries)
	}

	// Simple table output
	fmt.Printf("% -36s %-20s %-8s %-10s\n", "ID", "Name", "Docs", "Size")
	fmt.Println(strings.Repeat("-", 80))
	for _, lib := range libraries {
		fmt.Printf("% -36s %-20s %-8d %-10s\n", 
			lib.ID, lib.Name, lib.NbDocuments, formatBytes(lib.TotalSize))
	}

	return nil
}

func runLibrariesGet(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	library, err := client.GetLibrary(args[0])
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(library)
	}

	fmt.Printf("ID: %s\n", library.ID)
	fmt.Printf("Name: %s\n", library.Name)
	if library.Description != nil {
		fmt.Printf("Description: %s\n", *library.Description)
	}
	fmt.Printf("Documents: %d\n", library.NbDocuments)
	fmt.Printf("Total Size: %s\n", formatBytes(library.TotalSize))
	fmt.Printf("Created: %s\n", library.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", library.UpdatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func runLibrariesCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	description, _ := cmd.Flags().GetString("description")
	library, err := client.CreateLibrary(args[0], description)
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(library)
	}

	fmt.Printf("Created library: %s (ID: %s)\n", library.Name, library.ID)
	return nil
}

func runLibrariesUpdate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	name := args[0]
	var description *string = nil
	if len(args) > 1 {
		name = args[1]
	}
	if desc, _ := cmd.Flags().GetString("description"); desc != "" {
		description = &desc
	}

	library, err := client.UpdateLibrary(args[0], name, description)
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(library)
	}

	fmt.Printf("Updated library: %s (ID: %s)\n", library.Name, library.ID)
	return nil
}

func runLibrariesDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Print("Are you sure you want to delete this library? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := client.DeleteLibrary(args[0]); err != nil {
		return err
	}

	fmt.Printf("Deleted library: %s\n", args[0])
	return nil
}

// Documents commands

func runDocumentsList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Resolve library name to ID
	libraryID, err := client.ResolveLibraryID(args[0])
	if err != nil {
		return err
	}

	documents, err := client.ListDocuments(libraryID)
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(documents)
	}

	fmt.Printf("% -36s %-20s %-10s %-12s\n", "ID", "Name", "Size", "Status")
	fmt.Println(strings.Repeat("-", 80))
	for _, doc := range documents {
		fmt.Printf("% -36s %-20s %-10s %-12s\n", 
			doc.ID, doc.Name, formatBytes(doc.Size), doc.ProcessStatus)
	}

	return nil
}

func runDocumentsGet(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Resolve library name to ID
	libraryID, err := client.ResolveLibraryID(args[0])
	if err != nil {
		return err
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "" {
		output = args[1] + ".download"
	}

	if err := client.DownloadDocument(libraryID, args[1], output); err != nil {
		return err
	}

	fmt.Printf("Downloaded to: %s\n", output)
	return nil
}

func runDocumentsUpload(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Resolve library name to ID
	libraryID, err := client.ResolveLibraryID(args[0])
	if err != nil {
		return err
	}

	filePath, _ := cmd.Flags().GetString("file")
	_ = filePath

	if filePath == "" {
		return fmt.Errorf("file path is required")
	}

	// If filename is specified, we need to handle it differently
	// For now, just use the filePath
	doc, err := client.UploadDocument(libraryID, filePath)
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(doc)
	}

	fmt.Printf("Uploaded: %s (ID: %s, Status: %s)\n", doc.Name, doc.ID, doc.ProcessStatus)
	return nil
}

func runDocumentsDelete(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Resolve library name to ID
	libraryID, err := client.ResolveLibraryID(args[0])
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Print("Are you sure you want to delete this document? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := client.DeleteDocument(libraryID, args[1]); err != nil {
		return err
	}

	fmt.Printf("Deleted document: %s\n", args[1])
	return nil
}

// Sync commands

func runSyncOnce(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	direction, _ := cmd.Flags().GetString("direction")
	mode, _ := cmd.Flags().GetString("mode")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	maxWorkers, _ := cmd.Flags().GetInt("max-workers")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	stateFile, _ := cmd.Flags().GetString("state-file")
	extensions, _ := cmd.Flags().GetStringSlice("extensions")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	include, _ := cmd.Flags().GetStringSlice("include")

	// Resolve library name to ID
	libraryID, err := client.ResolveLibraryID(args[0])
	if err != nil {
		return err
	}

	config := models.SyncConfig{
		LocalPath:      args[1],
		LibraryID:      libraryID,
		Direction:      models.SyncDirection(direction),
		Mode:          models.SyncMode(mode),
		BatchSize:     batchSize,
		MaxWorkers:    maxWorkers,
		DryRun:        dryRun,
		Force:         force,
		StateFile:     stateFile,
		Extensions:    extensions,
		ExcludePatterns: exclude,
		IncludePatterns: include,
	}

	syncer := sync.NewSyncer(client, config)
	result, err := syncer.SyncOnce()
	if err != nil {
		return err
	}

	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(result)
	}

	// Print summary
	fmt.Printf("Sync completed in %v\n", result.EndTime.Sub(result.StartTime))
	fmt.Printf("Added: %d, Updated: %d, Deleted: %d, Skipped: %d, Errors: %d\n",
		result.TotalAdded, result.TotalUpdated, result.TotalDeleted, result.TotalSkipped, result.TotalErrors)

	if len(result.Actions) > 0 && verbose {
		fmt.Println("\nActions:")
		for _, action := range result.Actions {
			fmt.Printf("  [%s] %s\n", action.ActionType, action.Path)
		}
	}

	return nil
}

func runSyncContinuous(cmd *cobra.Command, args []string) error {
	interval, _ := cmd.Flags().GetInt("interval")
	if interval <= 0 {
		interval = 60
	}

	fmt.Printf("Starting continuous sync every %d seconds...\n", interval)
	fmt.Println("Press Ctrl+C to stop")

	for {
		if err := runSyncOnce(cmd, args); err != nil {
			fmt.Printf("Sync error: %v\n", err)
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Get state file from flag or args
	stateFile, _ := cmd.Flags().GetString("state-file")
	if len(args) > 0 {
		stateFile = args[0]
	}
	if stateFile == "" {
		stateFile = ".mistral_sync_state.json"
	}

	// Try to load existing state
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No sync state found.")
			fmt.Println("Run 'sync once' first to create a sync state file.")
			return nil
		}
		return fmt.Errorf("failed to read state file: %v", err)
	}

	var state models.SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state file: %v", err)
	}

	// Get library name for display
	libraryName := state.LibraryID
	if lib, err := client.GetLibrary(state.LibraryID); err == nil {
		libraryName = lib.Name
	}

	// Check if we can compare local/remote
	var comparison *sync.SyncComparison
	if state.LocalPath != "" {
		syncer := sync.NewSyncer(client, models.SyncConfig{
			LibraryID: state.LibraryID,
			LocalPath: state.LocalPath,
			StateFile: stateFile,
		})
		comparison, err = syncer.CompareLocalAndRemote()
		if err != nil {
			// Non-fatal, just skip comparison
			fmt.Printf("Warning: could not compare files: %v\n", err)
		}
	}

	// JSON output
	if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
		status := map[string]interface{}{
			"library_id":   state.LibraryID,
			"library_name": libraryName,
			"local_path":   state.LocalPath,
			"last_sync":    state.LastSync.Format(time.RFC3339),
		}
		if comparison != nil {
			status["pending_changes"] = comparison
		}
		return json.NewEncoder(os.Stdout).Encode(status)
	}

	// Human-readable output
	fmt.Printf("Sync Status\n")
	fmt.Printf("===========\n")
	fmt.Printf("Library:   %s (%s)\n", libraryName, state.LibraryID)
	fmt.Printf("Local:     %s\n", state.LocalPath)
	fmt.Printf("Last Sync: %s\n", state.LastSync.Format("2006-01-02 15:04:05"))

	if comparison != nil {
		fmt.Printf("\nPending Changes:\n")
		fmt.Printf("  To Upload:   %d files\n", len(comparison.LocalOnly))
		for _, f := range comparison.LocalOnly {
			fmt.Printf("    + %s\n", f)
		}
		fmt.Printf("  To Download: %d files\n", len(comparison.RemoteOnly))
		for _, f := range comparison.RemoteOnly {
			fmt.Printf("    - %s\n", f)
		}
		fmt.Printf("  Modified:    %d files\n", len(comparison.Modified))
		for _, f := range comparison.Modified {
			fmt.Printf("    ~ %s\n", f)
		}
		fmt.Printf("  In Sync:     %d files\n", len(comparison.Same))
	}

	return nil
}

// Utility functions

func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

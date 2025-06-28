package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"parental-control/internal/enforcement"
	"parental-control/internal/logging"
)

// ApplicationInfo represents information about an installed application
type ApplicationInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Executable  string `json:"executable"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// ApplicationsAPIServer handles application discovery API endpoints
type ApplicationsAPIServer struct {
	processMonitor enforcement.ProcessMonitor
}

// NewApplicationsAPIServer creates a new applications API server
func NewApplicationsAPIServer(processMonitor enforcement.ProcessMonitor) *ApplicationsAPIServer {
	return &ApplicationsAPIServer{
		processMonitor: processMonitor,
	}
}

// RegisterRoutes registers application API routes
func (api *ApplicationsAPIServer) RegisterRoutes(server *Server) {
	server.AddHandlerFunc("/api/v1/applications/discover", api.handleDiscoverApplications)
	server.AddHandlerFunc("/api/v1/applications/running", api.handleGetRunningApplications)
}

// handleDiscoverApplications returns a list of installed applications suitable for blocking
func (api *ApplicationsAPIServer) handleDiscoverApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	// Discover applications from multiple sources
	applications := make(map[string]*ApplicationInfo)

	// Add running processes
	if runningApps, err := api.getRunningApplications(ctx); err == nil {
		for _, app := range runningApps {
			applications[app.Executable] = app
		}
	}

	// Add common applications from typical installation directories
	if commonApps, err := api.getCommonApplications(); err == nil {
		for _, app := range commonApps {
			// Don't overwrite running applications (they have better info)
			if _, exists := applications[app.Executable]; !exists {
				applications[app.Executable] = app
			}
		}
	}

	// Convert map to slice for response
	result := make([]*ApplicationInfo, 0, len(applications))
	for _, app := range applications {
		result = append(result, app)
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"applications": result,
		"count":        len(result),
	})
}

// handleGetRunningApplications returns currently running applications
func (api *ApplicationsAPIServer) handleGetRunningApplications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	applications, err := api.getRunningApplications(ctx)
	if err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get running applications: "+err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"applications": applications,
		"count":        len(applications),
	})
}

// getRunningApplications extracts user applications from running processes
func (api *ApplicationsAPIServer) getRunningApplications(ctx context.Context) ([]*ApplicationInfo, error) {
	if api.processMonitor == nil {
		return []*ApplicationInfo{}, nil
	}

	processes, err := api.processMonitor.GetProcesses(ctx)
	if err != nil {
		return nil, err
	}

	// Use a map to deduplicate applications by executable name
	appMap := make(map[string]*ApplicationInfo)

	for _, proc := range processes {
		if proc.Path == "" || proc.Name == "" {
			continue
		}

		// Skip system processes and common system daemons
		if api.isSystemProcess(proc) {
			continue
		}

		// Create application info
		app := &ApplicationInfo{
			Name:       api.formatApplicationName(proc.Name),
			Path:       proc.Path,
			Executable: proc.Name,
			Category:   api.categorizeApplication(proc.Name, proc.Path),
		}

		// Add description if we can determine it
		if desc := api.getApplicationDescription(proc.Name, proc.Path); desc != "" {
			app.Description = desc
		}

		// Use executable name as key to avoid duplicates
		appMap[proc.Name] = app
	}

	// Convert map to slice
	applications := make([]*ApplicationInfo, 0, len(appMap))
	for _, app := range appMap {
		applications = append(applications, app)
	}

	return applications, nil
}

// getCommonApplications returns applications from common installation directories
func (api *ApplicationsAPIServer) getCommonApplications() ([]*ApplicationInfo, error) {
	var applications []*ApplicationInfo

	// Common application directories to scan
	commonDirs := []string{
		"/usr/bin",
		"/usr/local/bin",
		"/opt",
		"/snap/bin",
		"/var/lib/flatpak/exports/bin",
		"/home/*/.local/bin",
	}

	// On Linux, also check for .desktop files
	desktopDirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/home/*/.local/share/applications",
	}

	// Scan common directories for executables
	for _, dir := range commonDirs {
		if apps, err := api.scanDirectoryForApplications(dir); err == nil {
			applications = append(applications, apps...)
		}
	}

	// Scan desktop files for better application metadata
	for _, dir := range desktopDirs {
		if apps, err := api.scanDesktopFiles(dir); err == nil {
			applications = append(applications, apps...)
		}
	}

	// Filter to only include applications likely to be user applications
	filtered := make([]*ApplicationInfo, 0)
	for _, app := range applications {
		if api.isUserApplication(app) {
			filtered = append(filtered, app)
		}
	}

	return filtered, nil
}

// scanDirectoryForApplications scans a directory for executable files
func (api *ApplicationsAPIServer) scanDirectoryForApplications(dir string) ([]*ApplicationInfo, error) {
	var applications []*ApplicationInfo

	// Handle glob patterns in directory paths
	if strings.Contains(dir, "*") {
		matches, err := filepath.Glob(dir)
		if err != nil {
			return applications, err
		}

		for _, match := range matches {
			if apps, err := api.scanDirectoryForApplications(match); err == nil {
				applications = append(applications, apps...)
			}
		}
		return applications, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return applications, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		// Check if file is executable
		if info, err := entry.Info(); err == nil {
			if info.Mode()&0111 != 0 { // Has execute permission
				app := &ApplicationInfo{
					Name:       api.formatApplicationName(entry.Name()),
					Path:       fullPath,
					Executable: entry.Name(),
					Category:   api.categorizeApplication(entry.Name(), fullPath),
				}

				if desc := api.getApplicationDescription(entry.Name(), fullPath); desc != "" {
					app.Description = desc
				}

				applications = append(applications, app)
			}
		}
	}

	return applications, nil
}

// scanDesktopFiles scans for .desktop files to get application metadata
func (api *ApplicationsAPIServer) scanDesktopFiles(dir string) ([]*ApplicationInfo, error) {
	var applications []*ApplicationInfo

	// Handle glob patterns
	if strings.Contains(dir, "*") {
		matches, err := filepath.Glob(dir)
		if err != nil {
			return applications, err
		}

		for _, match := range matches {
			if apps, err := api.scanDesktopFiles(match); err == nil {
				applications = append(applications, apps...)
			}
		}
		return applications, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return applications, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desktop") {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		if app, err := api.parseDesktopFile(fullPath); err == nil && app != nil {
			applications = append(applications, app)
		}
	}

	return applications, nil
}

// parseDesktopFile parses a .desktop file to extract application information
func (api *ApplicationsAPIServer) parseDesktopFile(filePath string) (*ApplicationInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	app := &ApplicationInfo{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name=") {
			app.Name = strings.TrimPrefix(line, "Name=")
		} else if strings.HasPrefix(line, "Exec=") {
			exec := strings.TrimPrefix(line, "Exec=")
			// Extract just the executable name from the command
			parts := strings.Fields(exec)
			if len(parts) > 0 {
				execName := filepath.Base(parts[0])
				app.Executable = execName
				app.Path = parts[0]
				
				// Map command names to actual process names
				app.Executable = api.mapCommandToProcessName(execName, parts[0])
			}
		} else if strings.HasPrefix(line, "Comment=") {
			app.Description = strings.TrimPrefix(line, "Comment=")
		} else if strings.HasPrefix(line, "Categories=") {
			categories := strings.TrimPrefix(line, "Categories=")
			app.Category = api.mapDesktopCategory(categories)
		}
	}

	// Only return if we have at least name and executable
	if app.Name != "" && app.Executable != "" {
		return app, nil
	}

	return nil, nil
}

// Helper methods for application classification and filtering

func (api *ApplicationsAPIServer) isSystemProcess(proc *enforcement.ProcessInfo) bool {
	systemProcesses := map[string]bool{
		"systemd":        true,
		"kernel":         true,
		"init":           true,
		"kthreadd":       true,
		"ksoftirqd":      true,
		"migration":      true,
		"rcu_":           true,
		"watchdog":       true,
		"systemd-":       true,
		"dbus":           true,
		"NetworkManager": true,
		"sshd":           true,
		"cron":           true,
		"rsyslog":        true,
	}

	// Check exact matches and prefixes
	name := proc.Name
	if systemProcesses[name] {
		return true
	}

	for prefix := range systemProcesses {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Check if it's a kernel thread (path in brackets)
	if strings.HasPrefix(proc.CommandLine, "[") && strings.HasSuffix(proc.CommandLine, "]") {
		return true
	}

	// Check if PID indicates system process
	return enforcement.IsSystemProcess(proc.PID)
}

func (api *ApplicationsAPIServer) isUserApplication(app *ApplicationInfo) bool {
	// Categories that are likely user applications
	userCategories := map[string]bool{
		"Game":          true,
		"Entertainment": true,
		"Internet":      true,
		"Social":        true,
		"Education":     true,
		"Graphics":      true,
		"Audio":         true,
		"Video":         true,
		"Office":        true,
		"Development":   true,
	}

	if userCategories[app.Category] {
		return true
	}

	// Check common user application patterns
	userApps := []string{
		"firefox", "chrome", "chromium", "brave", "edge",
		"steam", "discord", "spotify", "vlc", "gimp",
		"blender", "code", "atom", "sublime", "vim",
		"libreoffice", "thunderbird", "telegram", "signal",
		"zoom", "skype", "obs", "audacity", "minecraft",
	}

	execLower := strings.ToLower(app.Executable)
	for _, userApp := range userApps {
		if strings.Contains(execLower, userApp) {
			return true
		}
	}

	return false
}

func (api *ApplicationsAPIServer) formatApplicationName(executable string) string {
	// Remove file extension
	name := strings.TrimSuffix(executable, filepath.Ext(executable))

	// Capitalize first letter and make it more readable
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}

	// Replace common patterns
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	return name
}

func (api *ApplicationsAPIServer) categorizeApplication(executable, path string) string {
	execLower := strings.ToLower(executable)
	pathLower := strings.ToLower(path)

	// Games
	gamePatterns := []string{"steam", "game", "minecraft", "wow", "league"}
	for _, pattern := range gamePatterns {
		if strings.Contains(execLower, pattern) || strings.Contains(pathLower, pattern) {
			return "Game"
		}
	}

	// Browsers
	browserPatterns := []string{"firefox", "chrome", "chromium", "brave", "edge", "safari", "browser"}
	for _, pattern := range browserPatterns {
		if strings.Contains(execLower, pattern) {
			return "Internet"
		}
	}

	// Social/Communication
	socialPatterns := []string{"discord", "telegram", "signal", "whatsapp", "slack", "teams", "zoom", "skype"}
	for _, pattern := range socialPatterns {
		if strings.Contains(execLower, pattern) {
			return "Social"
		}
	}

	// Entertainment
	mediaPatterns := []string{"vlc", "spotify", "music", "video", "movie", "netflix", "youtube"}
	for _, pattern := range mediaPatterns {
		if strings.Contains(execLower, pattern) {
			return "Entertainment"
		}
	}

	// Development
	devPatterns := []string{"code", "atom", "sublime", "vim", "emacs", "idea", "studio", "git"}
	for _, pattern := range devPatterns {
		if strings.Contains(execLower, pattern) {
			return "Development"
		}
	}

	return "Other"
}

func (api *ApplicationsAPIServer) getApplicationDescription(executable, path string) string {
	// This could be enhanced to read from .desktop files or other metadata sources
	descriptions := map[string]string{
		"firefox":  "Web Browser",
		"chrome":   "Web Browser",
		"chromium": "Web Browser",
		"steam":    "Gaming Platform",
		"discord":  "Voice and Text Chat",
		"spotify":  "Music Streaming",
		"vlc":      "Media Player",
		"gimp":     "Image Editor",
		"blender":  "3D Creation Suite",
	}

	execLower := strings.ToLower(executable)
	for app, desc := range descriptions {
		if strings.Contains(execLower, app) {
			return desc
		}
	}

	return ""
}

func (api *ApplicationsAPIServer) mapDesktopCategory(categories string) string {
	categoryMap := map[string]string{
		"Game":             "Game",
		"AudioVideo":       "Entertainment",
		"Audio":            "Entertainment",
		"Video":            "Entertainment",
		"Network":          "Internet",
		"WebBrowser":       "Internet",
		"InstantMessaging": "Social",
		"Chat":             "Social",
		"Office":           "Office",
		"Graphics":         "Graphics",
		"Development":      "Development",
		"Education":        "Education",
	}

	// Check each category in the desktop file
	for desktopCat, ourCat := range categoryMap {
		if strings.Contains(categories, desktopCat) {
			return ourCat
		}
	}

	return "Other"
}

// Helper methods for JSON responses
func (api *ApplicationsAPIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (api *ApplicationsAPIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	api.writeJSONResponse(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	})
}

// mapCommandToProcessName maps command names to actual process names that appear in the process list
func (api *ApplicationsAPIServer) mapCommandToProcessName(execName, fullPath string) string {
	// Handle cases where the command name differs from the actual process name
	commandToProcessMap := map[string]string{
		"brave-beta":     "brave",
		"brave-browser":  "brave", 
		"google-chrome":  "chrome",
		"firefox-esr":    "firefox",
		"code-oss":       "code",
		"steam-runtime":  "steam",
	}
	
	// Check direct mapping first
	if processName, exists := commandToProcessMap[execName]; exists {
		return processName
	}
	
	// Check path-based mappings
	for command, process := range commandToProcessMap {
		if strings.Contains(fullPath, command) {
			return process
		}
	}
	
	// Return original if no mapping found
	return execName
}

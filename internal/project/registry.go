package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RegistrySearchResult represents a search result from the GitHub API.
type RegistrySearchResult struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	URL         string `json:"url"`
}

// TemplateManifest represents the anvil-template.yaml manifest in a registry template repo.
type TemplateManifest struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	Author          string   `yaml:"author"`
	Version         string   `yaml:"version"`
	MinAnvilVersion string   `yaml:"min_anvil_version,omitempty"`
	Files           []string `yaml:"files"`
	Tags            []string `yaml:"tags,omitempty"`
}

// RegistryMeta is the sidecar metadata stored alongside installed registry templates.
type RegistryMeta struct {
	Source      string `yaml:"source"`
	Version     string `yaml:"version"`
	InstalledAt string `yaml:"installed_at"`
	ManifestURL string `yaml:"manifest_url"`
}

// SearchCache holds cached search results.
type SearchCache struct {
	Query    string                 `json:"query"`
	Results  []RegistrySearchResult `json:"results"`
	CachedAt time.Time             `json:"cached_at"`
}

const (
	searchCacheTTL    = 1 * time.Hour
	registryUserAgent = "anvil-cli"
	githubAPIBase     = "https://api.github.com"
)

var manifestNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

// SearchRegistry queries GitHub for repositories with the anvil-template topic matching the query.
func SearchRegistry(query string) ([]RegistrySearchResult, error) {
	// Check cache first
	if cached, err := getCachedSearch(query); err == nil {
		return cached, nil
	}

	searchURL := fmt.Sprintf("%s/search/repositories?q=%s+topic:anvil-template&sort=stars&order=desc&per_page=20",
		githubAPIBase, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", registryUserAgent)

	if token := getGitHubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to reach template registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}

	var searchResp struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			StarCount   int    `json:"stargazers_count"`
			HTMLURL     string `json:"html_url"`
			Owner       struct {
				Login string `json:"login"`
			} `json:"owner"`
			Name string `json:"name"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse registry response: %w", err)
	}

	results := make([]RegistrySearchResult, 0, len(searchResp.Items))
	for _, item := range searchResp.Items {
		results = append(results, RegistrySearchResult{
			Owner:       item.Owner.Login,
			Repo:        item.Name,
			Description: item.Description,
			Stars:       item.StarCount,
			URL:         item.HTMLURL,
		})
	}

	// Cache results
	_ = cacheSearchResults(query, results)

	return results, nil
}

// FetchManifest downloads and parses anvil-template.yaml from a GitHub repository.
func FetchManifest(owner, repo string) (*TemplateManifest, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/anvil-template.yaml", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", registryUserAgent)

	if token := getGitHubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to reach template registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("template not found: %s/%s", owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest TemplateManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if err := ValidateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	return &manifest, nil
}

// ValidateManifest checks that a manifest conforms to the schema.
func ValidateManifest(m *TemplateManifest) error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !manifestNameRegex.MatchString(m.Name) {
		return fmt.Errorf("name must match %s", manifestNameRegex.String())
	}
	if m.Description == "" {
		return fmt.Errorf("description is required")
	}
	if len(m.Description) > 200 {
		return fmt.Errorf("description exceeds 200 characters")
	}
	if m.Author == "" {
		return fmt.Errorf("author is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if len(m.Files) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	if len(m.Tags) > 10 {
		return fmt.Errorf("maximum 10 tags allowed")
	}
	for _, tag := range m.Tags {
		if len(tag) > 30 {
			return fmt.Errorf("tag %q exceeds 30 characters", tag)
		}
	}
	return nil
}

// DownloadTemplate downloads template files listed in the manifest to destDir.
func DownloadTemplate(owner, repo string, manifest *TemplateManifest, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, file := range manifest.Files {
		// Prevent directory traversal in file paths
		clean := filepath.Clean(file)
		if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("invalid file path in manifest: %s", file)
		}

		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s", owner, repo, file)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request for %s: %w", file, err)
		}
		req.Header.Set("User-Agent", registryUserAgent)

		if token := getGitHubToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", file, err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("failed to download %s: HTTP %d", file, resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		destPath := filepath.Join(destDir, filepath.Base(file))
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", file, err)
		}

		fmt.Printf("  Downloaded: %s\n", file)
	}

	return nil
}

// WriteRegistryMeta writes a .registry-meta.yaml sidecar file for an installed template.
func WriteRegistryMeta(destDir, templateName, source, version string) error {
	meta := RegistryMeta{
		Source:      source,
		Version:     version,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		ManifestURL: fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/anvil-template.yaml", source),
	}

	data, err := yaml.Marshal(&meta)
	if err != nil {
		return fmt.Errorf("failed to marshal registry meta: %w", err)
	}

	metaPath := filepath.Join(destDir, templateName+".registry-meta.yaml")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry meta: %w", err)
	}

	return nil
}

// CheckCompatibility compares min_anvil_version against the current version.
// Returns (compatible, message).
func CheckCompatibility(manifest *TemplateManifest, currentVersion string) (bool, string) {
	if manifest.MinAnvilVersion == "" {
		return true, ""
	}

	// Strip leading 'v' for comparison
	current := strings.TrimPrefix(currentVersion, "v")
	required := strings.TrimPrefix(manifest.MinAnvilVersion, "v")

	if current == "dev" || current == "" {
		return true, ""
	}

	if compareVersions(current, required) < 0 {
		return false, fmt.Sprintf("template requires anvil v%s or later (you have v%s)", required, current)
	}

	return true, ""
}

// ListInstalledRegistryTemplates scans for .registry-meta.yaml sidecar files.
func ListInstalledRegistryTemplates(projectPath string) ([]RegistryMeta, error) {
	var metas []RegistryMeta

	for _, basePath := range templatePaths(projectPath) {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read templates directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".registry-meta.yaml") {
				data, err := os.ReadFile(filepath.Join(basePath, entry.Name()))
				if err != nil {
					continue
				}
				var meta RegistryMeta
				if err := yaml.Unmarshal(data, &meta); err != nil {
					continue
				}
				metas = append(metas, meta)
			}
		}
	}

	return metas, nil
}

// getCachedSearch returns cached search results if they exist and are fresh.
func getCachedSearch(query string) ([]RegistrySearchResult, error) {
	cachePath := searchCachePath(query)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var cache SearchCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	if time.Since(cache.CachedAt) > searchCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	return cache.Results, nil
}

// cacheSearchResults writes search results to the cache directory.
func cacheSearchResults(query string, results []RegistrySearchResult) error {
	cachePath := searchCachePath(query)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}

	cache := SearchCache{
		Query:    query,
		Results:  results,
		CachedAt: time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func searchCachePath(query string) string {
	homeDir, _ := os.UserHomeDir()
	hash := sha256.Sum256([]byte(query))
	return filepath.Join(homeDir, ".anvil", "cache", "registry", "search-"+hex.EncodeToString(hash[:8])+".json")
}

// getGitHubToken attempts to get a GitHub auth token from the gh CLI.
func getGitHubToken() string {
	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// compareVersions does a simple semver comparison.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) || i < len(bParts); i++ {
		var aNum, bNum int
		if i < len(aParts) {
			fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}
	return 0
}

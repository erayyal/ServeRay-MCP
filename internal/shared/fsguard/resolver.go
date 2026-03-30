package fsguard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Root struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modified_at"`
}

type Resolver struct {
	allowHidden bool
	roots       map[string]string
}

func New(rawRoots []string, allowHidden bool) (*Resolver, error) {
	roots := make(map[string]string, len(rawRoots))
	seenPaths := make(map[string]struct{}, len(rawRoots))

	for _, raw := range rawRoots {
		name, path, err := parseRoot(raw)
		if err != nil {
			return nil, err
		}
		if !allowHidden && containsHiddenOrSensitiveSegment(path) {
			return nil, fmt.Errorf("root %q is hidden or sensitive; enable FILESYSTEM_ALLOW_HIDDEN to allow it", path)
		}
		if _, exists := roots[name]; exists {
			return nil, fmt.Errorf("duplicate root alias %q", name)
		}
		if _, exists := seenPaths[path]; exists {
			return nil, fmt.Errorf("duplicate root path %q", path)
		}
		roots[name] = path
		seenPaths[path] = struct{}{}
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("at least one filesystem root is required")
	}

	return &Resolver{
		allowHidden: allowHidden,
		roots:       roots,
	}, nil
}

func (r *Resolver) ListRoots() []Root {
	out := make([]Root, 0, len(r.roots))
	for name, path := range r.roots {
		out = append(out, Root{Name: name, Path: path})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (r *Resolver) Resolve(rootName, relativePath string) (string, error) {
	rootPath, ok := r.roots[rootName]
	if !ok {
		return "", fmt.Errorf("unknown root %q", rootName)
	}

	if relativePath == "" {
		relativePath = "."
	}
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	cleaned := filepath.Clean(relativePath)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	if !r.allowHidden && containsHiddenOrSensitiveSegment(cleaned) {
		return "", fmt.Errorf("hidden or sensitive paths are blocked")
	}

	candidate := filepath.Join(rootPath, cleaned)
	resolved, err := resolveWithSymlinks(candidate)
	if err != nil {
		return "", err
	}

	if resolved != rootPath && !strings.HasPrefix(resolved, rootPath+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes the configured root")
	}
	return resolved, nil
}

func (r *Resolver) ListDir(rootName, relativePath string, maxEntries int) ([]Entry, error) {
	resolved, err := r.Resolve(rootName, relativePath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if !r.allowHidden && containsHiddenOrSensitiveSegment(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat entry: %w", err)
		}
		out = append(out, Entry{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(filepath.Join(relativePath, entry.Name())),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
		if maxEntries > 0 && len(out) >= maxEntries {
			break
		}
	}
	return out, nil
}

func (r *Resolver) ReadTextFile(rootName, relativePath string, startLine, maxLines int, maxBytes int64) (string, error) {
	resolved, err := r.Resolve(rootName, relativePath)
	if err != nil {
		return "", err
	}

	file, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	reader := bufio.NewScanner(io.LimitReader(file, maxBytes))
	currentLine := 0
	lines := make([]string, 0, maxLines)

	for reader.Scan() {
		currentLine++
		if currentLine < startLine {
			continue
		}
		lines = append(lines, reader.Text())
		if maxLines > 0 && len(lines) >= maxLines {
			break
		}
	}
	if err := reader.Err(); err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return strings.Join(lines, "\n"), nil
}

func parseRoot(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty filesystem root entry")
	}

	name := ""
	path := raw
	if left, right, found := strings.Cut(raw, "="); found {
		name = strings.TrimSpace(left)
		path = strings.TrimSpace(right)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve root %q: %w", raw, err)
	}
	absolutePath, err = filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", "", fmt.Errorf("evaluate root %q: %w", raw, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", "", fmt.Errorf("stat root %q: %w", raw, err)
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("root %q must be a directory", raw)
	}

	if name == "" {
		name = filepath.Base(absolutePath)
	}
	name = slug(name)
	if name == "" {
		return "", "", fmt.Errorf("root %q produced an empty alias", raw)
	}

	return name, absolutePath, nil
}

func resolveWithSymlinks(candidate string) (string, error) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	parent, fileName := filepath.Split(candidate)
	parent = filepath.Clean(parent)
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", fmt.Errorf("resolve parent: %w", err)
	}
	return filepath.Join(resolvedParent, fileName), nil
}

func containsHiddenOrSensitiveSegment(path string) bool {
	sensitive := map[string]struct{}{
		".aws":                      {},
		".git":                      {},
		".kube":                     {},
		".ssh":                      {},
		"$recycle.bin":              {},
		"system volume information": {},
	}

	cleaned := filepath.ToSlash(filepath.Clean(path))
	for _, segment := range strings.Split(cleaned, "/") {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "." {
			continue
		}
		lower := strings.ToLower(segment)
		if strings.HasPrefix(segment, ".") {
			return true
		}
		if _, blocked := sensitive[lower]; blocked {
			return true
		}
	}
	return false
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "_", "-", "/", "-", "\\", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	return value
}

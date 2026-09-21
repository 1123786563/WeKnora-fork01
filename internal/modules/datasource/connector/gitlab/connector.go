package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
)

var (
	_ datasource.StreamingConnector = (*Connector)(nil)
	_ datasource.TargetedFetcher    = (*Connector)(nil)
)

type Connector struct {
	client        *client
	canonicalBase string
}

// NewConnector creates a stateless connector. Each data source provides its
// own GitLab URL and access token in its encrypted credentials.
func NewConnector() *Connector    { return &Connector{} }
func (c *Connector) Type() string { return types.ConnectorTypeGitLab }

func (c *Connector) configured(ds *types.DataSourceConfig) (*Connector, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	baseURL, _ := ds.Credentials["base_url"].(string)
	token, _ := ds.Credentials["access_token"].(string)
	client, err := newClient(baseURL, token)
	if err != nil {
		return nil, err
	}
	return &Connector{client: client, canonicalBase: client.baseURL}, nil
}

func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	configured, err := c.configured(ds)
	if err != nil {
		return err
	}
	if err := configured.client.ping(ctx); err != nil {
		return err
	}
	if ds != nil {
		if _, ok := ds.Settings["projects"]; ok {
			if _, err := parseConfig(ds); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *Connector) ListResources(ctx context.Context, ds *types.DataSourceConfig, parent string) ([]types.Resource, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	if _, err := parseConfig(ds); err != nil {
		return nil, err
	}
	if parent == "" {
		ps, err := c.client.projects(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]types.Resource, 0, len(ps))
		for _, p := range ps {
			out = append(out, types.Resource{ExternalID: fmt.Sprint(p.ID), Name: p.PathWithNamespace, Type: "project", URL: p.WebURL, HasChildren: true})
		}
		return out, nil
	}
	id, dir := splitResourceID(parent)
	p, err := c.client.project(ctx, id)
	if err != nil {
		return nil, err
	}
	ref := p.DefaultBranch
	entries, err := c.client.tree(ctx, id, ref, dir)
	if err != nil {
		return nil, err
	}
	out := make([]types.Resource, 0, len(entries))
	for _, e := range entries {
		if e.Type == "tree" {
			out = append(out, types.Resource{ExternalID: id + ":" + e.Path, Name: e.Name, Type: "directory", ParentID: parent, HasChildren: true})
		}
	}
	return out, nil
}
func (c *Connector) ResolveResourceAncestors(context.Context, *types.DataSourceConfig, []string) ([]string, error) {
	return []string{}, nil
}
func (c *Connector) FetchAll(ctx context.Context, ds *types.DataSourceConfig, _ []string) ([]types.FetchedItem, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	cfg, err := parseConfig(ds)
	if err != nil {
		return nil, err
	}
	var out []types.FetchedItem
	for _, s := range cfg.Projects {
		p, err := c.client.project(ctx, s.ProjectID)
		if err != nil {
			return nil, err
		}
		ref := s.Ref
		if ref == "" {
			ref = p.DefaultBranch
		}
		files, err := c.files(ctx, s.ProjectID, ref, s.Paths)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			item, err := c.item(ctx, p, ref, file)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
	}
	return out, nil
}

type cursor struct {
	Projects map[string]string `json:"projects"`
}

func (c *Connector) FetchIncremental(ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, nil, err
	}
	cfg, err := parseConfig(ds)
	if err != nil {
		return nil, nil, err
	}
	prev := cursor{Projects: map[string]string{}}
	if old != nil {
		b, _ := json.Marshal(old.ConnectorCursor)
		_ = json.Unmarshal(b, &prev)
		if prev.Projects == nil {
			prev.Projects = map[string]string{}
		}
	}
	var out []types.FetchedItem
	next := cursor{Projects: map[string]string{}}
	for _, s := range cfg.Projects {
		p, err := c.client.project(ctx, s.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		ref := s.Ref
		if ref == "" {
			ref = p.DefaultBranch
		}
		head, err := c.client.commitSHA(ctx, s.ProjectID, ref)
		if err != nil {
			return nil, nil, err
		}
		previous := prev.Projects[s.ProjectID]
		if previous == "" {
			files, err := c.files(ctx, s.ProjectID, ref, s.Paths)
			if err != nil {
				return nil, nil, err
			}
			for _, f := range files {
				item, err := c.item(ctx, p, ref, f)
				if err != nil {
					return nil, nil, err
				}
				out = append(out, item)
			}
		} else if previous != head {
			diff, err := c.client.compare(ctx, s.ProjectID, previous, head)
			if err != nil || diff.CompareTimeout {
				// A compare may be unavailable after history rewrites or truncated by
				// the server. Re-enumerate the configured scope to preserve updates.
				files, listErr := c.files(ctx, s.ProjectID, ref, s.Paths)
				if listErr != nil {
					return nil, nil, fmt.Errorf("gitlab list files %s: %w", s.ProjectID, listErr)
				}
				for _, f := range files {
					item, itemErr := c.item(ctx, p, ref, f)
					if itemErr != nil {
						return nil, nil, itemErr
					}
					out = append(out, item)
				}
			} else {
				for _, d := range diff.Diffs {
					if d.DeletedFile {
						if c.inScope(d.OldPath, s.Paths) && isSupportedFile(d.OldPath) {
							out = append(out, c.deleted(p, ref, d.OldPath))
						}
						continue
					}
					if d.RenamedFile && c.inScope(d.OldPath, s.Paths) && isSupportedFile(d.OldPath) {
						out = append(out, c.deleted(p, ref, d.OldPath))
					}
					if c.inScope(d.NewPath, s.Paths) && isSupportedFile(d.NewPath) {
						item, err := c.item(ctx, p, ref, d.NewPath)
						if err != nil {
							return nil, nil, err
						}
						out = append(out, item)
					}
				}
			}
		}
		next.Projects[s.ProjectID] = head
	}
	raw, _ := json.Marshal(next)
	return out, &types.SyncCursor{LastSyncTime: time.Now().UTC(), ConnectorCursor: map[string]interface{}{"projects": next.Projects, "raw": string(raw)}}, nil
}

// FetchStream is the production sync path. It emits each supported repository
// file as soon as it is read, so large GitLab projects do not accumulate their
// contents in memory. The service prefers this method for StreamingConnector
// implementations; FetchAll and FetchIncremental remain compatibility methods
// required by the base Connector interface.
func (c *Connector) FetchStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	var err error
	if c, err = c.configured(ds); err != nil {
		return nil, err
	}
	cfg, err := parseConfig(ds)
	if err != nil {
		return nil, err
	}
	prev := cursor{Projects: map[string]string{}}
	if old != nil {
		b, _ := json.Marshal(old.ConnectorCursor)
		_ = json.Unmarshal(b, &prev)
		if prev.Projects == nil {
			prev.Projects = map[string]string{}
		}
	}
	next := cursor{Projects: make(map[string]string, len(cfg.Projects))}
	for projectID, commit := range prev.Projects {
		next.Projects[projectID] = commit
	}

	for _, selection := range cfg.Projects {
		project, err := c.client.project(ctx, selection.ProjectID)
		if err != nil {
			return nil, err
		}
		ref := selection.Ref
		if ref == "" {
			ref = project.DefaultBranch
		}
		head, err := c.client.commitSHA(ctx, selection.ProjectID, ref)
		if err != nil {
			return nil, err
		}

		previous := prev.Projects[selection.ProjectID]
		switch {
		case previous == "":
			err = c.streamFiles(ctx, project, ref, selection.Paths, h)
		case previous != head:
			err = c.streamChanges(ctx, project, ref, previous, head, selection.Paths, h)
		}
		if err != nil {
			return nil, err
		}

		next.Projects[selection.ProjectID] = head
		checkpoint := gitLabCursor(next)
		if err := h.Checkpoint(ctx, checkpoint); err != nil {
			return nil, err
		}
	}
	return gitLabCursor(next), nil
}

func gitLabCursor(value cursor) *types.SyncCursor {
	raw, _ := json.Marshal(value)
	return &types.SyncCursor{
		LastSyncTime:    time.Now().UTC(),
		ConnectorCursor: map[string]interface{}{"projects": value.Projects, "raw": string(raw)},
	}
}

// FetchByExternalID refetches a single repository file for a targeted reindex
// round. The external id minted by item embeds everything the rebuild needs
// ("gitlab:<base>:<projectID>:<ref>:<file>"), so the id is matched against the
// configured instance's canonical base and then split into project / ref /
// file; the file is rebuilt through the same client.project + item path a
// normal sync uses, keeping the item shape identical. The git file fetch path
// never sets ReplacesSubtree, so the targeted item keeps it unset too.
func (c *Connector) FetchByExternalID(
	ctx context.Context, ds *types.DataSourceConfig, externalID string,
) (*types.FetchedItem, error) {
	if externalID == "" {
		return nil, fmt.Errorf("%w: empty external id", datasource.ErrItemNotFound)
	}
	configured, err := c.configured(ds)
	if err != nil {
		return nil, err
	}
	// Anchor the parse on the configured canonical base: base URLs contain
	// colons (scheme, optional port, /api/v4), so a blind split is ambiguous.
	prefix := "gitlab:" + configured.canonicalBase + ":"
	if !strings.HasPrefix(externalID, prefix) {
		return nil, fmt.Errorf(
			"%w: external id %q does not belong to this GitLab instance (%s)",
			datasource.ErrItemNotFound, externalID, configured.canonicalBase)
	}
	// Git refnames cannot contain ':', so exactly the first two colons of the
	// remainder separate project and ref; everything after the second belongs
	// to the file path (which may legitimately contain colons).
	parts := strings.SplitN(strings.TrimPrefix(externalID, prefix), ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("%w: malformed gitlab external id %q", datasource.ErrItemNotFound, externalID)
	}
	projectID, ref, file := parts[0], parts[1], parts[2]

	p, err := configured.client.project(ctx, projectID)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.status == http.StatusNotFound {
			return nil, fmt.Errorf("%w: gitlab project %q: %v", datasource.ErrItemNotFound, projectID, err)
		}
		return nil, fmt.Errorf("gitlab get project %s: %w", projectID, err)
	}
	item, err := configured.item(ctx, p, ref, file)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.status == http.StatusNotFound {
			return nil, fmt.Errorf("%w: gitlab file %q@%s: %v", datasource.ErrItemNotFound, file, ref, err)
		}
		return nil, err
	}
	if item.ExternalID != externalID {
		// The id did not round-trip (e.g. a namespace-path project id resolved
		// to its numeric id). Ingesting the rebuilt item would file it under a
		// different identity instead of replacing the failed one, so refuse.
		return nil, fmt.Errorf(
			"%w: gitlab refetch of %q rebuilt id %q",
			datasource.ErrItemNotFound, externalID, item.ExternalID)
	}
	return &item, nil
}

func (c *Connector) streamChanges(
	ctx context.Context, project *project, ref, from, to string, roots []string, h datasource.StreamHandler,
) error {
	diff, err := c.client.compare(ctx, fmt.Sprint(project.ID), from, to)
	if err != nil || diff.CompareTimeout {
		// A compare can be unavailable after history rewrites or be truncated by
		// GitLab. Re-enumerating the configured scope preserves file updates.
		return c.streamFiles(ctx, project, ref, roots, h)
	}
	for _, change := range diff.Diffs {
		if change.DeletedFile {
			if c.inScope(change.OldPath, roots) && isSupportedFile(change.OldPath) {
				if err := h.Emit(ctx, c.deleted(project, ref, change.OldPath)); err != nil {
					return err
				}
			}
			continue
		}
		if change.RenamedFile && c.inScope(change.OldPath, roots) && isSupportedFile(change.OldPath) {
			if err := h.Emit(ctx, c.deleted(project, ref, change.OldPath)); err != nil {
				return err
			}
		}
		if c.inScope(change.NewPath, roots) && isSupportedFile(change.NewPath) {
			item, err := c.item(ctx, project, ref, change.NewPath)
			if err != nil {
				return err
			}
			if err := h.Emit(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Connector) streamFiles(
	ctx context.Context, project *project, ref string, roots []string, h datasource.StreamHandler,
) error {
	return c.walkFiles(ctx, fmt.Sprint(project.ID), ref, roots, func(file string) error {
		item, err := c.item(ctx, project, ref, file)
		if err != nil {
			return err
		}
		return h.Emit(ctx, item)
	})
}

func (c *Connector) files(ctx context.Context, id, ref string, roots []string) ([]string, error) {
	var out []string
	err := c.walkFiles(ctx, id, ref, roots, func(file string) error {
		out = append(out, file)
		return nil
	})
	return out, err
}

// walkFiles visits supported blobs while traversing the selected directories.
// It deliberately does not collect paths, keeping streaming sync memory bounded
// by the traversal depth plus the current file body.
func (c *Connector) walkFiles(ctx context.Context, id, ref string, roots []string, visit func(string) error) error {
	if len(roots) == 0 {
		roots = []string{""}
	}
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := c.client.tree(ctx, id, ref, dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Type == "tree" {
				if err := walk(e.Path); err != nil {
					return err
				}
			} else if e.Type == "blob" && isSupportedFile(e.Path) {
				if err := visit(e.Path); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, r := range roots {
		if err := walk(r); err != nil {
			return err
		}
	}
	return nil
}

func isSupportedFile(file string) bool {
	_, ok := gitLabSupportedFileExtensions[strings.ToLower(path.Ext(file))]
	return ok
}

// gitLabSupportedFileExtensions limits repository sync to formats the current
// knowledge import pipeline can process. This is intentionally connector-local:
// GitLab exposes arbitrary repository blobs, unlike document-centric sources.
var gitLabSupportedFileExtensions = map[string]struct{}{
	".pdf": {}, ".txt": {}, ".docx": {}, ".doc": {}, ".epub": {},
	".html": {}, ".htm": {}, ".mhtml": {}, ".md": {}, ".markdown": {}, ".mdx": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {},
	".csv": {}, ".xlsx": {}, ".xls": {}, ".pptx": {}, ".ppt": {}, ".json": {},
	".mp3": {}, ".wav": {}, ".m4a": {}, ".flac": {}, ".ogg": {},
}

func (c *Connector) item(ctx context.Context, p *project, ref, file string) (types.FetchedItem, error) {
	body, err := c.client.raw(ctx, fmt.Sprint(p.ID), ref, file)
	if err != nil {
		return types.FetchedItem{}, err
	}
	id := fmt.Sprintf("gitlab:%s:%d:%s:%s", c.canonicalBase, p.ID, ref, file)
	// UpdatedAt is intentionally left unset: the file's last commit time is not
	// fetched here, and a fetch timestamp would be a fabricated source time.
	return types.FetchedItem{
		ExternalID:       id,
		Title:            p.PathWithNamespace + "/" + file,
		FileName:         knowledgeRelativePath(p.Name, ref, file),
		Content:          body,
		ContentType:      "text/plain",
		SourceResourceID: fmt.Sprint(p.ID),
		Metadata: map[string]string{
			"channel":           types.ConnectorTypeGitLab,
			"source_type":       "gitlab",
			"gitlab_project_id": fmt.Sprint(p.ID),
			"gitlab_ref":        ref,
			"gitlab_path":       file,
			"gitlab_url":        p.WebURL + "/-/blob/" + ref + "/" + file,
		},
	}, nil
}

// knowledgeRelativePath maps a repository file to the KB folder convention:
// <GitLab project name>-<branch>/<repository-relative file path>.
func knowledgeRelativePath(projectName, ref, file string) string {
	root := strings.TrimSpace(projectName) + "-" + strings.ReplaceAll(strings.TrimSpace(ref), "/", "-")
	return path.Join(root, file)
}
func (c *Connector) deleted(p *project, ref, file string) types.FetchedItem {
	return types.FetchedItem{ExternalID: fmt.Sprintf("gitlab:%s:%d:%s:%s", c.canonicalBase, p.ID, ref, file), IsDeleted: true, Metadata: map[string]string{"channel": types.ConnectorTypeGitLab, "gitlab_path": file}}
}
func (c *Connector) inScope(file string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, r := range roots {
		if file == r || strings.HasPrefix(file, r+"/") {
			return true
		}
	}
	return false
}
func splitResourceID(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 1 {
		return value, ""
	}
	return parts[0], parts[1]
}

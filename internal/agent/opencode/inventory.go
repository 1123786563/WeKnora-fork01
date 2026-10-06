package opencode

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

const (
	sessionInventoryPageSize = 100
	maxSessionInventoryPages = 1000
	maxSessionInventoryRows  = 100000
	maxSessionCursorBytes    = 2048
)

// SessionInfo is the pinned OpenCode 1.18.4 Session.Info projection needed to
// identify a session's project and server-selected filesystem location.
type SessionInfo struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"projectID"`
	Title     string          `json:"title"`
	Location  SessionLocation `json:"location"`
	Time      SessionTime     `json:"time"`
}

// SessionLocation is the location metadata returned by OpenCode Session.Info.
type SessionLocation struct {
	Directory   string `json:"directory"`
	WorkspaceID string `json:"workspaceID"`
}

// SessionTime is the timestamp metadata returned by OpenCode Session.Info.
type SessionTime struct {
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}

// Inventory performs complete, request-scoped OpenCode session inventory
// reads. It is tied to one server-selected directory and expected project.
type Inventory struct {
	client    *Client
	directory string
	projectID string
}

// NewInventory creates a scoped inventory reader. The directory is server
// selected and immutable for this inventory instance; it is sent through the
// pinned OpenCode location header and never accepted from a session record.
func NewInventory(client *Client, directory, projectID string) (*Inventory, error) {
	if client == nil || client.base == nil || client.http == nil {
		return nil, errors.New("OpenCode inventory client is not initialized")
	}
	if err := validateDirectory(directory); err != nil {
		return nil, err
	}
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(projectID) != projectID {
		return nil, errors.New("OpenCode inventory project ID is invalid")
	}
	bound := client
	if client.directory == "" {
		var err error
		bound, err = client.WithDirectory(directory)
		if err != nil {
			return nil, err
		}
	} else if client.directory != directory {
		return nil, errors.New("OpenCode inventory directory conflicts with client binding")
	}
	bound = withInventoryRedirectPolicy(bound)
	return &Inventory{client: bound, directory: directory, projectID: projectID}, nil
}

// ListSessions returns a complete inventory for the exact project and
// directory. Any malformed, duplicated, foreign, looping, or truncated page
// makes the entire result fail closed.
func (i *Inventory) ListSessions(ctx context.Context) ([]SessionInfo, error) {
	if i == nil || i.client == nil {
		return nil, errors.New("OpenCode inventory is not initialized")
	}
	var out []SessionInfo
	seenIDs := make(map[string]struct{})
	seenCursors := make(map[string]struct{})
	cursor := ""
	for page := 0; page < maxSessionInventoryPages; page++ {
		query := url.Values{}
		query.Set("limit", fmt.Sprint(sessionInventoryPageSize))
		query.Set("project", i.projectID)
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var response struct {
			Data   []SessionInfo `json:"data"`
			Cursor *struct {
				Next string `json:"next"`
			} `json:"cursor"`
		}
		if err := i.client.jsonRequest(ctx, http.MethodGet, "/api/session?"+query.Encode(), nil, http.StatusOK, &response); err != nil {
			return nil, fmt.Errorf("list OpenCode sessions: %w", err)
		}
		if response.Cursor == nil {
			return nil, errors.New("OpenCode session inventory has missing or null cursor envelope")
		}
		if response.Data == nil {
			return nil, errors.New("OpenCode session inventory has missing or null data")
		}
		if len(response.Data) > sessionInventoryPageSize {
			return nil, errors.New("OpenCode session inventory page exceeds requested limit")
		}
		if len(out)+len(response.Data) > maxSessionInventoryRows {
			return nil, errors.New("OpenCode session inventory exceeds row limit")
		}
		for _, info := range response.Data {
			if err := i.validateSessionInfo(info, ""); err != nil {
				return nil, err
			}
			if _, exists := seenIDs[info.ID]; exists {
				return nil, fmt.Errorf("OpenCode session inventory contains duplicate ID %q", info.ID)
			}
			seenIDs[info.ID] = struct{}{}
			out = append(out, info)
		}

		next := ""
		if response.Cursor != nil {
			next = response.Cursor.Next
		}
		if err := validateInventoryCursor(next); err != nil {
			return nil, err
		}
		if next == "" {
			if len(response.Data) == sessionInventoryPageSize {
				return nil, errors.New("OpenCode session inventory is truncated: full page has no continuation cursor")
			}
			return out, nil
		}
		if len(response.Data) == 0 {
			return nil, errors.New("OpenCode session inventory has an empty page with a continuation cursor")
		}
		if _, exists := seenCursors[next]; exists {
			return nil, errors.New("OpenCode session inventory cursor loop detected")
		}
		seenCursors[next] = struct{}{}
		cursor = next
	}
	return nil, errors.New("OpenCode session inventory exceeded page limit")
}

// withInventoryRedirectPolicy keeps the shared Client's same-origin and
// directory checks, then verifies that a redirect preserves this request's
// project and pagination query values exactly.
func withInventoryRedirectPolicy(client *Client) *Client {
	copyClient := *client
	copyHTTP := *client.http
	previous := copyHTTP.CheckRedirect
	base := client.base
	copyHTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
		}
		if !sameOrigin(req.URL, base) {
			return errors.New("OpenCode inventory redirect crossed origin boundary")
		}
		if len(via) == 0 || via[len(via)-1] == nil || via[len(via)-1].URL == nil {
			return errors.New("OpenCode inventory redirect has no source request")
		}
		if err := sameInventoryQueryScope(via[len(via)-1].URL.RawQuery, req.URL.RawQuery); err != nil {
			return err
		}
		return nil
	}
	copyClient.http = &copyHTTP
	return &copyClient
}

func sameInventoryQueryScope(previousRaw, nextRaw string) error {
	previous, err := url.ParseQuery(previousRaw)
	if err != nil {
		return errors.New("OpenCode inventory redirect source query is malformed")
	}
	next, err := url.ParseQuery(nextRaw)
	if err != nil {
		return errors.New("OpenCode inventory redirect query scope is malformed")
	}
	for _, key := range []string{"project", "limit", "cursor"} {
		previousValues, previousOK := previous[key]
		nextValues, nextOK := next[key]
		if previousOK != nextOK || len(previousValues) != len(nextValues) {
			return fmt.Errorf("OpenCode inventory redirect changed query scope field %q", key)
		}
		for index := range previousValues {
			if previousValues[index] != nextValues[index] {
				return fmt.Errorf("OpenCode inventory redirect changed query scope field %q", key)
			}
		}
	}
	return nil
}

// GetSession returns one typed session record only when its ID, project, and
// location exactly match this inventory's immutable scope.
func (i *Inventory) GetSession(ctx context.Context, sessionID string) (SessionInfo, error) {
	if i == nil || i.client == nil {
		return SessionInfo{}, errors.New("OpenCode inventory is not initialized")
	}
	if !validSessionID(sessionID) {
		return SessionInfo{}, errors.New("OpenCode session ID is malformed")
	}
	var response struct {
		Data SessionInfo `json:"data"`
	}
	path := escapedPath("api", "session", sessionID)
	if err := i.client.jsonRequest(ctx, http.MethodGet, path, nil, http.StatusOK, &response); err != nil {
		return SessionInfo{}, fmt.Errorf("get OpenCode session metadata: %w", err)
	}
	if err := i.validateSessionInfo(response.Data, sessionID); err != nil {
		return SessionInfo{}, err
	}
	return response.Data, nil
}

func (i *Inventory) validateSessionInfo(info SessionInfo, expectedID string) error {
	if !validSessionID(info.ID) {
		return fmt.Errorf("OpenCode session metadata has malformed ID %q", info.ID)
	}
	if expectedID != "" && info.ID != expectedID {
		return errors.New("OpenCode session metadata ID does not match requested ID")
	}
	if info.ProjectID == "" || info.ProjectID != i.projectID {
		return errors.New("OpenCode session metadata project does not match inventory scope")
	}
	if info.Location.Directory == "" || info.Location.Directory != i.directory {
		return errors.New("OpenCode session metadata directory does not match inventory scope")
	}
	return nil
}

func validateInventoryCursor(cursor string) error {
	if len(cursor) > maxSessionCursorBytes {
		return errors.New("OpenCode session inventory cursor exceeds size limit")
	}
	for _, r := range cursor {
		if unicode.IsControl(r) {
			return errors.New("OpenCode session inventory cursor contains control character")
		}
	}
	return nil
}

// validSessionID enforces the pinned 1.18.4 ses_ timestamp/counter and base62
// random suffix shape recorded by the protocol fixtures.
func validSessionID(id string) bool {
	if len(id) != len("ses_")+12+14 || !strings.HasPrefix(id, "ses_") {
		return false
	}
	for index, r := range id[len("ses_"):] {
		switch {
		case index < 12 && (r >= '0' && r <= '9' || r >= 'a' && r <= 'f'):
		case index >= 12 && (r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'):
		default:
			return false
		}
	}
	return true
}

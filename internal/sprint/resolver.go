package sprint

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"botty/internal/clickup"
)

const (
	parentFolder = 5
	parentList   = 6
)

var viewIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$`)

type Resolver interface {
	Resolve(ctx context.Context, anchor string, limit int) ([]Sprint, error)
}

type HierarchyAPI interface {
	GetView(ctx context.Context, viewID string) (clickup.View, error)
	GetList(ctx context.Context, listID string) (clickup.List, error)
	GetFolderLists(ctx context.Context, folderID string) ([]clickup.List, error)
}

func ParseViewID(anchor string) (string, error) {
	anchor = strings.TrimSpace(anchor)
	if viewIDPattern.MatchString(anchor) {
		return anchor, nil
	}

	parsed, err := url.Parse(anchor)
	if err != nil {
		return "", fmt.Errorf("anchor-view inválido: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "app.clickup.com") {
		return "", fmt.Errorf("anchor-view debe ser URL HTTPS de app.clickup.com o un view ID")
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	for index := 0; index+2 < len(segments); index++ {
		if segments[index] != "v" || segments[index+1] != "l" {
			continue
		}
		id, unescapeErr := url.PathUnescape(segments[index+2])
		if unescapeErr == nil && viewIDPattern.MatchString(id) {
			return id, nil
		}
	}
	return "", fmt.Errorf("no se encontró view ID en anchor-view")
}

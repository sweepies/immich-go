package upload

import (
	"context"
	"strings"
	"testing"

	"github.com/sweepies/immich-go/app"
	"github.com/sweepies/immich-go/immich"
	"github.com/sweepies/immich-go/internal/assets"
)

type emptyTagUpsertClient struct {
	immich.Client
}

func (c *emptyTagUpsertClient) UpsertTags(context.Context, []string) ([]immich.TagSimplified, error) {
	return []immich.TagSimplified{}, nil
}

func TestSaveTagRejectsEmptyUpsertResponse(t *testing.T) {
	client := &emptyTagUpsertClient{}
	command := &UpCmd{client: app.Client{Immich: client}}
	tag := assets.Tag{Name: "vacation", Value: "vacation"}

	got, err := command.saveTag(t.Context(), tag, []string{"asset-id"})
	if err == nil {
		t.Fatal("saveTag() error = nil, want empty-response error")
	}
	if !strings.Contains(err.Error(), "returned no tags") {
		t.Errorf("saveTag() error = %q, want empty-response context", err)
	}
	if got.ID != "" {
		t.Errorf("saveTag() tag ID = %q, want empty", got.ID)
	}
}

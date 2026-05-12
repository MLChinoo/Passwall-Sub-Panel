package handler

import (
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

func TestGroupDTOUsesEmptyTagsArray(t *testing.T) {
	dto := toGroupDTO(&domain.Group{
		ID:        1,
		Slug:      "default",
		Name:      "Default",
		TagFilter: domain.TagFilter{All: true},
	})

	if dto.TagFilter.Tags == nil {
		t.Fatal("TagFilter.Tags is nil, want empty slice")
	}
	if len(dto.TagFilter.Tags) != 0 {
		t.Fatalf("len(TagFilter.Tags) = %d, want 0", len(dto.TagFilter.Tags))
	}
}

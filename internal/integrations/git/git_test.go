package git

import (
	"testing"
)

func TestGetRepoName(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "HTTPS with .git suffix",
			url:      "https://gitlab.com/MrPuls/test-proj.git",
			expected: "test-proj",
		},
		{
			name:     "HTTPS without .git suffix",
			url:      "https://gitlab.com/MrPuls/test-proj",
			expected: "test-proj",
		},
		{
			name:     "SSH URL",
			url:      "git@gitlab.com:MrPuls/test-proj.git",
			expected: "test-proj",
		},
		{
			name:     "SSH URL without .git suffix",
			url:      "git@gitlab.com:MrPuls/my-repo",
			expected: "my-repo",
		},
		{
			name:     "GitHub HTTPS",
			url:      "https://github.com/user/repo.git",
			expected: "repo",
		},
		{
			name:     "Nested group URL",
			url:      "https://gitlab.com/org/group/subgroup/project.git",
			expected: "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetRepoName(tt.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("GetRepoName(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

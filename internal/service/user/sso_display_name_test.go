package user

import "testing"

// TestSSO_DisplayNamePriority pins down the username-derivation rule used by
// EnsureSSO. The priority is documented elsewhere: email → display name → UPN.
// This test makes the contract executable so it's harder to accidentally
// regress (e.g. by reordering the conditions, or by inadvertently dropping
// to UPN when a usable email is present).
func TestSSO_DisplayNamePriority(t *testing.T) {
	tests := []struct {
		name string
		in   EnsureSSOInput
		want string
	}{
		{
			name: "email wins over display name and UPN",
			in: EnsureSSOInput{
				UPN:         "8d612153-4929-4ce5-b5ad-e9713e1c0be7",
				Email:       "me@kazuha.org",
				DisplayName: "Kazuha Hub",
			},
			want: "me@kazuha.org",
		},
		{
			name: "display name used when email missing",
			in: EnsureSSOInput{
				UPN:         "8d612153-4929-4ce5-b5ad-e9713e1c0be7",
				DisplayName: "Kazuha Hub",
			},
			want: "Kazuha Hub",
		},
		{
			name: "UPN is the last resort",
			in: EnsureSSOInput{
				UPN: "8d612153-4929-4ce5-b5ad-e9713e1c0be7",
			},
			want: "8d612153-4929-4ce5-b5ad-e9713e1c0be7",
		},
		{
			name: "empty email does not get used",
			in: EnsureSSOInput{
				UPN:         "8d612153-4929-4ce5-b5ad-e9713e1c0be7",
				Email:       "",
				DisplayName: "me@kazuha.org",
			},
			want: "me@kazuha.org",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ssoDisplayName(tt.in)
			if got != tt.want {
				t.Errorf("ssoDisplayName(%+v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

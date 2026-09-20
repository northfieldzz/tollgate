package entity

import "testing"

func TestAPIKey_GetHash(t *testing.T) {
	tests := []struct {
		name string
		pk   string
		want string
	}{
		{
			name: "Normal case with KEY# prefix",
			pk:   "KEY#somehash123",
			want: "somehash123",
		},
		{
			name: "Exact prefix length",
			pk:   "KEY#",
			want: "",
		},
		{
			name: "Shorter than prefix length",
			pk:   "KEY",
			want: "",
		},
		{
			name: "Empty string",
			pk:   "",
			want: "",
		},
		{
			name: "Missing prefix but long enough",
			pk:   "12345",
			want: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &APIKey{
				PK: tt.pk,
			}
			if got := k.GetHash(); got != tt.want {
				t.Errorf("APIKey.GetHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

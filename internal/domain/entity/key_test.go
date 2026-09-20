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

func TestCreateKeyInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateKeyInput
		wantErr bool
	}{
		{
			name: "Both TenantID and ServiceID empty",
			input: CreateKeyInput{
				TenantID:  "",
				ServiceID: "",
			},
			wantErr: true,
		},
		{
			name: "Whitespace only",
			input: CreateKeyInput{
				TenantID:  "   ",
				ServiceID: "\t",
			},
			wantErr: true,
		},
		{
			name: "TenantID provided",
			input: CreateKeyInput{
				TenantID: "tenant-1",
			},
			wantErr: false,
		},
		{
			name: "ServiceID provided",
			input: CreateKeyInput{
				ServiceID: "service-1",
			},
			wantErr: false,
		},
		{
			name: "Both provided",
			input: CreateKeyInput{
				TenantID:  "tenant-1",
				ServiceID: "service-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateKeyInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

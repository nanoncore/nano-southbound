package vsol

import (
	"context"
	"strings"
	"testing"

	"github.com/nanoncore/nano-southbound/types"
)

func TestParseDBAProfiles(t *testing.T) {
	output := `
###############DBA PROFILE###########
*****************************
              Id: 1
            name: default
            type: 4
         maximum: 1024000 Kbps

*****************************
              Id: 3
            name: nano_dba_50000
            type: 4
         maximum: 50000 Kbps

`
	profiles, err := parseDBAProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}

	// Check first profile
	if profiles[0].ID != 1 || profiles[0].Name != "default" {
		t.Errorf("profile 0: got ID=%d Name=%q, want ID=1 Name=default", profiles[0].ID, profiles[0].Name)
	}
	if profiles[0].Type != 4 {
		t.Errorf("profile 0: got Type=%d, want 4", profiles[0].Type)
	}
	if profiles[0].MaxBW != 1024000 {
		t.Errorf("profile 0: got MaxBW=%d, want 1024000", profiles[0].MaxBW)
	}

	// Check second profile
	if profiles[1].ID != 3 || profiles[1].Name != "nano_dba_50000" {
		t.Errorf("profile 1: got ID=%d Name=%q, want ID=3 Name=nano_dba_50000", profiles[1].ID, profiles[1].Name)
	}
	if profiles[1].MaxBW != 50000 {
		t.Errorf("profile 1: got MaxBW=%d, want 50000", profiles[1].MaxBW)
	}
}

func TestParseDBAProfilesMultipleTypes(t *testing.T) {
	output := `
###############DBA PROFILE###########
*****************************
              Id: 1
            name: fixed_bw
            type: 1
           fixed: 100000 Kbps

*****************************
              Id: 2
            name: assured_bw
            type: 2
         assured: 50000 Kbps

*****************************
              Id: 3
            name: assured_max
            type: 3
         assured: 50000 Kbps
         maximum: 100000 Kbps

*****************************
              Id: 4
            name: max_bw
            type: 4
         maximum: 200000 Kbps

*****************************
              Id: 5
            name: full_bw
            type: 5
           fixed: 10000 Kbps
         assured: 50000 Kbps
         maximum: 200000 Kbps

`
	profiles, err := parseDBAProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 5 {
		t.Fatalf("expected 5 profiles, got %d", len(profiles))
	}

	// Type 1: fixed
	if profiles[0].Type != 1 || profiles[0].FixedBW != 100000 {
		t.Errorf("type 1: got Type=%d FixedBW=%d, want Type=1 FixedBW=100000", profiles[0].Type, profiles[0].FixedBW)
	}

	// Type 2: assured
	if profiles[1].Type != 2 || profiles[1].AssuredBW != 50000 {
		t.Errorf("type 2: got Type=%d AssuredBW=%d, want Type=2 AssuredBW=50000", profiles[1].Type, profiles[1].AssuredBW)
	}

	// Type 3: assured+max
	if profiles[2].Type != 3 || profiles[2].AssuredBW != 50000 || profiles[2].MaxBW != 100000 {
		t.Errorf("type 3: got Type=%d AssuredBW=%d MaxBW=%d", profiles[2].Type, profiles[2].AssuredBW, profiles[2].MaxBW)
	}

	// Type 4: maximum
	if profiles[3].Type != 4 || profiles[3].MaxBW != 200000 {
		t.Errorf("type 4: got Type=%d MaxBW=%d, want Type=4 MaxBW=200000", profiles[3].Type, profiles[3].MaxBW)
	}

	// Type 5: fixed+assured+max
	if profiles[4].Type != 5 || profiles[4].FixedBW != 10000 || profiles[4].AssuredBW != 50000 || profiles[4].MaxBW != 200000 {
		t.Errorf("type 5: got Type=%d FixedBW=%d AssuredBW=%d MaxBW=%d", profiles[4].Type, profiles[4].FixedBW, profiles[4].AssuredBW, profiles[4].MaxBW)
	}
}

func TestParseDBAProfilesEmpty(t *testing.T) {
	output := `
###############DBA PROFILE###########
`
	profiles, err := parseDBAProfiles(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestBuildDBAProfileCreateCommands(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		profile types.DBAProfile
		want    []string
	}{
		{
			name: "type 4 maximum",
			id:   3,
			profile: types.DBAProfile{
				Name:  "nano_dba_100000",
				Type:  4,
				MaxBW: 100000,
			},
			want: []string{
				"configure terminal",
				"profile dba id 3 name nano_dba_100000",
				"type 4 maximum 100000",
				"commit", "exit", "exit",
			},
		},
		{
			name: "type 1 fixed",
			id:   5,
			profile: types.DBAProfile{
				Name:    "fixed_100m",
				Type:    1,
				FixedBW: 100000,
			},
			want: []string{
				"configure terminal",
				"profile dba id 5 name fixed_100m",
				"type 1 fixed 100000",
				"commit", "exit", "exit",
			},
		},
		{
			name: "type 3 assured+max",
			id:   7,
			profile: types.DBAProfile{
				Name:      "assured_max",
				Type:      3,
				AssuredBW: 50000,
				MaxBW:     100000,
			},
			want: []string{
				"configure terminal",
				"profile dba id 7 name assured_max",
				"type 3 assured 50000 maximum 100000",
				"commit", "exit", "exit",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDBAProfileCreateCommands(tt.id, tt.profile)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d commands, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("command[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetectProfileCLIErrors(t *testing.T) {
	tests := []struct {
		name    string
		outputs []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no errors",
			outputs: []string{"", "gpon-olt-lab(config)#"},
			wantErr: false,
		},
		{
			name:    "profile in use",
			outputs: []string{"Can't delete profile:used by pon 0/1 onu 3."},
			wantErr: true,
			errMsg:  "profile is in use",
		},
		{
			name:    "already exists",
			outputs: []string{"Profile already existed"},
			wantErr: true,
			errMsg:  "profile already exists",
		},
		{
			name:    "not found isn't existed",
			outputs: []string{"profile isn't existed"},
			wantErr: true,
			errMsg:  "profile does not exist",
		},
		{
			name:    "not found is not exist",
			outputs: []string{"profile is not exist"},
			wantErr: true,
			errMsg:  "profile does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := detectProfileCLIErrors(tt.outputs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateProfileName(t *testing.T) {
	valid := []string{"default", "nano_dba_50000", "plan-100M", "profile.v2"}
	for _, name := range valid {
		if err := validateProfileName(name); err != nil {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}

	invalid := []string{"", "has space", "semi;colon", "pipe|cmd", "new\nline", "back`tick", strings.Repeat("a", 65)}
	for _, name := range invalid {
		if err := validateProfileName(name); err == nil {
			t.Errorf("expected %q to be invalid, got nil", name)
		}
	}
}

func TestGetDeleteDBAProfile_ValidatesProfileName(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(a *Adapter) error
	}{
		{
			name: "get rejects invalid name",
			call: func(a *Adapter) error {
				_, err := a.GetDBAProfile(ctx, "bad name")
				return err
			},
		},
		{
			name: "delete rejects invalid name",
			call: func(a *Adapter) error {
				return a.DeleteDBAProfile(ctx, "bad name")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &mockCLIExecutor{outputs: map[string]string{}}
			adapter := &Adapter{cliExecutor: exec}

			err := tt.call(adapter)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), "contains invalid characters") {
				t.Fatalf("expected invalid character error, got %v", err)
			}
			if len(exec.commands) != 0 {
				t.Fatalf("expected no CLI commands for invalid profile name, got %v", exec.commands)
			}
		})
	}
}

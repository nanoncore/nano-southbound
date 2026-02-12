package types

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LineProfileManager defines CRUD operations for line profiles.
// Implemented by vendors that support line profile management via CLI.
type LineProfileManager interface {
	ListLineProfiles(ctx context.Context) ([]*LineProfile, error)
	GetLineProfile(ctx context.Context, name string) (*LineProfile, error)
	CreateLineProfile(ctx context.Context, profile *LineProfile) error
	DeleteLineProfile(ctx context.Context, name string) error
}

// LineProfile defines service configuration for a line profile
// (V-SOL: "profile line").
type LineProfile struct {
	// Name is the profile name (required for create/delete/show).
	Name string `json:"name"`

	// ID is the OLT-assigned profile ID (optional, set on read/list).
	ID *int `json:"id,omitempty"`

	// Tconts defines the TCONT configuration.
	Tconts []*LineProfileTcont `json:"tconts,omitempty"`

	// Mvlan defines multicast VLAN configuration.
	Mvlan *LineProfileMvlan `json:"mvlan,omitempty"`

	// Committed indicates whether the profile is committed on the OLT.
	Committed *bool `json:"committed,omitempty"`
}

// LineProfileTcont defines a TCONT entry.
type LineProfileTcont struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
	DBA  string `json:"dba,omitempty"`

	Gemports []*LineProfileGemport `json:"gemports,omitempty"`
}

// LineProfileGemport defines a GEM port entry.
type LineProfileGemport struct {
	ID             int                       `json:"id"`
	Name           string                    `json:"name,omitempty"`
	TcontID        int                       `json:"tcont_id,omitempty"`
	TrafficLimitUp string                    `json:"traffic_limit_up,omitempty"`
	TrafficLimitDn string                    `json:"traffic_limit_down,omitempty"`
	Encrypt        *bool                     `json:"encrypt,omitempty"`
	State          string                    `json:"state,omitempty"`
	DownQueueMapID *int                      `json:"down_queue_map_id,omitempty"`
	Services       []*LineProfileService     `json:"services,omitempty"`
	ServicePorts   []*LineProfileServicePort `json:"service_ports,omitempty"`
}

// LineProfileService defines a service entry.
type LineProfileService struct {
	Name      string `json:"name"`
	GemportID int    `json:"gemport_id"`
	VLAN      int    `json:"vlan,omitempty"`
	COS       string `json:"cos,omitempty"`
}

// LineProfileServicePort defines a service-port entry.
type LineProfileServicePort struct {
	ID          int    `json:"id"`
	GemportID   int    `json:"gemport_id"`
	UserVLAN    int    `json:"user_vlan,omitempty"`
	VLAN        int    `json:"vlan,omitempty"`
	AdminStatus string `json:"admin_status,omitempty"`
	Description string `json:"description,omitempty"`
}

// LineProfileMvlan defines multicast VLAN configuration.
type LineProfileMvlan struct {
	VLANs []int  `json:"vlans,omitempty"`
	Raw   string `json:"raw,omitempty"`
}

// Validate checks that the line profile parameters are valid.
func (p *LineProfile) Validate() error {
	if p == nil {
		return fmt.Errorf("profile is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}

	tcontIDs := map[int]struct{}{}
	for idx, t := range p.Tconts {
		if t == nil {
			return fmt.Errorf("tcont entry at index %d is nil", idx)
		}
		if err := validateRange("tcont id", t.ID, 1, 255); err != nil {
			return err
		}
		tcontIDs[t.ID] = struct{}{}
		for _, g := range t.Gemports {
			if g == nil {
				continue
			}
			if err := g.Validate(tcontIDs); err != nil {
				return err
			}
		}
	}

	if p.Mvlan != nil {
		if err := p.Mvlan.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks that the GEM port parameters are valid.
func (g *LineProfileGemport) Validate(tcontIDs map[int]struct{}) error {
	if g == nil {
		return fmt.Errorf("gemport is required")
	}
	if err := validateRange("gemport id", g.ID, 1, 255); err != nil {
		return err
	}
	if g.TcontID == 0 {
		return fmt.Errorf("gemport tcont id is required")
	}
	if err := validateRange("gemport tcont id", g.TcontID, 1, 255); err != nil {
		return err
	}
	if len(tcontIDs) > 0 {
		if _, ok := tcontIDs[g.TcontID]; !ok {
			return fmt.Errorf("gemport tcont id %d not found in profile", g.TcontID)
		}
	}
	for _, s := range g.Services {
		if s == nil {
			continue
		}
		if err := s.Validate(); err != nil {
			return err
		}
	}
	for _, sp := range g.ServicePorts {
		if sp == nil {
			continue
		}
		if err := sp.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks that the service parameters are valid.
func (s *LineProfileService) Validate() error {
	if s == nil {
		return fmt.Errorf("service is required")
	}
	if s.Name == "" {
		return fmt.Errorf("service name is required")
	}
	if err := validateRange("service gemport id", s.GemportID, 1, 255); err != nil {
		return err
	}
	if s.VLAN != 0 {
		if err := validateRange("service vlan", s.VLAN, 1, 4094); err != nil {
			return err
		}
	}
	if err := ValidateCOS(s.COS); err != nil {
		return err
	}
	return nil
}

// Validate checks that the service-port parameters are valid.
func (s *LineProfileServicePort) Validate() error {
	if s == nil {
		return fmt.Errorf("service-port is required")
	}
	if err := validateRange("service-port id", s.ID, 1, 128); err != nil {
		return err
	}
	if err := validateRange("service-port gemport id", s.GemportID, 1, 255); err != nil {
		return err
	}
	if s.UserVLAN != 0 {
		if err := validateRange("service-port uservlan", s.UserVLAN, 1, 4094); err != nil {
			return err
		}
	}
	if s.VLAN != 0 {
		if err := validateRange("service-port vlan", s.VLAN, 1, 4094); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks mvlan parameters.
func (m *LineProfileMvlan) Validate() error {
	if m == nil {
		return nil
	}
	if len(m.VLANs) > 0 {
		for _, vlan := range m.VLANs {
			if err := validateRange("mvlan", vlan, 1, 4094); err != nil {
				return err
			}
		}
		return nil
	}
	if m.Raw == "" {
		return nil
	}
	vlans, err := parseVLANList(m.Raw)
	if err != nil {
		return err
	}
	m.VLANs = vlans
	return nil
}

func validateRange(field string, value, min, max int) error {
	if value < min || value > max {
		return fmt.Errorf("%s must be between %d and %d", field, min, max)
	}
	return nil
}

func parseVLANList(raw string) ([]int, error) {
	list := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	if len(list) == 0 {
		return nil, fmt.Errorf("mvlan list is empty")
	}
	vlans := make([]int, 0, len(list))
	for _, item := range list {
		if item == "" {
			continue
		}
		if strings.Contains(item, "-") {
			parts := strings.SplitN(item, "-", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid mvlan range %q", item)
			}
			start, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid mvlan range %q", item)
			}
			end, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid mvlan range %q", item)
			}
			if start > end {
				return nil, fmt.Errorf("invalid mvlan range %q", item)
			}
			for vlan := start; vlan <= end; vlan++ {
				vlans = append(vlans, vlan)
			}
			continue
		}
		val, err := strconv.Atoi(item)
		if err != nil {
			return nil, fmt.Errorf("invalid mvlan %q", item)
		}
		vlans = append(vlans, val)
	}
	return vlans, nil
}

// DBAProfile represents a V-SOL DBA (Dynamic Bandwidth Allocation) profile.
// DBA profiles control upstream T-CONT bandwidth allocation.
type DBAProfile struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Type      int    `json:"type"`       // 1=fixed, 2=assured, 3=assured+max, 4=maximum, 5=fixed+assured+max
	FixedBW   int    `json:"fixed_bw"`   // kbps (types 1, 5)
	AssuredBW int    `json:"assured_bw"` // kbps (types 2, 3, 5)
	MaxBW     int    `json:"max_bw"`     // kbps (types 3, 4, 5)
}

// TrafficProfile represents a V-SOL traffic shaping profile.
// Traffic profiles control per-GEM port SIR/PIR shaping.
type TrafficProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	SIR  int    `json:"sir"` // Sustained Information Rate (kbps)
	PIR  int    `json:"pir"` // Peak Information Rate (kbps)
}

// DBAProfileManager defines CRUD operations for DBA profiles.
type DBAProfileManager interface {
	ListDBAProfiles(ctx context.Context) ([]DBAProfile, error)
	GetDBAProfile(ctx context.Context, name string) (*DBAProfile, error)
	CreateDBAProfile(ctx context.Context, profile DBAProfile) error
	DeleteDBAProfile(ctx context.Context, name string) error
}

// TrafficProfileManager defines CRUD operations for traffic profiles.
type TrafficProfileManager interface {
	ListTrafficProfiles(ctx context.Context) ([]TrafficProfile, error)
	GetTrafficProfile(ctx context.Context, name string) (*TrafficProfile, error)
	CreateTrafficProfile(ctx context.Context, profile TrafficProfile) error
	DeleteTrafficProfile(ctx context.Context, name string) error
}

var cosRangePattern = regexp.MustCompile(`^\d(-\d)?$`)

// ValidateCOS checks the COS string format (optional helper).
func ValidateCOS(cos string) error {
	if cos == "" {
		return nil
	}
	if !cosRangePattern.MatchString(cos) {
		return fmt.Errorf("cos must be a number or range (e.g., 0-7)")
	}
	return nil
}

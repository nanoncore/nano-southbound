package gnmi

import (
	"testing"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestPathToString(t *testing.T) {
	tests := []struct {
		name string
		path *gnmipb.Path
		want string
	}{
		{
			name: "nil path",
			path: nil,
			want: "",
		},
		{
			name: "empty path no elems",
			path: &gnmipb.Path{},
			want: "/",
		},
		{
			name: "single elem no keys",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interfaces"},
				},
			},
			want: "/interfaces",
		},
		{
			name: "single elem with one key",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
				},
			},
			want: "/interface[name=eth0]",
		},
		{
			name: "single elem with multiple keys sorted alphabetically",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interface", Key: map[string]string{
						"z-key": "last",
						"a-key": "first",
						"m-key": "middle",
					}},
				},
			},
			want: "/interface[a-key=first][m-key=middle][z-key=last]",
		},
		{
			name: "multiple path elements",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "interfaces"},
					{Name: "interface", Key: map[string]string{"name": "eth0"}},
					{Name: "state"},
					{Name: "counters"},
				},
			},
			want: "/interfaces/interface[name=eth0]/state/counters",
		},
		{
			name: "elem with empty key map",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "system", Key: map[string]string{}},
				},
			},
			want: "/system",
		},
		{
			name: "key values with special characters",
			path: &gnmipb.Path{
				Elem: []*gnmipb.PathElem{
					{Name: "acl", Key: map[string]string{"name": "my-acl/v4"}},
				},
			},
			want: "/acl[name=my-acl/v4]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathToString(tt.path)
			if got != tt.want {
				t.Errorf("PathToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathToString_KeyOrderDeterministic(t *testing.T) {
	// Run multiple times to verify deterministic ordering
	// (Go map iteration is randomized)
	path := &gnmipb.Path{
		Elem: []*gnmipb.PathElem{
			{Name: "entry", Key: map[string]string{
				"delta":   "4",
				"alpha":   "1",
				"charlie": "3",
				"bravo":   "2",
			}},
		},
	}

	want := "/entry[alpha=1][bravo=2][charlie=3][delta=4]"
	for i := 0; i < 100; i++ {
		got := PathToString(path)
		if got != want {
			t.Fatalf("PathToString() iteration %d = %q, want %q (non-deterministic key order detected)", i, got, want)
		}
	}
}

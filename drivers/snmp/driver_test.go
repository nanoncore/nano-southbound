package snmp

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

func TestConvertSNMPValue(t *testing.T) {
	tests := []struct {
		name string
		pdu  gosnmp.SnmpPDU
		want interface{}
	}{
		// OctetString with []byte value
		{
			name: "OctetString with byte slice",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte("hello")},
			want: "hello",
		},
		{
			name: "OctetString with empty byte slice",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte("")},
			want: "",
		},
		{
			name: "OctetString with unexpected type falls back to Sprintf",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: 42},
			want: "42",
		},

		// Integer
		{
			name: "Integer with int value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(100)},
			want: int64(100),
		},
		{
			name: "Integer with negative value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(-1)},
			want: int64(-1),
		},
		{
			name: "Integer with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: int(0)},
			want: int64(0),
		},
		{
			name: "Integer with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: "not an int"},
			want: "not an int",
		},

		// Counter32 / Gauge32 / TimeTicks
		{
			name: "Counter32 with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: uint(12345)},
			want: uint64(12345),
		},
		{
			name: "Gauge32 with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Gauge32, Value: uint(999)},
			want: uint64(999),
		},
		{
			name: "TimeTicks with uint value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.TimeTicks, Value: uint(3600)},
			want: uint64(3600),
		},
		{
			name: "Counter32 with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: uint(0)},
			want: uint64(0),
		},
		{
			name: "Counter32 with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter32, Value: int64(42)},
			want: int64(42),
		},

		// Counter64
		{
			name: "Counter64 with uint64 value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: uint64(9999999999)},
			want: uint64(9999999999),
		},
		{
			name: "Counter64 with zero",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: uint64(0)},
			want: uint64(0),
		},
		{
			name: "Counter64 with unexpected type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Counter64, Value: "wrong"},
			want: "wrong",
		},

		// NoSuch / EndOfMibView
		{
			name: "NoSuchObject returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.NoSuchObject, Value: nil},
			want: nil,
		},
		{
			name: "NoSuchInstance returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.NoSuchInstance, Value: "some value"},
			want: nil,
		},
		{
			name: "EndOfMibView returns nil",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.EndOfMibView, Value: 123},
			want: nil,
		},

		// Default case
		{
			name: "Unknown type returns raw value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Asn1BER(0xFF), Value: "raw"},
			want: "raw",
		},
		{
			name: "Unknown type with nil value",
			pdu:  gosnmp.SnmpPDU{Type: gosnmp.Asn1BER(0xFF), Value: nil},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSNMPValue(tt.pdu)
			if got != tt.want {
				t.Errorf("convertSNMPValue() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

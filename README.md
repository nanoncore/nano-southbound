# nano-southbound

Southbound drivers and vendor adapters for Nanoncore network equipment management.

## Overview

This package provides protocol drivers and vendor-specific adapters for communicating with network equipment (OLTs, BNGs, routers) from various vendors.

## Supported Protocols

- **CLI** - SSH/Telnet command-line interface
- **NETCONF** - RFC 6241 NETCONF over SSH
- **gNMI** - gRPC Network Management Interface
- **SNMP** - Simple Network Management Protocol

## Supported Vendors

| Vendor | Primary Protocol | Equipment Types |
|--------|-----------------|-----------------|
| Nokia | NETCONF | ISAM FX, Lightspan |
| Huawei | CLI | MA5600T, MA5800 |
| ZTE | CLI | C300, C320, C600 |
| Cisco | NETCONF | Routed PON, ASR |
| Juniper | NETCONF | MX Series |
| Adtran | NETCONF | SDX 6000 Series |
| Calix | NETCONF | AXOS E7/E9 |
| DZS | CLI | Velocity Series |
| FiberHome | SNMP | AN5516 |
| Ericsson | NETCONF | MINI-LINK |
| V-SOL | CLI | V1600G Series |
| C-Data | CLI | FD1104S, FD1208S |

## Installation

```bash
go get github.com/nanoncore/nano-southbound
```

## Usage

```go
import (
    "github.com/nanoncore/nano-southbound"
    "github.com/nanoncore/nano-southbound/types"
)

// Create a driver for a V-SOL OLT
config := &types.EquipmentConfig{
    Vendor:   types.VendorVSOL,
    Address:  "192.168.1.1",
    Username: "admin",
    Password: "admin123",
    Timeout:  60 * time.Second,
}

driver, err := southbound.NewDriver(types.VendorVSOL, "", config)
if err != nil {
    log.Fatal(err)
}

// Connect
ctx := context.Background()
if err := driver.Connect(ctx, config); err != nil {
    log.Fatal(err)
}
defer driver.Disconnect(ctx)

// Use DriverV2 interface for OLT operations
if driverV2, ok := driver.(types.DriverV2); ok {
    // Discover unprovisioned ONUs
    discoveries, err := driverV2.DiscoverONUs(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }

    for _, d := range discoveries {
        fmt.Printf("Found ONU: %s on port %s\n", d.Serial, d.PONPort)
    }
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application                               │
│                    (nano-agent, etc.)                           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                      nano-southbound                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Factory (NewDriver)                     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                           │                                      │
│         ┌─────────────────┼─────────────────┐                   │
│         ▼                 ▼                 ▼                   │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │   Vendor    │   │   Vendor    │   │   Vendor    │           │
│  │  Adapters   │   │  Adapters   │   │  Adapters   │           │
│  │  (V-SOL,    │   │  (Nokia,    │   │  (Huawei,   │           │
│  │   C-Data)   │   │   Cisco)    │   │   ZTE)      │           │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘           │
│         │                 │                 │                   │
│         ▼                 ▼                 ▼                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              Protocol Drivers                            │   │
│  │     CLI    │   NETCONF   │    gNMI    │    SNMP         │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                           │
                           ▼
                    Network Equipment
```

## Contributing

Contributions are welcome! To add support for a new vendor:

1. Create a new package under `vendors/`
2. Implement the `Driver` interface (and optionally `DriverV2`)
3. Add vendor to the capability matrix in `factory.go`
4. Add tests and documentation

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

Apache 2.0 - See [LICENSE](LICENSE) for details.

# akikit

Go client SDK (gRPC + HTTP helpers) for the **Aki** service.

Aki uses **CPU virtualization** to run unmodified **iOS / macOS / Linux-Android**
runtimes and drive their native security routines in-process, exposing them as a
stable gRPC API. Clients get the exact outputs the real platform would produce,
without owning physical devices.

## Capabilities

The service emulates the target OS at the instruction level and performs the
following computations:

- **Apple Anisette** — provisioning and one-time data generation (OTP / MID / SRM,
  routing-info management, machine sync).
- **SAP sign / verify** — secure-association-protocol exchange, signing, prime
  signing, and signature verification.
- **Keybag & subscription** — FairPlay/SSV context handling, keybag import,
  subscription-bag and subscription-request computation, lease management.
- **Absinthe (absin)** — handshake exchange and request signing.
- **Apple device activation** — device-level activation computation.

It also exposes general signing primitives (QuickSign, RSA sign) used by the
flows above.

> **Not included:** **SEP (Secure Enclave) activation** computation is out of
> scope and is not provided by this service.

## Versioning

Emulated OS images and the API surface are **auto-incremented** — version numbers
advance automatically as new platform builds are tracked, so clients stay current
without manual image management.

## Usage

```go
ctx := context.Background()
client, closeFn, err := akikit.Dial(ctx, "host:port", &akikit.Device{
    UDID:        "...",
    SN:          "...",
    ProductType: "...",
    OSVersion:   "...",
})
if err != nil {
    log.Fatal(err)
}
defer closeFn()

// Example: SAP exchange + sign
ex, _ := client.SAPExchange(0, version, input)
sig, _ := client.SAPSign(ex.Ctx, data)
```

A session is opened per device (`OpenSession`) and must be closed when done
(`CloseSession`). Context-handle resources (SAP / Absin / SSV) should be torn down
with their respective `*Teardown` / `*Destroy` calls.

## Surface

| Area | RPCs |
| --- | --- |
| Session | `OpenSession`, `CloseSession`, `Ping`, `Hubs` |
| Anisette / provisioning | `RequestOTPForDSID`, `ProvisionStart`, `ProvisionEnd`, `IsProvisioned`, `ProvisionErase`, `ProvisionDestroy`, `Synchronize`, `SetRoutingInfo`, `GetRoutingInfo`, `LoginCode` |
| SAP | `SAPExchange`, `SAPSign`, `SAPPrimeSign`, `SAPVerify`, `SAPPrimeVerify`, `SAPTeardown` |
| Keybag / subscription (SSV) | `SSVGetFairPlayContext`, `SSVIsValidContext`, `SSVSubscriptionBag`, `SSVSubscriptionRequest`, `SSVImportKeybag`, `SSVImportSubscriptionKeybag`, `SSVImportSubscriptionResponse`, `SSVStopSubscriptionLease`, `SSVFairplayDestroy`, `KeyBagData` |
| Signing | `QuickSign`, `RSASign` |
| Absinthe | `AbsinExchange1`, `AbsinExchange2`, `AbsinSign`, `AbsinTeardown` |

See [aki.proto](aki.proto) for full message definitions.

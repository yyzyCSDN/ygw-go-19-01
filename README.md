# Multipart upload finalizer

This service stores multipart upload sessions, validates part numbering, sizes
and checksums, inspects each payload, builds immutable object manifests, commits
final upload state, records audit events, and supports batch finalization.

Run the example with `go run ./cmd/finalize-demo`.

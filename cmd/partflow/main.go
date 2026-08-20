package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"example.com/partflow"
	"example.com/partflow/controlplane"
)

func main() {
	store := multipart.NewStore()
	policies := multipart.NewPolicyRegistry()
	quota := multipart.NewQuotaLedger(time.Now)
	_ = policies.Replace("demo", []multipart.UploadPolicy{{Tenant: "demo", Prefix: "incoming/", MaxParts: 1000, MaxPartBytes: 64 << 20}})
	_ = quota.SetPolicy("demo", multipart.QuotaPolicy{MaxOpenUploads: 100, MaxBytes: 10 << 30, Window: time.Hour})
	server, err := controlplane.New(store, policies, quota, time.Now)
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("PARTFLOW_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	log.Printf("partflow listening on %s", address)
	if err := http.ListenAndServe(address, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

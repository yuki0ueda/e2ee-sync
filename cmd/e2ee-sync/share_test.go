package main

import (
	"encoding/json"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	code := generateCode()
	if len(code) != 32 {
		t.Errorf("generateCode() length = %d, want 32", len(code))
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			t.Errorf("generateCode() contains non-hex char: %c", c)
		}
	}
	// Two codes should be different (probabilistic but practically guaranteed)
	code2 := generateCode()
	if code == code2 {
		t.Error("generateCode() produced same code twice")
	}
}

func TestTransferPayloadRoundTrip(t *testing.T) {
	original := TransferPayload{
		UseHub:            true,
		HubEndpoint:       "my-hub:8080",
		BackendProvider:   "Cloudflare",
		BackendName:       "Cloudflare R2",
		S3AccessKeyID:     "AKID123",
		S3SecretAccessKey: "secret456",
		S3Endpoint:        "https://abc.r2.cloudflarestorage.com",
		S3Region:          "auto",
		EncPassword:       "enc-pass",
		EncSalt:           "enc-salt",
		WebDAVPassword:    "webdav-pass",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded TransferPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.UseHub != original.UseHub {
		t.Errorf("UseHub = %v, want %v", decoded.UseHub, original.UseHub)
	}
	if decoded.HubEndpoint != original.HubEndpoint {
		t.Errorf("HubEndpoint = %q, want %q", decoded.HubEndpoint, original.HubEndpoint)
	}
	if decoded.S3AccessKeyID != original.S3AccessKeyID {
		t.Errorf("S3AccessKeyID = %q, want %q", decoded.S3AccessKeyID, original.S3AccessKeyID)
	}
	if decoded.S3SecretAccessKey != original.S3SecretAccessKey {
		t.Errorf("S3SecretAccessKey = %q, want %q", decoded.S3SecretAccessKey, original.S3SecretAccessKey)
	}
	if decoded.EncPassword != original.EncPassword {
		t.Errorf("EncPassword = %q, want %q", decoded.EncPassword, original.EncPassword)
	}
	if decoded.EncSalt != original.EncSalt {
		t.Errorf("EncSalt = %q, want %q", decoded.EncSalt, original.EncSalt)
	}
	if decoded.WebDAVPassword != original.WebDAVPassword {
		t.Errorf("WebDAVPassword = %q, want %q", decoded.WebDAVPassword, original.WebDAVPassword)
	}
}

func TestTransferPayload_NoHubOmitsWebDAV(t *testing.T) {
	payload := TransferPayload{
		UseHub:          false,
		BackendProvider: "AWS",
		S3AccessKeyID:   "key",
	}
	data, _ := json.Marshal(payload)
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)

	// WebDAV should be omitted (omitempty) when empty
	if _, exists := decoded["webdav_password"]; exists {
		t.Error("webdav_password should be omitted when empty")
	}
	// HubEndpoint should be omitted when empty
	if _, exists := decoded["hub_endpoint"]; exists {
		t.Error("hub_endpoint should be omitted when empty")
	}
}
